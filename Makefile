SHELL := /bin/bash

ENV ?= local
CLUSTER ?= k3s-base
NAMESPACE ?= $(if $(filter local,$(ENV)),app,app-$(ENV))
TAG ?= dev
REGISTRY ?= k3s-base
CONTEXT ?=
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)

GO_SERVICES := $(notdir $(wildcard cmd/*))
INFRA := postgres redis
APPS := $(filter-out $(INFRA),$(basename $(notdir $(wildcard deploy/values/base/*.yaml))))

KCTX := $(if $(filter local,$(ENV)),k3d-$(CLUSTER),$(CONTEXT))
KUBECTL := kubectl $(if $(KCTX),--context $(KCTX))
HELM := helm $(if $(KCTX),--kube-context $(KCTX))

file_flag = $$([ -f $(1) ] && echo -f $(1))
values = -f deploy/values/base/$(1).yaml $(call file_flag,deploy/values/$(ENV)/$(1).yaml) $(call file_flag,deploy/values/$(ENV)/$(1).secret.yaml)
lint_values = -f deploy/values/base/$(1).yaml $(call file_flag,deploy/values/$(ENV)/$(1).yaml) $(if $(filter local,$(ENV)),,$(call file_flag,deploy/values/secrets.example/$(1).yaml))

.PHONY: help guard local-only fmt vet test lint helm-lint tidy build images import cluster-up cluster-down deploy deploy-infra deploy-apps undeploy secrets-init smoke replica-check loadtest up down status logs restart psql backup-now port-forward clean

help:
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-14s %s\n", $$1, $$2}'

guard:
	@test -n "$(KCTX)" || { echo "ENV=$(ENV) needs CONTEXT=<kube context>"; exit 1; }
	@[ "$(ENV)" = local ] || [ "$(REGISTRY)" != k3s-base ] || { echo "ENV=$(ENV) needs REGISTRY=<image registry>"; exit 1; }
	@[ "$(ENV)" = local ] || [ "$(TAG)" != dev ] || { echo "ENV=$(ENV) needs TAG=<release tag>"; exit 1; }
	@echo "target: env=$(ENV) context=$(KCTX) namespace=$(NAMESPACE) registry=$(REGISTRY) tag=$(TAG)"

local-only:
	@[ "$(ENV)" = local ] || { echo "this target works only with ENV=local"; exit 1; }

fmt: ## format go code
	gofmt -w cmd internal

vet: ## static checks
	go vet ./...

test: ## unit tests
	go test -race -count=1 ./...

helm-lint: ## lint and render charts for ENV (local, dev, prod)
	@helm lint deploy/helm/postgres $(call lint_values,postgres) >/dev/null && echo "ok $(ENV) postgres"
	@helm lint deploy/helm/redis $(call lint_values,redis) >/dev/null && echo "ok $(ENV) redis"
	@for s in $(APPS); do helm lint deploy/helm/service $(call lint_values,$$s) >/dev/null && helm template $$s deploy/helm/service $(call lint_values,$$s) >/dev/null && echo "ok $(ENV) $$s" || exit 1; done

lint: vet helm-lint ## vet and helm lint

tidy: ## tidy go modules
	go mod tidy

build: ## build go binaries into bin/
	@for s in $(GO_SERVICES); do CGO_ENABLED=0 go build -o bin/$$s ./cmd/$$s || exit 1; done

images: ## build and tag container images for REGISTRY and TAG
	@for s in $(GO_SERVICES); do docker build -q -f build/go.Dockerfile --build-arg SERVICE=$$s --build-arg VERSION=$(TAG) --build-arg COMMIT=$(COMMIT) -t $(REGISTRY)/$$s:$(TAG) . || exit 1; done

cluster-up: local-only ## create local k3s cluster (k3d)
	k3d cluster create --config infra/k3d/cluster.yaml --wait
	$(KUBECTL) create namespace $(NAMESPACE) --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) apply -f infra/k3s/traefik-config.yaml

cluster-down: local-only ## delete local cluster
	k3d cluster delete $(CLUSTER)

import: local-only ## load images into local cluster
	k3d image import -c $(CLUSTER) $(addprefix $(REGISTRY)/,$(addsuffix :$(TAG),$(GO_SERVICES)))

secrets-init: ## create <ENV> secret files from examples (fill them in, they are not committed)
	@mkdir -p deploy/values/$(ENV)
	@for f in deploy/values/secrets.example/*.yaml; do n=$$(basename $$f .yaml); [ -f deploy/values/$(ENV)/$$n.secret.yaml ] || cp $$f deploy/values/$(ENV)/$$n.secret.yaml; done
	@ls deploy/values/$(ENV)/*.secret.yaml

deploy-infra: guard ## deploy postgres and redis
	$(HELM) upgrade --install postgres deploy/helm/postgres -n $(NAMESPACE) --create-namespace $(call values,postgres) --wait --timeout 5m
	$(HELM) upgrade --install redis deploy/helm/redis -n $(NAMESPACE) --create-namespace $(call values,redis) --wait --timeout 3m

deploy-apps: guard ## deploy services (one release per deploy/values/base/<name>.yaml)
	@for s in $(APPS); do \
		$(HELM) upgrade --install $$s deploy/helm/service -n $(NAMESPACE) --create-namespace $(call values,$$s) \
			--set image.repository=$(REGISTRY)/$$s --set image.tag=$(TAG) --wait --timeout 5m || exit 1; \
	done

deploy: deploy-infra deploy-apps ## deploy everything to ENV

undeploy: guard ## remove all releases from ENV
	-@for r in $(APPS) $(INFRA); do $(HELM) uninstall $$r -n $(NAMESPACE); done

smoke: local-only ## live checks against the local ingress
	./scripts/smoke.sh

replica-check: ## verify the read replica
	KUBE_CONTEXT=$(KCTX) NAMESPACE=$(NAMESPACE) ./scripts/replica-check.sh

loadtest: local-only ## load test: make loadtest N=2000
	./scripts/loadtest.sh $(N)

up: local-only images cluster-up import deploy ## build, create local cluster and deploy

down: local-only cluster-down ## tear the local cluster down

status: ## show pods and autoscalers
	$(KUBECTL) get pods,hpa -n $(NAMESPACE) -o wide

logs: ## tail logs: make logs s=users
	$(KUBECTL) logs -n $(NAMESPACE) -l app.kubernetes.io/name=$(s) -f --tail=100

restart: guard ## rolling restart: make restart s=gateway
	$(KUBECTL) rollout restart deployment/$(s) -n $(NAMESPACE)
	$(KUBECTL) rollout status deployment/$(s) -n $(NAMESPACE)

psql: ## open psql in the database
	$(KUBECTL) exec -it -n $(NAMESPACE) postgres-0 -- psql -U postgres -d app

backup-now: guard ## run a database backup immediately
	$(KUBECTL) create job -n $(NAMESPACE) --from=cronjob/postgres-backup backup-$$(date +%s)

port-forward: ## forward postgres to localhost:5432
	$(KUBECTL) port-forward -n $(NAMESPACE) postgres-0 5432:5432

clean: ## remove build output
	rm -rf bin
