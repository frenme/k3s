# AGENTS.md

Guide for AI agents (Cursor, Claude Code, Codex) working on this repository and investigating its clusters.

## Layout

- `cmd/<service>` entrypoints, `internal/<service>` services (see Writing services), `internal/clients/<service>` HTTP clients of other services, `internal/lib` shared code: `app` (service bootstrap with logging and tracing), `httpx` (server, middleware, instrumented `NewClient`), `pg`, `cache`, `lock` (redis locks), `pagination` (keyset cursors), `idempotency`, `envconfig`, `version`.
- `api/*.yaml` OpenAPI generated from the code (`make openapi`), validated by `e2e`.
- `infra/deploy/helmfile.yaml.gotmpl` every release; `infra/deploy/environments/*.yaml` per-env settings; `infra/deploy/values/{base,local,dev,prod,derived}` release values; `infra/deploy/helm/*` own charts.
- `infra/terraform` Hetzner servers, `infra/k3d` local cluster.
- `make help` lists every command.

## Writing services

Every service follows the same layout, `internal/users` is the reference:

```
internal/<service>/
  model/          plain structs and domain errors, one file per model
  storage/        all database access of the service through GORM, one file per table, plus migrations/
  <feature>/      one package per feature (profile, auth, checkout)
    service.go    business logic, no HTTP and no SQL; declares the small interfaces it needs
    v1/           one folder per API version
      dto.go      request and response structs of this version, mapped from and to model
      handler.go  Huma operations: register routes, call the service, map errors
  routes.go       Setup opens Postgres and wires features; OpenAPI builds the spec without a database
```

- HTTP is [Huma](https://huma.rocks) on top of `net/http` (`internal/lib/httpx/api.go`). Routes, validation tags and docs live in `huma.Register` calls and DTO tags.
- Errors are defined once, as `httpx.Error{Status, Code, Message}`: entity errors in `model/errors.go`, scenario errors next to the feature service (`checkout.ErrUnknownUser`), shared ones (`not_found`, `invalid_input`, `internal_error`) in `internal/lib/httpx/errors.go`. Storage and services return them (wrapping with `%w` is fine), code checks them with `errors.Is`, which matches by `code`.
- Every response error is `{"code": "...", "message": "...", "details": [...]}`. Clients branch on `code`, never on `message`. Handlers end with `return nil, httpx.Fail(ctx, log, err)`: anything that is not an `httpx.Error` becomes `internal_error`, 5xx are logged, and internal details never reach the response.
- Input validation lives in DTO tags (`format`, `pattern`, `minimum`, `maxLength`), not in services: Huma returns 422 `invalid_input` with details, malformed JSON is 400 `bad_request`. Services only normalize input and enforce business rules.
- Public routes are `/v1/<resource>` on `:8080` (`s.Mux`), reached as `/api/v1/<service>/...` through the gateway. A breaking change goes to a new `v2/` folder next to `v1/`, reusing the same service.
- Internal routes are `/internal/v1/<resource>` on `:8081` (`s.Internal`), reached only by other services. The gateway refuses any path with an `internal` segment, and the NetworkPolicy opens `:8081` only to pods listed in `networkPolicy.internalFromPods`.
- Docs: `/api/v1/<service>/docs`, spec at `/api/v1/<service>/openapi.json`.
- Environment variables are read only through `envconfig`: each service declares an `EnvConfig` struct with `env`, `default` and `required` tags and calls `envconfig.Load`, so every problem is reported at once and nothing panics. This is separate from `AppConfig`, the runtime settings kept in the `common` service.
- Lists use keyset pagination: `limit` and an opaque `cursor` query parameter, `next_cursor` in the response. Storage calls `pg.Keyset` (ordered by `created_at DESC, id DESC`, fetches `limit+1`), the service wraps the result with `pagination.NewPage`.
- Endpoints that must be safe to retry take an `Idempotency-Key` header and build `idempotency.New(scope, key, payload)`; the key is stored hashed together with the scope (the owner) and a fingerprint of the payload, a replay with another payload fails with `idempotency_key_reused`.
- `api/*.yaml` is generated from the code by `make openapi`, never edited by hand. CI fails if it differs from the code, `e2e` validates real responses against it.
- Database access lives only in `storage/` and goes through GORM (`*pg.DB`: `Write` for writes, `pg.Read` for reads that prefer the replica and fall back to the primary). GORM row structs with tags stay private in `storage/` and are mapped to `model/` structs; depguard rejects GORM and pgx anywhere else.
- Schema changes are plain SQL files in `storage/migrations` named `<yyyymmddhhmmss>_<name>.sql`, never GORM `AutoMigrate`. While nothing is deployed the files only create objects (tables, indexes, views, seed rows). A file named `*.notx.sql` runs outside a transaction and must hold one statement (`CREATE INDEX CONCURRENTLY IF NOT EXISTS ...`); if it fails and leaves an invalid index, drop it with `DROP INDEX CONCURRENTLY IF EXISTS` in a new `*.notx.sql` before retrying.
- With the CloudNativePG pooler (`postgres.pooler.enabled`) services connect through PgBouncer in transaction mode: `DATABASE_URL` points to `postgres-pooler-rw`, `DATABASE_READ_URL` to `postgres-pooler-ro`, and `DATABASE_DIRECT_URL` to `postgres-rw`, used for migrations only (they need a session for the advisory lock). The migrator also runs `ALTER ROLE CURRENT_USER SET search_path`, because PgBouncer cannot carry `search_path` per connection. Pool sizes are derived in `helm/postgres/templates/pooler.yaml` from `max_connections`, the pooler instances and the number of database services, so scaling pods never raises the number of server connections. Never rely on session state (`SET`, advisory locks, `LISTEN`) in storage code.
- Atomic multi-step changes: the service declares `type transactor interface { Do(ctx, func(ctx) error) error }` (satisfied by `*pg.DB`) and wraps the storage calls in `s.tx.Do`; storage methods get their connection with `db.Conn(ctx)` for writes and `pg.Read(ctx, db, ...)` for reads, which join the transaction found in `ctx`. Do HTTP calls to other services before `Do`, never inside it.
- Feature packages never import each other; what they share lives in `model/` and `storage/`.
- A growing `service.go` splits per use case inside the same package (`register.go`, `login.go`).
- Each service owns one Postgres schema and one migration history in `storage/migrations`.
- Services never import another service's `internal/<service>` packages (enforced by depguard). They call each other over HTTP through `internal/clients/<service>` (`users.New(url)`, `users.Client`), which mirrors only the fields the caller needs.
- Migrations must stay compatible with the previous release during rolling updates: add first, switch code in the next release, drop last.
- `common` holds what is shared by the whole app: `appconfig` (runtime settings, key and JSON value, cached in redis for `APP_CONFIG_CACHE_TTL`; public ones are served at `/api/v1/common/app-config`, any key to services at `/internal/v1/common/app-config/{key}` through `internal/clients/common`) and `health` (`/api/v1/common/health` is public and answers only `ok` or `unhealthy`; `/api/v1/common/health/details` needs the admin password and lists every service, postgres and redis with latency, error and the failing checks from the service's `/readyz`). It scales like any other service because every service depends on it. Example use: `users` refuses sign-ups when `registration_enabled` is false.
- Admin reads other services only through views they own: `users.admin_users` and `orders.admin_orders`, created by each service's migrations. A view is a contract: add columns freely, but a migration that drops or retypes a column the view uses fails (Postgres refuses), so change the view first. `contract/` runs the real migrations of users and orders and reads them through the admin storage.
- `admin` is the exception that reads every schema: its Postgres role is in `pg_read_all_data` (`readAllSchemas: true` in `common.yaml`), so it can read but never write other services' tables. It runs as exactly one pod (`singleton: true`: no HPA, no PDB, `Recreate` rollout) and its routes `/api/v1/admin/...` pass the gateway only with the admin password (`gateway.AdminMiddleware`, HTTP Basic). The password is hardcoded in `internal/gateway/admin.go` for now.

## Gateway

- Every `/api/v1/...` request must carry `Authorization: Bearer <jwt>`, verified in `internal/gateway` with `JWT_SECRET` (HS*) or `JWT_PUBLIC_KEY` (PEM, RS*/ES*/EdDSA), optional `JWT_ISSUER` and `JWT_AUDIENCE`; `exp` and `sub` are required. Tokens are issued by an external identity provider, nothing here issues them.
- Access is declared where the route is: public API operations are registered with `httpx.Register(api, op, httpx.AccessPublic|AccessUser|AccessAdmin, handler)`, which records `x-access` in the OpenAPI spec. `make openapi` turns the specs into `internal/gateway/rules_gen.go` (CI fails if it is stale and the generator fails for an operation registered without access). Anything without a rule needs a JWT; `docs` and `openapi` paths are public and `/api/v1/admin/**` always needs the admin password (`staticRules` in `internal/gateway/auth.go`). `admin` means HTTP Basic. The gateway removes `Authorization` and any client supplied `X-User-ID`, then sets `X-User-ID` to the token subject. Services trust it only because the NetworkPolicy lets nothing but the gateway reach `:8080`; per resource ownership checks in services are not done yet.
- Rate limits per route live in the ingress values (`infra/deploy/values/base/gateway.yaml`, `ingress.routes.<name>.rateLimit`, enforced by Traefik); `health` is the example of a dedicated limit.
- The secret is `secrets.jwt` (plain in `environments/local.yaml`, sops in `dev.secrets.yaml` and `prod.secrets.yaml`).

## Scheduler

`scheduler` runs cron jobs (`internal/scheduler/jobs/catalog.go`) and has internal routes only: `GET /internal/v1/scheduler/jobs` and `POST /internal/v1/scheduler/jobs/{name}/run`, reached by the admin service as `/api/v1/admin/scheduler/jobs[/{name}/run]`. It runs with several replicas: each cron tick takes a redis lock (`tick:<job>`, held for the schedule period) and each run takes `run:<job>`, so one replica executes a tick and a manual run never overlaps a scheduled one. It has no database; jobs reach other services through `internal/clients`. Metrics: `scheduler_job_runs_total`, `scheduler_job_duration_seconds`, `scheduler_job_last_success_timestamp_seconds`.

## Environments

| ENV   | kube context   | app namespace |
|-------|----------------|---------------|
| local | `k3d-k3s-base` | `app`         |
| dev   | `k3s-dev`      | `app-dev`     |
| prod  | `k3s-prod`     | `app-prod`    |

Platform namespaces: `monitoring` (Prometheus, Grafana, Alertmanager, Loki, Tempo, Alloy), `cnpg-system`, `cert-manager`, `kube-system`, `cattle-system`.

Nodes are labeled `role=core` (databases, monitoring, services) and `role=edge` (ingress only).

Servers are listed in `infra/terraform/envs/<env>.tfvars`: `server` is the control plane (core), `nodes` is a map of extra servers with `location`, `server_type`, `region` and `role` (`edge` by default, `core` for more core capacity such as a database replica). Add or remove entries and run `make infra-apply ENV=<env>`; a new `edge` gets Traefik and its own gateway pod (gateway is a DaemonSet on `role=edge`), core nodes sit in a spread placement group on different hosts. A database replica is a `core` node plus `postgres.instances: 2` in `environments/<env>.yaml`.

CoreDNS is our own manifest `infra/k3s/coredns.yaml` (k3s CoreDNS is disabled) pinned to `role=core`, so cluster DNS never moves to a remote edge. `dev` runs without metrics and traces (`telemetry` in `environments/dev.yaml`): only services, Loki, Alloy and Grafana for logs.

## Connecting

Read-only kubectl:

```bash
make agent-kubeconfig ENV=prod
export KUBECONFIG=$PWD/kubeconfig-agent-prod.yaml
```

The `agent` ServiceAccount has the `view` ClusterRole plus read access to nodes, volumes, metrics and the CloudNativePG, Prometheus, Traefik and upgrade resources. It cannot read Secrets, exec into pods, port-forward or change anything.

Logs, traces and metrics through Grafana MCP, `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "grafana-prod": {
      "command": "make",
      "args": ["-s", "-C", "/path/to/k3s", "mcp", "ENV=prod"]
    }
  }
}
```

The server only exposes read tools (search, datasources, Prometheus, Loki, Tempo, alerting, dashboards). It needs the operator kubeconfig for the port-forward and Docker.

## Telemetry

- Every service logs JSON to stdout: `time`, `level`, `msg`, `service`, plus `trace_id` and `span_id` inside a request.
- `X-Request-ID` response header equals the trace ID, so a user report with that header leads straight to the trace. The gateway ignores incoming `traceparent` and starts a new trace with a link to it.
- Traces: gateway → users/orders → postgres (pgx) and redis, OTLP to Tempo.
- `http_request_duration_seconds` carries exemplars with `trace_id`.
- Outgoing calls made with `httpx.NewClient(name, timeout)` are traced and counted in `http_client_requests_total` and `http_client_request_duration_seconds`. `NewClient` also retries idempotent requests (GET, HEAD, OPTIONS; twice, with jittered backoff, on network errors and 502/503/504), opens a circuit breaker after 5 consecutive failures for 10s (`ErrCircuitOpen`, gauge `http_client_circuit_open`) and caps concurrent requests at 100 (`ErrBulkheadFull`); tune with `httpx.WithResilience`, switch off with `httpx.WithoutResilience` (the health checks do). `timeout` is the total budget including retries.
- Postgres pools export `db_pool_*` (acquired, idle, open, max connections, acquire counters and wait time) per `service` and `pool` (`write`, `read`).
- Business counters live next to the code that changes state (`users_registered_total`, `orders_created_total{result}`); add new ones the same way with `promauto`.
- Secrets, tokens, `Authorization` values, URL passwords and JWTs are masked as `***` before logs reach Loki.

Loki labels: `namespace`, `pod`, `container`, `app`, `node`, `stream`, `service_name`. Log level is the `detected_level` field (`debug`, `info`, `warn`, `error`), filter it after the selector.

Datasource uids: `prometheus`, `loki`, `tempo`.

## Queries

LogQL:

```logql
{namespace="app-prod"} | detected_level="error"
{namespace="app-prod", app="orders"} | json | status >= 500
{namespace="app-prod"} |= "4bf92f3577b34da6a3ce929d0e0e4736"
sum by (app) (count_over_time({namespace="app-prod"} | detected_level="error" [5m]))
{namespace="app-prod", app="gateway"} | json | msg="request" | duration_ms > 500
```

TraceQL:

```traceql
{ resource.service.name = "orders" && status = error }
{ resource.k8s.namespace.name = "app-prod" && duration > 500ms }
{ span.http.route = "/v1/orders/{id}" && span.http.response.status_code >= 500 }
{ resource.service.name = "users" } >> { span.db.system.name = "postgresql" && duration > 100ms }
```

PromQL:

```promql
sum by (service) (rate(http_requests_total{namespace="app-prod", code="5xx"}[5m]))
histogram_quantile(0.99, sum by (service, le) (rate(http_request_duration_seconds_bucket{namespace="app-prod"}[5m])))
ALERTS{alertstate="firing"}
cnpg_pg_replication_lag
```

## Investigating

1. Start from firing alerts and the error rate by service.
2. Pull error logs for the service and time window; take a `trace_id` from them or an exemplar on the latency panel.
3. Open the trace and find the failing or slow span (upstream call, SQL, redis).
4. Check the cluster state: `kubectl get pods -n <ns>`, `kubectl describe pod`, `kubectl get events -n <ns> --sort-by=.lastTimestamp`, `kubectl logs --previous`, `kubectl top pods`.
5. Database: `kubectl get clusters.postgresql.cnpg.io -n <ns> postgres -o yaml` status, replication lag and WAL archiving metrics.
6. Report the cause with links to logs, the trace and the commit that likely introduced it.

## Rules

- Investigation is read-only. Never apply, delete, scale, restart, exec, port-forward or run `make deploy` against dev or prod unless the user asks for that action.
- Never print or copy secrets, connection strings or tokens, even when a log line has one that escaped masking; report the leak instead.
- Keep Loki queries scoped to a namespace and a short time range.
- Code style: no comments in code, golangci-lint clean, `make lint test test-integration e2e` before handing over.
