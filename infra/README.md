# infra

Everything the services run on.

```
terraform/   Hetzner servers (core, edge nodes), firewalls, k3s cloud-init
k3d/         the local cluster
k3s/         own k3s manifests (CoreDNS pinned to the core)
deploy/      Helmfile, own charts, settings per environment
```

## Environments

| ENV | cluster | namespace |
|---|---|---|
| `local` | k3d | `app` |
| `dev` | Hetzner | `app-dev` |
| `prod` | Hetzner | `app-prod` |

Dev and prod are meant to sit behind Cloudflare. A host is `<subdomain>.<domain>` (`dev.` or `prod.`), `domain` is set in `deploy/environments/common.yaml`. HTTPS is terminated by Cloudflare (SSL mode Full), the cluster issues no certificates.

## Local

`make up` from the repository root builds the images, creates the k3d cluster and deploys. `make down` removes it.

## A remote environment

1. `terraform/envs/<env>.tfvars`: your SSH key and the servers (`server` is the core, `nodes` are edges or more cores).
2. `sops deploy/environments/<env>.secrets.yaml`: the Hetzner token (`hcloud.token`) and the rest of the secrets.
3. `deploy/environments/common.yaml`: your `domain`; `backups: true` once a real S3 exists (`s3` section).
4. Images in a registry the servers can pull (built for `linux/amd64`), then `REGISTRY` and `TAG` for the deploy.
5. `make infra-plan ENV=<env>`, `make infra-apply ENV=<env>`.
6. `make kubeconfig ENV=<env> HOST=<core ip>`, then `REGISTRY=... TAG=... make deploy ENV=<env>`.

Servers are billed by the hour. Core servers are protected from deletion: set `protected = false` in the tfvars and apply before `make infra-destroy ENV=<env>`.

## CI deploy

On a `v*` tag GitLab builds and scans the images, deploys dev and then prod (manual). The deploy job needs two CI/CD variables:

- `KUBECONFIG` (type File): a kubeconfig that has the contexts `k3s-dev` and `k3s-prod` (`make kubeconfig ENV=<env> HOST=<core ip>` creates one per environment, merge them).
- `SOPS_AGE_KEY` (masked): the private age key that decrypts `deploy/environments/*.secrets.yaml`. Without it the `helm` job lints only `local`.

The registry login comes from GitLab itself.

## The service registry

`deploy/environments/common.yaml` `apps` has one entry per service, `name: {}` for a typical one (database, redis and a public route are on by default). The allowed keys are described in `deploy/environments/apps.schema.json`; a new service also needs `secrets.postgres.<name>` in the secrets files.

Details for agents and for a deeper reading: [AGENTS.md](AGENTS.md).
