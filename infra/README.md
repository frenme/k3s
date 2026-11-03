# infra

Paths are relative to `infra/`; commands run from the repository root.

```
terraform/   Hetzner servers, firewalls, k3s cloud-init
k3d/         the local cluster
k3s/         own k3s manifests (CoreDNS)
deploy/      Helmfile, own charts, settings per environment
```

| ENV | cluster | namespace |
|---|---|---|
| `local` | k3d | `app` |
| `dev` | Hetzner | `app-dev` |
| `prod` | Hetzner | `app-prod` |

## Defaults

- Postgres: **one instance, no replica**. A pooler (PgBouncer) sits in front.
- Backups (etcd snapshots and Postgres) are **off**: `backups: false`.
- The cluster issues **no certificates**: HTTPS is Cloudflare's, SSL mode Full.
- Ports 80 and 443 of the edges are open to everyone (`cloudflareOnly: false`).
- `dev` runs without metrics and traces, only services and logs.
- One core server (control plane, databases, services) and one or more edge servers (Traefik and the gateway only).

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

1. `terraform/envs/<env>.tfvars`: your SSH public key and the servers (`server` is the core, `nodes` are edges).
2. `sops infra/deploy/environments/<env>.secrets.yaml`: `hcloud.token` and the other secrets.
3. `deploy/environments/common.yaml`: your `domain` (the host is `<subdomain>.<domain>`, `dev.` or `prod.`).
4. Images in a registry the servers can pull, built for `linux/amd64`:
   ```bash
   for s in $(ls app/cmd); do
     docker buildx build --platform linux/amd64 --build-arg SERVICE=$s -t $REGISTRY/$s:$TAG --push app
   done
   ```
5. Servers:
   ```bash
   make infra-plan ENV=<env>
   make infra-apply ENV=<env>
   ```
6. Deploy:
   ```bash
   make kubeconfig ENV=<env> HOST=<core ip>
   REGISTRY=<registry> TAG=<tag> make deploy ENV=<env>
   ```
7. Remove everything: set `protected = false` in `server` of the tfvars, `make infra-apply ENV=<env>`, then `make infra-destroy ENV=<env>`. Servers are billed by the hour.

In Cloudflare: proxy the domain and set SSL/TLS to Full.

## Changing the setup

| I want | Do |
|---|---|
| a new service | one entry in `deploy/environments/common.yaml` `apps` and `secrets.postgres.<name>`, see below |
| an edge in another region | add an entry to `nodes` in the tfvars, `make infra-apply` |
| a Postgres replica | a `nodes` entry with `role = "core"` and `postgres.instances: 2` in `deploy/environments/<env>.yaml` |
| backups | a real S3 in `s3` of `common.yaml`, keys in the secrets, `backups: true` |
| origin reachable only through Cloudflare | `cloudflareOnly: true` |

Failover to a replica needs the Kubernetes API, which lives on the core server: if that server dies, the replica is not promoted.

## The service registry

`deploy/environments/common.yaml` `apps` has one entry per service. `<name>: {}` is a typical service: its own database schema, redis and a public route `/api/v1/<name>` through the gateway. Everything else is derived. Options and their meaning: `deploy/environments/apps.schema.json` (a typo or a wrong type fails the render). A new service needs, besides the entry, `secrets.postgres.<name>` (plain in `local.yaml`, `sops` in dev and prod) and its name in `.gitlab-ci.yml` (`images` and `scan`).

## CI

A `v*` tag builds and scans the images, deploys dev, then prod by hand. The deploy job needs the CI/CD variables `KUBECONFIG` (File, with the contexts `k3s-dev` and `k3s-prod`) and `SOPS_AGE_KEY` (the private age key).

Rules for agents and deeper details: [AGENTS.md](AGENTS.md).
