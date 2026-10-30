# AGENTS.md

Guide for AI agents (Cursor, Claude Code, Codex) working on this repository and investigating its clusters.

## Layout

- `cmd/<service>` entrypoints, `internal/<service>` services (see Writing services), `internal/clients/<service>` HTTP clients of other services, `internal/lib` shared code: `app` (service bootstrap with logging and tracing), `httpx` (`Serve`, `Probes`, middleware, `Register`, instrumented `NewClient`, `ServiceClient` for calls to other services), `pg`, `cache`, `lock` (redis locks), `pagination` (`Params`, `Page`, `Fetch`), `idempotency`, `envconfig`, `version`.
- `openapi/*.yaml` OpenAPI generated from the code (`make openapi`), validated by `e2e`.
- `infra/deploy/helmfile.yaml.gotmpl` every release; `infra/deploy/environments/{common,local,dev,prod}.yaml` settings (`common.yaml` holds the `apps` registry and defaults, an env file only what differs); `infra/deploy/values/{base,local,dev,prod,derived}` values of vendor charts and of the gateway ingress; `infra/deploy/helm/*` own charts. See Deploy config.
- `infra/terraform` Hetzner servers, `infra/k3d` local cluster.
- `make help` lists every command.

## Writing services

Every service follows the same layout, `internal/users` is the reference:

```
internal/<service>/
  model/          plain structs, one file per model, and all errors of the service in errors.go
  storage/        all database access of the service through GORM, one file per table, plus migrations/
  <feature>/      one package per resource (user, order, auth)
    service.go    business logic, no HTTP and no SQL; declares the small interfaces it needs
    v1/           one folder per API version
      dto.go      request and response structs of this version, mapped from and to model
      handler.go  Huma operations: register routes, call the service, map errors
  routes.go       Setup opens Postgres and wires features; OpenAPI builds the spec without a database
```

- Every Go file reads top to bottom in the same order: plain `const` and `var`, then types, each followed by its constructor (`NewX`) and methods, then the `const` and `var` of that type (`ErrX`, enum values), then free functions. `handler.go`: `Handler`, `NewHandler`, `Register`, then the handlers; `dto.go`: only types, the `toX` converters at the end (a feature never declares DTOs in `handler.go`); `routes.go`: `Setup`, `OpenAPI`, `register`, then helpers.
- HTTP is [Huma](https://huma.rocks) on top of `net/http` (`internal/lib/httpx/api.go`). Routes, validation tags and docs live in `huma.Register` calls and DTO tags.
- Errors are defined once, as `httpx.Error{Status, Code, Message}`: every error of a service (entity and scenario ones) in its `model/errors.go`, shared ones (`not_found`, `invalid_input`, `invalid_cursor`, `unauthorized`, `idempotency_key_reused`, `upstream_unavailable`, `internal_error`) in `internal/lib/httpx/errors.go`. Storage and services return them as they are (wrapping with `%w` is fine) and never redefine or translate them; code checks them with `errors.Is`, which matches by `code`.
- Every response error is `{"code": "...", "message": "...", "details": [...]}`. Clients branch on `code`, never on `message`. Handlers just `return nil, err`: `httpx.Register` (and `httpx.RegisterInternal` for `:8081`) turns anything that is not an `httpx.Error` into `internal_error`, logs 5xx at error level with `code` and `err`, and never lets internal details reach the response. Handlers have no logger.
- Input validation lives in DTO tags (`format`, `pattern`, `minimum`, `maxLength`), not in services: Huma returns 400 `invalid_input` with details, malformed JSON is 400 `bad_request`. Services only normalize input and enforce business rules.
- The current user is never taken from a body or a query: a handler calls `httpx.CurrentUser(ctx)`, which reads `X-User-ID` set by the gateway (401 `unauthorized` without it; `:8081` calls carry no user). A resource of another user answers 404, never 403, so its existence is not revealed: `orders` filters by the owner in storage, `GET /v1/users/{id}` answers only for the user itself, and admins read everything through `admin`.
- Status codes stay to the common ones: 200 for every success (a create and an idempotent replay included), 400 for any wrong input (`code` tells what exactly), 401, 403, 404 (also for a wrong method and for a resource of another user), 409, 413, 502, 503, 504. Operations do not use 201, 204, 405, 422 or custom codes (only the protocol itself does: 204 for a CORS preflight, 304 for ETag); a new operation answers 200 and lists its errors in `Errors`.
- Public routes are `/v1/<resource>` on `:8080` (`s.Mux`), reached as `/api/v1/<service>/...` through the gateway. A breaking change goes to a new `v2/` folder next to `v1/`, reusing the same service.
- Internal routes are `/internal/v1/<resource>` on `:8081` (`s.Internal`), reached only by other services. The gateway refuses any path with an `internal` segment, and the NetworkPolicy opens `:8081` only to pods listed in `networkPolicy.internalFromPods`.
- Startup: `app.Run(name, setup)` (`app.RunPublic` for the gateway, the only service open to the outside: it starts a new trace instead of continuing the client's) builds an `app.Service` (`Postgres`, `PostgresNoMigrations`, `Redis`, `Probes`: `s.Probes.Add(name, check)` adds a check to `/readyz`), runs `setup` and then `httpx.Serve` for both ports with graceful shutdown. `pg` is split into `db.go` (`DB`, `Do`, `Conn`, `Read`), `keyset.go` and `pool.go` (`Open`, pools, migrations).
- Docs: `/api/v1/<service>/docs`, spec at `/api/v1/<service>/openapi.json`.
- Environment variables are read only through `envconfig`: each service declares an `EnvConfig` struct with `env`, `default` and `required` tags and calls `envconfig.Load[EnvConfig]()`, so every problem is reported at once and nothing panics. This is separate from `AppConfig`, the runtime settings kept in the `common` service.
- Lists use keyset pagination: `limit` and an opaque `cursor` query parameter, `next_cursor` in the response. The request embeds `pagination.Params` (`limit`, `cursor`) and the response body is `pagination.Page[T]`. Storage calls `pg.Keyset` (ordered by `created_at DESC, id DESC`, fetches `limit+1`) and the service or handler calls `pagination.Fetch(ctx, params, find, cursorOf)`; `pagination.Map` converts the items to DTOs.
- Endpoints that must be safe to retry take an `Idempotency-Key` header and build `idempotency.New(scope, key, payload)`; the key is stored hashed together with the scope (the owner) and a fingerprint of the payload, a replay with another payload fails with `idempotency_key_reused`.
- `openapi/*.yaml` is generated from the code by `make openapi`, never edited by hand. CI fails if it differs from the code, `e2e` validates real responses against it.
- Database access lives only in `storage/` and goes through GORM (`*pg.DB`: `Write` for writes, `pg.Read` for reads that prefer the replica and fall back to the primary). GORM row structs with tags stay private in `storage/` and are mapped to `model/` structs; depguard rejects GORM and pgx anywhere else.
- Schema changes are plain SQL files in `storage/migrations` named `<yyyymmddhhmmss>_<name>.sql`, never GORM `AutoMigrate`. While nothing is deployed the files only create objects (tables, indexes, views, seed rows). A file named `*.notx.sql` runs outside a transaction and must hold one statement (`CREATE INDEX CONCURRENTLY IF NOT EXISTS ...`); if it fails and leaves an invalid index, drop it with `DROP INDEX CONCURRENTLY IF EXISTS` in a new `*.notx.sql` before retrying.
- With the CloudNativePG pooler (`postgres.pooler.enabled`) services connect through PgBouncer in transaction mode: `DATABASE_URL` points to `postgres-pooler-rw`, `DATABASE_READ_URL` to `postgres-pooler-ro`, and `DATABASE_DIRECT_URL` to `postgres-rw`, used for migrations only (they need a session for the advisory lock). The migrator also runs `ALTER ROLE CURRENT_USER SET search_path`, because PgBouncer cannot carry `search_path` per connection. Pool sizes are derived in `helm/postgres/templates/pooler.yaml` from `max_connections`, the pooler instances and the number of database services, so scaling pods never raises the number of server connections. Never rely on session state (`SET`, advisory locks, `LISTEN`) in storage code.
- Atomic multi-step changes: the service declares `type transactor interface { Do(ctx, func(ctx) error) error }` (satisfied by `*pg.DB`) and wraps the storage calls in `s.tx.Do`; storage methods get their connection with `db.Conn(ctx)` for writes and `pg.Read(ctx, db, ...)` for reads, which join the transaction found in `ctx`. Do HTTP calls to other services before `Do`, never inside it.
- An id that is not a UUID never reaches Postgres: storage methods check it with `pg.IsUUID` and answer the same not found as for a missing row (services and handlers do not know about it).
- Feature packages never import each other; what they share lives in `model/` and `storage/`.
- A growing `service.go` splits per use case inside the same package (`register.go`, `login.go`). A feature that only reads data (admin `users`, `orders`, `scheduler`) has no `service.go`: its handler talks to a storage type or a client directly.
- Each service owns one Postgres schema and one migration history in `storage/migrations`.
- Services never import another service's `internal/<service>` packages (enforced by depguard). They call each other over HTTP through `internal/clients/<service>` (`users.New(url)`), a thin package over `httpx.ServiceClient`: one line per route, `httpx.Get[T](ctx, c.api, path)` or `httpx.Post[T](ctx, c.api, path, body)`, with a struct that mirrors only the fields the caller needs. The client decodes the upstream error into `httpx.Error`, so a caller returns it as is (`users.ErrNotFound` is only a code to check with `errors.Is`); a network failure or a 5xx becomes `httpx.ErrUnavailable`; a missing app config key is a setup problem of the caller, so `common.Bool` returns a plain error (500 `internal_error`, logged) instead of the upstream 404 with the service name in `details` and the real cause kept for logs.
- Migrations must stay compatible with the previous release during rolling updates: add first, switch code in the next release, drop last.
- `common` holds what is shared by the whole app: `appconfig` (runtime settings, key and JSON value, cached in redis for `APP_CONFIG_CACHE_TTL`; public ones are served at `/api/v1/common/app-config`, any key to services at `/internal/v1/common/app-config/{key}` through `internal/clients/common`) and `health` (`/api/v1/common/health` is public and answers only `ok` or `unhealthy`; `/api/v1/common/health/details` needs a token with the admin role and lists every service, postgres and redis with latency, error and the failing checks from the service's `/readyz`). It scales like any other service because every service depends on it. Example use: `users` refuses sign-ups when `registration_enabled` is false.
- Admin reads other services only through views they own: `users.admin_users` and `orders.admin_orders`, created by each service's migrations. A view is a contract: add columns freely, but a migration that drops or retypes a column the view uses fails (Postgres refuses), so change the view first. `contract/` runs the real migrations of users and orders and reads them through the admin storage.
- `admin` is the exception that reads every schema: its Postgres role is in `pg_read_all_data` (`readAllSchemas: true` in `common.yaml`), so it can read but never write other services' tables. It runs as exactly one pod (`singleton: true` in its `values`: no HPA, no PDB, `Recreate` rollout; for other services `replicas: 0` means HPA, a positive number pins the replicas without autoscaling) and its routes `/api/v1/admin/...` pass the gateway only with a token whose subject has the `admin` role.

## Gateway

`internal/gateway` is a pipeline of small packages; `setup.go` only wires them, outermost first:

- `edge`: real client IP (`X-Forwarded-For` is trusted only from `TRUSTED_PROXIES`), CORS, request body limit.
- `policy`: finds the rule of the request (`policy.Resolve`), which also becomes the `route` label of the metrics (`GET /api/v1/orders/*`, never the raw path).
- `auth`: JWT verification and roles (below).
- `validate`: rejects requests that break the OpenAPI spec (types, formats, limits, unknown fields) with 400 before they reach a service; `REQUEST_VALIDATION=false` turns it off.
- `cache`: answers repeated public GETs from memory (ETag and 304, one upstream call for concurrent misses, serves the last copy when the service fails).
- `proxy`: reverse proxy per service with a route timeout (504), a circuit breaker and a concurrency limit per service (503), retries of GET from `httpx.NewTransport`.
- `docs`: `/api/docs` and `/api/openapi.json`, the merged public API of every service.
- `model`: all gateway errors.

Policy lives next to the handler. Public API operations are registered with `httpx.Register(api, op, httpx.AccessPublic|AccessUser|AccessAdmin, handler, options...)` where the options are `httpx.WithTimeout`, `httpx.WithMaxBody` and `httpx.WithCache` (the only uses so far: `create-order` and `list-public-app-config`). They are written to the spec as `x-access`, `x-timeout`, `x-max-body` and `x-cache-ttl`, and `make openapi` turns the specs into `internal/gateway/policy/rules_gen.go` and `internal/gateway/docs/openapi_gen.json`. CI fails when they are stale and the generator fails for an operation registered without access. A `HEAD` request is checked by the rule of the same `GET` route, because the services answer `HEAD` with their `GET` handler. Anything without a rule needs a JWT; `docs` and `openapi` paths are public and `/api/v1/admin/**` always needs the admin role (`static` in `policy/policy.go`). Never edit generated files.

- Every `/api/v1/...` request that is not public carries `Authorization: Bearer <jwt>`. The key comes from one of `JWT_SECRET` (HS*), `JWT_PUBLIC_KEY` (PEM) or `JWT_JWKS_URL` (an identity provider's key set: fetched at start, refreshed every `JWT_JWKS_REFRESH` and on an unknown `kid`, at most one attempt per 10 s, successful or not, however many requests bring an unknown `kid`); `exp` and `sub` are required, `JWT_ISSUER` and `JWT_AUDIENCE` are optional. Tokens are issued outside, nothing here issues them.
- Roles belong to the `users` service (`users/role`, `GET/PUT/DELETE /api/v1/users/{id}/roles[/{role}]` for admins, `GET /internal/v1/users/{id}/roles` for the gateway). Every user has `user`; `admin` is granted explicitly; the migration seeds the admin `00000000-0000-4000-8000-000000000001` (`admin@example.com`) for bootstrapping. For `admin` routes the gateway asks users for the roles of the token subject (cached `ROLES_CACHE_TTL`, so a grant or revoke takes up to that long to apply) and answers 403 without the role and 503 when users is down; concurrent lookups of one subject share a single call to users that survives the first client leaving (`context.WithoutCancel`). The `admin` routes also add the `audit` log line.
- The gateway removes `Authorization` (on every route, public ones included) and any client supplied `X-User-ID` and `X-User-Roles`, then sets `X-User-ID` (and `X-User-Roles` on admin routes). Services trust them only because the NetworkPolicy lets nothing but the gateway reach `:8080`.
- Every request log line has `route`, `status`, `duration_ms`, `bytes`, `client_ip`, `user_id` and `upstream`; handlers add fields with `httpx.LogAttrs`, override the metric route with `httpx.SetRoute` and request an `audit` line with `httpx.MarkAudit`.
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

Platform namespaces: `monitoring` (Prometheus, Grafana, Alertmanager, Loki, Tempo, Alloy), `cnpg-system`, `cert-manager` (only with `backups: true`, the Postgres backup plugin needs it), `kube-system`, `cattle-system`.

Nodes are labeled `role=core` (databases, monitoring, services) and `role=edge` (ingress only). Edge nodes carry the taint `role=edge:NoSchedule`, so nothing runs there unless it tolerates it: the gateway (`apps.gateway.values.tolerations`), Traefik and its svclb pods (`infra/deploy/helm/platform/templates/traefik.yaml`, svclb takes tolerations from a Service annotation), the k3s agent upgrade Plan, and DaemonSets that tolerate everything (Alloy, kured, node-exporter). Everything else lands on core without a `nodeSelector`; a new chart needs no placement settings.

Servers are listed in `infra/terraform/envs/<env>.tfvars`: `server` is the control plane (core), `nodes` is a map of extra servers with `location`, `server_type` and `role` (`edge` by default, `core` for more core capacity such as a database replica). The k3s version and the S3 endpoint and bucket for etcd snapshots are read from `environments/common.yaml` (`k3sVersion`, `s3`, an env file may override them), the same values the cluster uses for upgrade plans and Postgres backups. Add or remove entries and run `make infra-apply ENV=<env>`; a new `edge` gets Traefik and its own gateway pod (gateway is a DaemonSet on `role=edge`), core nodes sit in a spread placement group on different hosts. A database replica is a `core` node plus `postgres.instances: 2` in `environments/<env>.yaml`.

Core servers are protected from deletion and rebuild (`protected`, true by default for `server` and `core` nodes; set `protected = false` before `make infra-destroy` or when replacing one) and `backups = true` turns on Hetzner snapshots for a server. Edge nodes join with a separate agent token, so a compromised edge cannot join the cluster as a control-plane server. etcd snapshots go to S3 only when `backups: true` (`false` by default in `common.yaml`; set it in an env file once a real S3 exists, the same flag turns on Postgres backups); without it they stay on the server disk and die with the server, which is fine for a test `dev`. SSH (22) and the Kubernetes API (6443) are open to the internet: SSH accepts keys only, the API needs a client certificate. The agent token, the edge taint and the CoreDNS manifest come from cloud-init, so they reach only servers created after the change; on existing servers edit `/etc/rancher/k3s/config.yaml` by hand and run `kubectl taint nodes <edge> role=edge:NoSchedule`.

CoreDNS is our own manifest `infra/k3s/coredns.yaml` (k3s CoreDNS is disabled); the edge taint keeps cluster DNS off remote edges. `dev` runs without metrics and traces (`telemetry` in `environments/dev.yaml`): only services, Loki, Alloy and Grafana for logs; the flags drive what `values/derived/monitoring.yaml.gotmpl` turns off.

## Deploy config

- `environments/common.yaml` `apps` is the registry of services; one entry per app, flags: `database`, `redis`, `route` (public: the gateway proxies `/api/v1/<name>` to it), `healthCheck` (the app polls every other non-gateway app), `readAllSchemas`, `gateway`, `calls` (the apps it calls over the internal port). Everything else is derived in `values/derived/app.yaml.gotmpl`: database and redis settings, `ROUTES`, `HEALTH_TARGETS`, and the NetworkPolicy (`allowFromPods` from `route` and `healthCheck`, `internalFromPods` from the `calls` of other apps). A new service is one registry entry, a secret per database in the env secrets files, and a `cmd/<service>` directory.
- Chart values of one app go to `apps.<name>.values`; an env file adds its own `apps.<name>.values` on top (maps merge). `appDefaults` of an env applies to every app and loses to `apps.<name>.values`. Chart defaults (`helm/service/values.yaml`) are the fallback: PDB on, NetworkPolicy on, the `monitoring` namespace allowed.
- `values/{base,<env>}/<release>.yaml` exist for vendor charts and for the gateway ingress and rate limits; a missing file is ignored on purpose.
- The public host of an env is `<subdomain>.<domain>`: `domain` is the main domain in `common.yaml`, `subdomain` is `dev` or `prod` in the env file (local has none and answers on any host). The cluster issues no certificates: HTTPS is terminated by Cloudflare, the edge answers with Traefik's default certificate and there is no HTTP to HTTPS redirect on the edge (turn on Always Use HTTPS in Cloudflare).
- Dev and prod are meant to sit behind Cloudflare (local does not): Traefik trusts `X-Forwarded-For` from the Cloudflare ranges in `cloudflare` of `common.yaml` (refresh them from https://www.cloudflare.com/ips when Cloudflare changes them), and the per-route rate limits count the real client IP (last `X-Forwarded-For` entry, IPv6 grouped by /64) instead of the Cloudflare address; a request that bypasses Cloudflare has no such header and shares one bucket per route. Ports 80 and 443 on the edges are open to everyone by default; `cloudflareOnly: true` (in `common.yaml` or an env file) opens them only to the Cloudflare ranges so the origin cannot be reached around Cloudflare. Set the zone to SSL mode Full in Cloudflare. Local has no `subdomain`: limits count the connecting address.
- `backups: true` enables the barman plugin, WAL archiving and the daily backup of Postgres to `s3://<s3.bucket>/<env>/`; `postgres.backupServerName` (default `postgres`) must change after a restore, because the new cluster archives under its own name.
- `telemetry.metrics`, `telemetry.traces` and `telemetry.retention` (`metrics` for Prometheus, `logs` for Loki and Tempo) are the only telemetry switches. `namespace` and `kubeContext` of an env are read by the Makefile too.

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
- Outgoing calls made with `httpx.NewClient(name, timeout)` are traced and counted in `http_client_requests_total` and `http_client_request_duration_seconds`. `NewClient` also retries idempotent requests (GET, HEAD, OPTIONS; twice, with jittered backoff, on network errors and 502/503/504), counted in `http_client_retries_total`; switch it off with `httpx.WithoutRetries()` (the health checks do). `timeout` is the total budget including retries.
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
