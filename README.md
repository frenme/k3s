# k3s

A template (boilerplate) backend: Go microservices behind an edge gateway, running on k3s. Locally it runs in k3d, remotely on Hetzner servers created with Terraform.

```
user -> Cloudflare -> edge (Traefik, gateway) -> services on core -> Postgres, Redis
```

- `app/` the Go services: `gateway`, `users`, `orders`, `common`, `admin`, `scheduler`. See [app/README.md](app/README.md).
- `infra/` Terraform, k3d, Helmfile and charts, monitoring. See [infra/README.md](infra/README.md).
- `Makefile` every command, `make help` lists them.

## Quick start

Tools: `make doctor` shows what is missing (go, docker, k3d, kubectl, helm, helmfile, sops, age, terraform ...).

```bash
make up        # build images, create the local k3d cluster, deploy everything
make e2e       # end to end tests against it
make down      # delete the cluster
```

Checks before a commit: `make lint test test-integration`.

## Agents

AI agents read `AGENTS.md`, then `app/AGENTS.md` or `infra/AGENTS.md` for the part they work on.
