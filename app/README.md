# app

The Go services of the template. One module (`github.com/egor/k3s-base`), one binary per service in `cmd/`.

| Service | Role |
|---|---|
| `gateway` | public entry point: JWT, roles, validation, cache, proxy; runs on the edges |
| `users` | users and roles |
| `orders` | orders (uses `users` over the internal port) |
| `common` | runtime settings and the aggregated health of the whole app |
| `admin` | read only view over every schema, admin role only |
| `scheduler` | cron jobs |

## Layout

```
cmd/<service>      entrypoint
internal/<service> the service: model, storage (GORM, SQL migrations), features with v1/ handlers
internal/clients   HTTP clients of the services
internal/lib       shared code: app bootstrap, httpx, pg, cache, lock, pagination, idempotency
openapi/           OpenAPI specs generated from the code (make openapi)
e2e/               tests against a running cluster
Dockerfile         builds one service: --build-arg SERVICE=<name>, context is this folder
```

## Commands

Run from the repository root:

```bash
make test               # unit tests
make test-integration   # tests with real Postgres and Redis (needs docker)
make lint               # gofmt, golangci-lint and helmfile lint
make openapi            # regenerate openapi/*.yaml and the gateway rules
make e2e                # tests against the local cluster (after make up)
make images             # build the container images
```

## Adding a service

1. `cmd/<name>` and `internal/<name>` next to the existing ones, following `internal/users`.
2. One entry in the service registry of the infrastructure, see [../infra/README.md](../infra/README.md).
3. The name in the image matrix of `.gitlab-ci.yml`.

The conventions (errors, validation, pagination, migrations, gateway policy) are in [AGENTS.md](AGENTS.md).
