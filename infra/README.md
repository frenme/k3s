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

- Postgres: **one instance, no replica**. PgBouncer is always in front.
- Backups (etcd snapshots and Postgres) are **off**: `backups: false`.
- The cluster issues **no certificates**: HTTPS is Cloudflare's, SSL mode Full.
- Ports 80 and 443 of the edges are open to everyone (`cloudflareOnly: false`).
- `dev` runs without metrics and traces: services, logs and a plain Grafana.
- One core server (control plane, databases, services) and one or more edge servers (Traefik and the gateway only). Or **one server for everything**: `edge = true` inside `server` and no `nodes`.
- Database passwords are not stored: each is derived from `secrets.dbSecret` and the service name.
- Automatic node reboots and k3s upgrades (`autoUpdates`) are **on in prod only**.

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

1. First setup, once per project:
   ```bash
   make init
   ```
   It creates your age key for sops (back it up) and puts it in `.sops.yaml`, asks for your `domain` in `deploy/environments/common.yaml` (the host is `<subdomain>.<domain>`, `dev.` or `prod.`) and puts your SSH public key into the tfvars. `DOMAIN`, `SSH_KEY_FILE` and `YES=1` make it non-interactive. Secrets encrypted for the old key become unreadable: recreate them with step 3.
2. `terraform/envs/<env>.tfvars`: the servers (`server` is the core, `nodes` are edges; `edge = true` inside `server` and no `nodes` is one server for everything).
3. Secrets (asks for the Hetzner token, makes `dbSecret`, `redis`, `jwt`, `grafana` and encrypts them with the age key of `.sops.yaml`):
   ```bash
   make secrets ENV=<env>
   ```
4. Images in a registry the servers can pull (built for `linux/amd64`, every service of `app/cmd`):
   ```bash
   make push REGISTRY=<registry> TAG=<tag>
   ```
5. Everything else in one command: servers, kubeconfig, wait until every node is Ready, deploy:
   ```bash
   make bootstrap ENV=<env> REGISTRY=<registry> TAG=<tag>
   ```
   The steps are also separate: `make infra-plan`, `make infra-apply`, `make kubeconfig` (waits for k3s, takes the ip from Terraform; `HOST=<ip>` overrides it), `make wait-nodes`, `make deploy`.
6. Remove everything: set `protected = false` at the top of the tfvars, `make infra-apply ENV=<env>`, then `make infra-destroy ENV=<env>`. Servers are billed by the hour.

In Cloudflare: proxy the domain and set SSL/TLS to Full.

### Your own Ubuntu server instead of Hetzner

On a clean Ubuntu with SSH access (no Terraform, no Hetzner token):

```bash
make install-host ROLE=single HOST=<ip>            # one server for everything
make install-host ROLE=server HOST=<core ip>       # core
make install-host ROLE=edge HOST=<ip> SERVER=<core ip>   # edge (or ROLE=core)
make kubeconfig ENV=<env> HOST=<core ip>
REGISTRY=<registry> TAG=<tag> make deploy ENV=<env>
```

It installs k3s of the version from `common.yaml` with the same settings as the Terraform cloud-init. It does not set up firewalls: open 22, 80 and 443 to the world yourself, and let the servers reach each other (6443 of the core, UDP 51820 and TCP 10250 between all).

## Changing the setup

| I want | Do |
|---|---|
| a new service | one line in `deploy/environments/common.yaml` `apps`, see below |
| an edge in another region | add an entry to `nodes` in the tfvars, `make infra-apply` |
| a Postgres replica | a `nodes` entry with `role = "core"` and `postgres.instances: 2` in `deploy/environments/<env>.yaml` |
| backups | a real S3 in `s3` of `common.yaml`, keys in the secrets, `backups: true` |
| origin reachable only through Cloudflare | `cloudflareOnly: true`; refresh the ranges now and then with `make cloudflare-ips` |
| automatic reboots and k3s upgrades | `autoUpdates: true` in the env file |

Failover to a replica needs the Kubernetes API, which lives on the core server: if that server dies, the replica is not promoted.

## The service registry

`deploy/environments/common.yaml` `apps` has one entry per service. `<name>: {}` is a typical service: its own database schema, redis and a public route `/api/v1/<name>` through the gateway. Everything else is derived. Options and their meaning: `deploy/environments/apps.schema.json` (a typo or a wrong type fails the render). Nothing else is needed: the database password is derived from `secrets.dbSecret`, and CI builds and scans every folder of `app/cmd`. Its own rate limit is a route in `apps.gateway.values.ingress.routes` of the same file.

## CI

A `v*` tag builds and scans the images, deploys dev, then prod by hand. The deploy job needs the CI/CD variables `KUBECONFIG` (File, with the contexts `k3s-dev` and `k3s-prod`) and `SOPS_AGE_KEY` (the private age key).

Rules for agents and deeper details: [AGENTS.md](AGENTS.md).
