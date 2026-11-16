# infra

Paths are relative to `infra/`; commands run from the repository root.

```
terraform/   Hetzner servers, firewalls, k3s cloud-init
k3d/         the local cluster
deploy/      Helmfile, own charts, settings per environment
```

| ENV | cluster | namespace |
|---|---|---|
| `local` | k3d | `app` |
| `dev` | Hetzner | `app-dev` |
| `prod` | Hetzner | `app-prod` |

## Defaults

- Postgres: one instance, no replica; PgBouncer in front.
- Backups (etcd and Postgres) are off: `backups: false`.
- No certificates in the cluster: HTTPS is Cloudflare's, SSL mode Full.
- Ports 80 and 443 of the edges are open to everyone (`cloudflareOnly: false`).
- `dev` runs without metrics and traces: services, logs and a plain Grafana.
- One core server (control plane, databases, services) and one or more edge servers (Traefik and the gateway only), or one server for everything: `edge = true` inside `server` and no `nodes`.
- Database passwords are derived from `secrets.dbSecret` and the service name, not stored.
- Automatic reboots and k3s upgrades (`autoUpdates`) are on in prod only.

## Local

```bash
make up         # build images, create k3d, deploy
make e2e        # tests
make status     # pods and autoscalers
make logs s=users
make restart s=gateway
make down
```

## Dev or prod

1. Once per project: `make init`. It creates your age key for sops (back it up), puts it in `.sops.yaml`, asks for your `domain` (hosts are `dev.<domain>` and `prod.<domain>`) and puts your SSH public key into the tfvars. `DOMAIN`, `SSH_KEY_FILE` and `YES=1` make it non-interactive. Secrets encrypted for the old key become unreadable: recreate them in step 3.
2. Edit `terraform/envs/<env>.tfvars`: `server` is the core, `nodes` are edges.
3. `make secrets ENV=<env>` asks for the Hetzner token, makes `dbSecret`, `redis`, `jwt`, `grafana` and encrypts them.
4. `make push REGISTRY=<registry> TAG=<tag>` builds every service of `app/cmd` for `linux/amd64` into a registry the servers can pull.
5. `make bootstrap ENV=<env> REGISTRY=<registry> TAG=<tag>`: servers, kubeconfig, wait until every node is Ready, deploy. The steps also run alone: `make infra-plan`, `infra-apply`, `kubeconfig` (`HOST=<ip>` overrides the Terraform ip), `wait-nodes`, `deploy`.
6. Remove everything: `protected = false` in the tfvars, `make infra-apply ENV=<env>`, `make infra-destroy ENV=<env>`. Servers are billed by the hour.

In Cloudflare: proxy the domain and set SSL/TLS to Full.

### Your own Ubuntu server instead of Hetzner

```bash
make install-host ROLE=single HOST=<ip>                  # one server for everything
make install-host ROLE=server HOST=<core ip>             # core
make install-host ROLE=edge HOST=<ip> SERVER=<core ip>   # edge (or ROLE=core)
make kubeconfig ENV=<env> HOST=<core ip>
REGISTRY=<registry> TAG=<tag> make deploy ENV=<env>
```

Same k3s version and settings as the cloud-init, but no firewalls: open 22, 80 and 443 to the world yourself, and let the servers reach each other (6443 of the core, UDP 51820 and TCP 10250 between all).

## dev and prod on one domain

Every environment is its own cluster with its own Terraform workspace, secrets and kube context. They can share one Hetzner project, but never deploy two environments to the same cluster (see `AGENTS.md`).

- Apply environments one after another: the Terraform workspace is global to `terraform/`.
- Edges around the world: put every edge IP into `prod.<domain>` as proxied A records. Cloudflare keeps a client on one of them and switches within seconds when it dies; it does not pick the nearest (that needs Cloudflare Load Balancing).
- A gateway far from the core pays about two round trips to the core per authenticated call: +0.2 s from the US, +0.35 s from Singapore (2 ms next to the core).
- The Ingress serves only `<subdomain>.<domain>`; any other hostname answers 404.

## Changing the setup

| I want | Do |
|---|---|
| a new service | one line in `deploy/environments/common.yaml` `apps`: `<name>: {}` (options: `deploy/environments/apps.schema.json`) |
| an edge in another region | add an entry to `nodes` in the tfvars, `make infra-apply` |
| remove a node | delete its entry, `make infra-apply`, then `kubectl delete node k3s-<env>-<name>` |
| a Postgres replica | a `nodes` entry with `role = "core"` and `postgres.instances: 2` in the env file (failover needs the API on the core server) |
| backups | a real S3 in `s3` of `common.yaml`, keys in the secrets, `backups: true` |
| restore Postgres | with `backups: true`: `releaseValues.postgres.restore: {enabled: true, serverName: <old name>}` in the env file and a new `postgres.backupServerName` |
| origin only through Cloudflare | `cloudflareOnly: true`; refresh the ranges with `make cloudflare-ips` |
| automatic reboots and upgrades | `autoUpdates: true` in the env file |
| a private registry | `registry: {host, username, password}` in the env secrets (`sops`), only for servers created after the change |

## CI

A `v*` tag builds and scans the images, deploys dev, then prod by hand. The deploy job needs the CI/CD variables `KUBECONFIG` (File, with the contexts `k3s-dev` and `k3s-prod`) and `SOPS_AGE_KEY` (the private age key).

Rules for agents and details: [AGENTS.md](AGENTS.md).
