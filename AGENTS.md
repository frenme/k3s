# AGENTS.md

A template backend: Go microservices and the infrastructure they run on (k3s on Hetzner, k3d locally).

- `app/` the Go code. Read `app/AGENTS.md` before changing it.
- `infra/` Terraform, k3d, Helmfile and charts. Read `infra/AGENTS.md` before changing it.
- The root holds the `Makefile` (every command, `make help`), `.gitlab-ci.yml`, `renovate.json` and `.sops.yaml`.

Rules for every task:

- Work only in the part you were asked about.
- Never print or copy secrets, connection strings or tokens.
- Investigating a cluster is read-only unless the user asks for an action.
