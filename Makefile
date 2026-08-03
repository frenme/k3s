SHELL := /bin/bash

CLUSTER ?= k3s-base
NAMESPACE ?= app
TAG ?= dev
REGISTRY ?= k3s-base
GO_SERVICES := gateway users orders
SERVICES := $(GO_SERVICES) web
SECRET_VALUES = $(wildcard deploy/values/$(1).secret.yaml)

.PHONY: help fmt vet test build images cluster-up cluster-down import deploy-infra deploy-apps deploy undeploy smoke up down status logs restart psql backup-now port-forward loadtest

help:
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-14s %s\n", $$1, $$2}'

fmt: ## format go code
	gofmt -w cmd internal

vet: ## static checks
	go vet ./...

test: ## unit tests
	go test -race -count=1 ./...

build: ## build go binaries into bin/
	@for s in $(GO_SERVICES); do CGO_ENABLED=0 go build -o bin/$$s ./cmd/$$s || exit 1; done

images: ## build container images
	@for s in $(GO_SERVICES); do docker build -q -f build/go.Dockerfile --build-arg SERVICE=$$s -t $(REGISTRY)/$$s:$(TAG) . || exit 1; done
	docker build -q -f build/web.Dockerfile -t $(REGISTRY)/web:$(TAG) .

cluster-up: ## create local k3s cluster (k3d)
	k3d cluster create --config infra/k3d/cluster.yaml --wait
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -

cluster-down: ## delete local cluster
	k3d cluster delete $(CLUSTER)

import: ## load images into local cluster
	k3d image import -c $(CLUSTER) $(addprefix $(REGISTRY)/,$(addsuffix :$(TAG),$(SERVICES)))

deploy-infra: ## deploy postgres and redis
	helm upgrade --install postgres deploy/helm/postgres -n $(NAMESPACE) -f deploy/values/postgres.yaml $(addprefix -f ,$(call SECRET_VALUES,postgres)) --wait --timeout 3m
	helm upgrade --install redis deploy/helm/redis -n $(NAMESPACE) -f deploy/values/redis.yaml --wait --timeout 2m

deploy-apps: ## deploy services
	@for s in users orders gateway web; do \
		helm upgrade --install $$s deploy/helm/service -n $(NAMESPACE) \
			-f deploy/values/$$s.yaml $(addprefix -f ,$(call SECRET_VALUES,$$s)) \
			--set image.repository=$(REGISTRY)/$$s --set image.tag=$(TAG) --wait --timeout 3m || exit 1; \
	done

deploy: deploy-infra deploy-apps ## deploy everything

undeploy: ## remove all releases
	-@for r in web gateway orders users redis postgres; do helm uninstall $$r -n $(NAMESPACE); done

smoke: ## live checks against the ingress
	./scripts/smoke.sh

up: images cluster-up import deploy ## build, create cluster and deploy

down: cluster-down ## tear everything down

status: ## show pods
	kubectl get pods -n $(NAMESPACE) -o wide

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

loadtest: ## simple load test: make loadtest N=500
	./scripts/loadtest.sh $(N)

