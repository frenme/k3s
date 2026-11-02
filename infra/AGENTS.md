# AGENTS.md of infra/

Guide for AI agents working on the infrastructure and investigating the clusters. Paths are relative to `infra/`; `make` commands run from the repository root (`make help`). The Go services are described in `app/AGENTS.md`.

## Layout

- `deploy/helmfile.yaml.gotmpl` every release; `deploy/environments/{common,local,dev,prod}.yaml` settings (`common.yaml` holds the `apps` registry and defaults, an env file only what differs); `deploy/values/{base,local,dev,prod,derived}` values of vendor charts and of the gateway ingress; `deploy/helm/*` own charts. See Deploy config.
- `terraform` Hetzner servers, `k3d` local cluster, `k3s` own k3s manifests (CoreDNS).

## Environments

| ENV   | kube context   | app namespace |
|-------|----------------|---------------|
| local | `k3d-k3s-base` | `app`         |
| dev   | `k3s-dev`      | `app-dev`     |
| prod  | `k3s-prod`     | `app-prod`    |

Platform namespaces: `monitoring` (Prometheus, Grafana, Alertmanager, Loki, Tempo, Alloy), `cnpg-system`, `cert-manager` (only with `backups: true`, the Postgres backup plugin needs it), `kube-system`, `cattle-system`.

Nodes are labeled `role=core` (databases, monitoring, services) and `role=edge` (ingress only). Edge nodes carry the taint `role=edge:NoSchedule`, so nothing runs there unless it tolerates it: the gateway (`apps.gateway.values.tolerations`), Traefik and its svclb pods (`deploy/helm/platform/templates/traefik.yaml`, svclb takes tolerations from a Service annotation), the k3s agent upgrade Plan, and DaemonSets that tolerate everything (Alloy, kured, node-exporter). Everything else lands on core without a `nodeSelector`; a new chart needs no placement settings.

Servers are listed in `terraform/envs/<env>.tfvars`: `server` is the control plane (core), `nodes` is a map of extra servers with `location`, `server_type` and `role` (`edge` by default, `core` for more core capacity such as a database replica). The k3s version and the S3 endpoint and bucket for etcd snapshots are read from `environments/common.yaml` (`k3sVersion`, `s3`, an env file may override them), the same values the cluster uses for upgrade plans and Postgres backups. Add or remove entries and run `make infra-apply ENV=<env>`; a new `edge` gets Traefik and its own gateway pod (gateway is a DaemonSet on `role=edge`), core nodes sit in a spread placement group on different hosts. A database replica is a `core` node plus `postgres.instances: 2` in `environments/<env>.yaml`.

Core servers are protected from deletion and rebuild (`protected`, true by default for `server` and `core` nodes; set `protected = false` before `make infra-destroy` or when replacing one) and `backups = true` turns on Hetzner snapshots for a server. Edge nodes join with a separate agent token, so a compromised edge cannot join the cluster as a control-plane server. etcd snapshots go to S3 only when `backups: true` (`false` by default in `common.yaml`; set it in an env file once a real S3 exists, the same flag turns on Postgres backups); without it they stay on the server disk and die with the server, which is fine for a test `dev`. The servers have no IPv6 and k3s encrypts Secrets in etcd (`secrets-encryption`). SSH (22) and the Kubernetes API (6443) are open to the internet: SSH accepts keys only, the API needs a client certificate. The agent token, the edge taint and the CoreDNS manifest come from cloud-init, so they reach only servers created after the change; on existing servers edit `/etc/rancher/k3s/config.yaml` by hand and run `kubectl taint nodes <edge> role=edge:NoSchedule`.

CoreDNS is our own manifest `k3s/coredns.yaml` (k3s CoreDNS is disabled); the edge taint keeps cluster DNS off remote edges. `dev` runs without metrics and traces (`telemetry` in `environments/dev.yaml`): only services, Loki, Alloy and Grafana for logs; the flags drive what `values/derived/monitoring.yaml.gotmpl` turns off.

## Deploy config

- `environments/common.yaml` `apps` is the registry of services, one entry per app. Every app except the gateway has a database (own schema, role and `secrets.postgres.<name>`), redis and a public route (`/api/v1/<name>` proxied by the gateway) by default, so a typical service is just `name: {}`. `database: false`, `redis: false` and `route: false` opt out (the scheduler has no public route). The only special flags: `gateway` (the single public entry point), `readAllSchemas` (admin) and `healthCheck` (`common` polls `/readyz` of every other app, which needs `:8080` open to it). Everything else is derived in `values/derived/app.yaml.gotmpl`: database and redis settings, `ROUTES`, `HEALTH_TARGETS` and the NetworkPolicy (`:8080` from the gateway and the `healthCheck` app, `:8081` from every app). The allowed keys, their types, defaults and meaning are in `deploy/environments/apps.schema.json`: editors that support it (the modeline at the top of `common.yaml` and `prod.yaml`) autocomplete and flag mistakes, and Helmfile rejects an unknown key or a flag that is not `true` or `false` in an entry, so a typo fails the render instead of being ignored. A new flag is added to that file first. A new service is one registry entry, a secret per database in the env secrets files (`local.yaml` in plain text), the name in the image matrix of `.gitlab-ci.yml` and a `cmd/<service>` directory.
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
