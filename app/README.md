# app

Go microservices of the template. One module (`github.com/egor/k3s-base`), one binary per service in `cmd/`.

| Service | What it does |
|---|---|
| `gateway` | the only public entry point: JWT, roles, request validation, cache, proxy to the services |
| `users` | users and roles |
| `orders` | orders of a user |
| `common` | runtime settings and the aggregated health of the app |
| `admin` | read-only view over every schema, admin role only |
| `scheduler` | cron jobs |

## Layout

```
cmd/<service>/main.go   entrypoint
internal/<service>/     model, storage (GORM + SQL migrations), features with v1/ handlers
internal/clients/       HTTP clients of the services
internal/lib/           shared code: app bootstrap, httpx, pg, cache, lock, pagination, idempotency
openapi/                OpenAPI specs, generated from the code
e2e/                    tests against the local cluster
Dockerfile              builds one service: --build-arg SERVICE=<name>, build context is app/
```

## Commands

Run from the repository root.

```bash
make test               # unit tests
make test-integration   # real Postgres and Redis in docker
make lint               # golangci-lint and helmfile lint
make openapi            # regenerate openapi/ and the gateway rules
make up                 # build images, create the local k3d cluster, deploy
make e2e                # end to end tests against the local cluster
make down               # delete the local cluster
```

Before handing over: `make lint test test-integration openapi`, and `make up e2e` when a service changed.

## Adding a service

1. Code: `cmd/<name>/main.go` and `internal/<name>/`, copy the structure of `internal/users`.
2. Infra, otherwise the service is built but never deployed:
   - one line in `infra/deploy/environments/common.yaml`: `<name>: {}` (database, redis and a public route are on by default);
   - the database password `secrets.postgres.<name>`: plain in `infra/deploy/environments/local.yaml`, with `sops` in `dev.secrets.yaml` and `prod.secrets.yaml`;
   - the name in the `SERVICE: [...]` list of `images` and `scan` in `.gitlab-ci.yml`.
3. `make openapi`, then `make up` and `make e2e`.

The coding rules (errors, validation, pagination, migrations, gateway policy) are in [AGENTS.md](AGENTS.md).
