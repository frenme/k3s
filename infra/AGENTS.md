# AGENTS.md of infra/

Rules for the infrastructure and for investigating the clusters. Paths are relative to `infra/`; `make` commands run from the repository root. The services: `app/AGENTS.md`.

## Layout

- `deploy/helmfile.yaml.gotmpl` every release; `deploy/environments/{common,local,dev,prod}.yaml` settings (`common.yaml` has the `apps` registry and the defaults, an env file only what differs); `deploy/values/{base,<env>,derived}` values of vendor charts and of the gateway ingress; `deploy/helm/*` own charts.
- `terraform` Hetzner servers, `k3d` the local cluster, `k3s` own k3s manifests (CoreDNS).

| ENV | kube context | namespace |
|---|---|---|
| local | `k3d-k3s-base` | `app` |
| dev | `k3s-dev` | `app-dev` |
| prod | `k3s-prod` | `app-prod` |

Platform namespaces: `monitoring`, `cnpg-system`, `cert-manager` (only with `backups: true`), `kube-system`, `cattle-system`.

## Servers

- `terraform/envs/<env>.tfvars`: `server` is the core (k3s control plane, databases, services), `nodes` is a map of extra servers with `location`, `server_type` and `role` (`edge` by default, `core` for more capacity). `make infra-apply ENV=<env>`.
- Nodes are labeled `role=core` or `role=edge`. Edges carry the taint `role=edge:NoSchedule`: only Traefik, the gateway (a DaemonSet) and DaemonSets that tolerate everything run there; everything else lands on core without any placement setting.
- Core servers are protected from deletion (`protected`, true by default): set `protected = false` and apply before `make infra-destroy`. `backups = true` adds Hetzner snapshots.
- Edges join with a separate agent token, so a compromised edge cannot join as a control-plane server. The servers have no IPv6, k3s encrypts Secrets in etcd, SSH (22) and the API (6443) are open to the internet (SSH accepts keys only).
- The agent token, the edge taint and the CoreDNS manifest come from cloud-init, so they reach only servers created after the change; on an existing server edit `/etc/rancher/k3s/config.yaml` by hand.
- CoreDNS is our own `k3s/coredns.yaml` (the k3s one is disabled), pinned to core by the edge taint.

## Defaults

- Postgres: one instance, no replica, PgBouncer in front. A replica is a `core` node plus `postgres.instances: 2`; its failover needs the Kubernetes API, which lives on the core server.
- `backups: false` in `common.yaml`. `backups: true` turns on etcd snapshots to S3 (`s3` section, keys in the secrets), the barman plugin, WAL archiving and a daily Postgres backup. After a restore change `postgres.backupServerName`.
- The cluster issues no certificates: HTTPS is Cloudflare's (SSL mode Full), the edge answers with Traefik's default certificate, there is no HTTP to HTTPS redirect on the edge.
- Host of an env: `<subdomain>.<domain>`; `domain` is in `common.yaml`, `subdomain` (`dev`, `prod`) in the env file, local has none.
- Dev and prod expect Cloudflare: Traefik trusts `X-Forwarded-For` from the ranges in `cloudflare` of `common.yaml` (refresh them from cloudflare.com/ips) and rate limits count the real client IP. Ports 80 and 443 of the edges are open to everyone; `cloudflareOnly: true` opens them only to those ranges.
- `telemetry.metrics`, `telemetry.traces`, `telemetry.retention` are the only monitoring switches; `dev` runs with metrics and traces off (services, Loki, Alloy, Grafana).

## Service registry

- `deploy/environments/common.yaml` `apps`: one entry per service. Every app except the gateway gets a database (own schema and role), redis and a public route `/api/v1/<name>` by default, so a typical service is `<name>: {}`.
- Keys, types and meaning are in `deploy/environments/apps.schema.json`; Helmfile rejects an unknown key or a flag that is not `true` or `false`. A new flag goes to that file first. Special keys: `gateway`, `readAllSchemas` (admin), `healthCheck` (`common`), `values` (chart values of the service, merged over `appDefaults` and the env's own `apps.<name>.values`).
- Derived in `values/derived/app.yaml.gotmpl`: database and redis settings, `ROUTES`, `HEALTH_TARGETS` and the NetworkPolicy (`:8080` only from the gateway and the `healthCheck` app, `:8081` from every app).
- A new service also needs `secrets.postgres.<name>` (plain in `local.yaml`, sops in `dev.secrets.yaml` and `prod.secrets.yaml`), its name in `.gitlab-ci.yml` (`images` and `scan`) and, for its own rate limit, a route in `values/base/gateway.yaml`.
- `values/{base,<env>}/<release>.yaml` are for vendor charts and the gateway ingress; a missing file is ignored on purpose.

## Connecting

Read-only kubectl and Grafana MCP:

```bash
make agent-kubeconfig ENV=prod
export KUBECONFIG=$PWD/kubeconfig-agent-prod.yaml
```

```json
{"mcpServers": {"grafana-prod": {"command": "make", "args": ["-s", "-C", "/path/to/k3s", "mcp", "ENV=prod"]}}}
```

The `agent` ServiceAccount has `view` plus read access to nodes, volumes, metrics and the CloudNativePG, Prometheus, Traefik and upgrade resources; no Secrets, exec, port-forward or changes. The MCP server is read-only and needs the operator kubeconfig and Docker.

## Queries

Loki labels: `namespace`, `pod`, `container`, `app`, `node`, `stream`, `service_name`; level is `detected_level`, filter it after the selector. Datasource uids: `prometheus`, `loki`, `tempo`.

```logql
{namespace="app-prod"} | detected_level="error"
{namespace="app-prod", app="gateway"} | json | msg="request" | duration_ms > 500
{namespace="app-prod"} |= "<trace_id>"
```

```traceql
{ resource.service.name = "orders" && status = error }
{ resource.service.name = "users" } >> { span.db.system.name = "postgresql" && duration > 100ms }
```

```promql
sum by (service) (rate(http_requests_total{namespace="app-prod", code="5xx"}[5m]))
histogram_quantile(0.99, sum by (service, le) (rate(http_request_duration_seconds_bucket{namespace="app-prod"}[5m])))
ALERTS{alertstate="firing"}
```

## Investigating

1. Firing alerts and the error rate by service.
2. Error logs of the service for the time window; take a `trace_id` from them or an exemplar of the latency panel.
3. Open the trace, find the failing or slow span (upstream call, SQL, redis).
4. Cluster state: `kubectl get pods -n <ns>`, `describe pod`, `get events --sort-by=.lastTimestamp`, `logs --previous`, `top pods`.
5. Database: `kubectl get clusters.postgresql.cnpg.io -n <ns> postgres -o yaml`, replication lag and WAL archiving metrics.
6. Report the cause with links to the logs, the trace and the likely commit.

## Rules

- Investigation is read-only: never apply, delete, scale, restart, exec, port-forward or `make deploy` against dev or prod unless asked.
- Never print or copy secrets, connection strings or tokens, even when a log line leaks one; report the leak.
- Keep Loki queries scoped to a namespace and a short time range.
