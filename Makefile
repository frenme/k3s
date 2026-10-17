SHELL := /bin/bash

CLUSTER ?= k3s-base
NAMESPACE ?= app
TAG ?= dev
REGISTRY ?= k3s-base
ENV ?=
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)

GO_SERVICES := $(notdir $(wildcard cmd/*))
IMAGES := $(GO_SERVICES)
INFRA := postgres redis
APPS := $(filter-out $(INFRA),$(basename $(notdir $(wildcard deploy/values/*.yaml))))

file_flag = $$([ -f $(1) ] && echo -f $(1))
overlay = $(if $(ENV),$(call file_flag,deploy/values/$(ENV)/$(1).yaml)) $(call file_flag,deploy/values/$(1).secret.yaml)
values = -f deploy/values/$(1).yaml $(call overlay,$(1))

.PHONY: help fmt vet test lint helm-lint tidy build images import cluster-up cluster-down deploy deploy-infra deploy-apps undeploy smoke loadtest up down status logs restart psql backup-now port-forward clean

help:
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-14s %s\n", $$1, $$2}'

fmt: ## format go code
	gofmt -w cmd internal

vet: ## static checks
	go vet ./...

test: ## unit tests
	go test -race -count=1 ./...

helm-lint: ## lint and render all charts
	@for c in service postgres redis; do helm lint deploy/helm/$$c || exit 1; done
	@for s in $(APPS); do helm template $$s deploy/helm/service $(call values,$$s) >/dev/null || exit 1; done

lint: vet helm-lint ## vet and helm lint

tidy: ## tidy go modules
	go mod tidy

build: ## build go binaries into bin/
	@for s in $(GO_SERVICES); do CGO_ENABLED=0 go build -o bin/$$s ./cmd/$$s || exit 1; done

images: ## build container images
	@for s in $(GO_SERVICES); do docker build -q -f build/go.Dockerfile --build-arg SERVICE=$$s --build-arg VERSION=$(TAG) --build-arg COMMIT=$(COMMIT) -t $(REGISTRY)/$$s:$(TAG) . || exit 1; done

cluster-up: ## create local k3s cluster (k3d)
	k3d cluster create --config infra/k3d/cluster.yaml --wait
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -

cluster-down: ## delete local cluster
	k3d cluster delete $(CLUSTER)

import: ## load images into local cluster
	k3d image import -c $(CLUSTER) $(addprefix $(REGISTRY)/,$(addsuffix :$(TAG),$(IMAGES)))

deploy-infra: ## deploy postgres and redis
	helm upgrade --install postgres deploy/helm/postgres -n $(NAMESPACE) $(call values,postgres) --wait --timeout 3m
	helm upgrade --install redis deploy/helm/redis -n $(NAMESPACE) $(call values,redis) --wait --timeout 2m

deploy-apps: ## deploy services (one release per deploy/values/<name>.yaml)
	@for s in $(APPS); do \
		helm upgrade --install $$s deploy/helm/service -n $(NAMESPACE) $(call values,$$s) \
			--set image.repository=$(REGISTRY)/$$s --set image.tag=$(TAG) --wait --timeout 3m || exit 1; \
	done

deploy: deploy-infra deploy-apps ## deploy everything

undeploy: ## remove all releases
	-@for r in $(APPS) $(INFRA); do helm uninstall $$r -n $(NAMESPACE); done

smoke: ## live checks against the ingress
	./scripts/smoke.sh

loadtest: ## load test: make loadtest N=2000
	./scripts/loadtest.sh $(N)

up: images cluster-up import deploy ## build, create cluster and deploy

down: cluster-down ## tear everything down

status: ## show pods and autoscalers
	kubectl get pods,hpa -n $(NAMESPACE) -o wide

logs: ## tail logs: make logs s=users
	kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=$(s) -f --tail=100

restart: ## rolling restart: make restart s=gateway
	kubectl rollout restart deployment/$(s) -n $(NAMESPACE)
	kubectl rollout status deployment/$(s) -n $(NAMESPACE)

psql: ## open psql in the database
	kubectl exec -it -n $(NAMESPACE) postgres-0 -- psql -U postgres -d app

backup-now: ## run a database backup immediately
	kubectl create job -n $(NAMESPACE) --from=cronjob/postgres-backup backup-$$(date +%s)

port-forward: ## forward postgres to localhost:5432
	kubectl port-forward -n $(NAMESPACE) postgres-0 5432:5432

clean: ## remove build output
	rm -rf bin
