# AGENTS.md

Guide for AI agents (Cursor, Claude Code, Codex) working on this repository and investigating its clusters.

## Layout

- `cmd/<service>` entrypoints, `internal/<service>` handlers and stores, `internal/lib` shared code (app bootstrap, httpx, pg, rds, logx, tracing).
- `api/openapi.yaml` public API, validated by `e2e`.
- `infra/deploy/helmfile.yaml.gotmpl` every release; `infra/deploy/environments/*.yaml` per-env settings; `infra/deploy/values/{base,local,dev,prod,derived}` release values; `infra/deploy/helm/*` own charts.
- `infra/terraform` Hetzner servers, `infra/k3d` local cluster.
- `make help` lists every command.

## Environments

| ENV   | kube context   | app namespace |
|-------|----------------|---------------|
| local | `k3d-k3s-base` | `app`         |
| dev   | `k3s-dev`      | `app-dev`     |
| prod  | `k3s-prod`     | `app-prod`    |

Platform namespaces: `monitoring` (Prometheus, Grafana, Alertmanager, Loki, Tempo, Alloy), `cnpg-system`, `cert-manager`, `kube-system`, `cattle-system`.

Nodes are labeled `role=core` (databases, monitoring, services) and `role=edge` (ingress only).

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
{ span.http.route = "/orders/{id}" && span.http.response.status_code >= 500 }
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
