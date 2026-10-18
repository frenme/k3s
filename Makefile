SHELL := /bin/bash

ENV ?= local
CLUSTER ?= k3s-base
TAG ?= dev
REGISTRY ?= k3s-base
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
export REGISTRY TAG

GO_SERVICES := $(notdir $(wildcard cmd/*))
NAMESPACE := $(if $(filter local,$(ENV)),app,app-$(ENV))
CONTEXT := $(if $(filter local,$(ENV)),k3d-$(CLUSTER),k3s-$(ENV))
KUBECTL := kubectl --context $(CONTEXT) -n $(NAMESPACE)
HELMFILE := helmfile -f deploy/helmfile.yaml.gotmpl -e $(ENV)

.PHONY: help local-only fmt vet test lint build images cluster-up cluster-down import deploy template destroy up down smoke loadtest status db-status logs restart psql backup-now kubeconfig clean

help:
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-12s %s\n", $$1, $$2}'

local-only:
	@[ "$(ENV)" = local ] || { echo "this target works only with ENV=local"; exit 1; }

fmt: ## format go code
	gofmt -w cmd internal

vet: ## formatting and static checks
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; exit 1; }
	go vet ./...

test: ## unit tests
	go test -race -count=1 ./...

lint: vet ## vet and lint every release of ENV
	$(if $(filter local,$(ENV)),,REGISTRY=lint.invalid TAG=lint) $(HELMFILE) lint

build: ## build go binaries into bin/
	@for s in $(GO_SERVICES); do CGO_ENABLED=0 go build -o bin/$$s ./cmd/$$s || exit 1; done

images: ## build container images tagged REGISTRY/<service>:TAG
	@for s in $(GO_SERVICES); do docker build -q -f build/go.Dockerfile --build-arg SERVICE=$$s --build-arg VERSION=$(TAG) --build-arg COMMIT=$(COMMIT) -t $(REGISTRY)/$$s:$(TAG) . || exit 1; done

cluster-up: local-only ## create the local k3s cluster
	k3d cluster create --config infra/k3d/cluster.yaml --wait

cluster-down: local-only ## delete the local k3s cluster
	k3d cluster delete $(CLUSTER)

import: local-only ## load images into the local cluster
	k3d image import -c $(CLUSTER) $(addprefix $(REGISTRY)/,$(addsuffix :$(TAG),$(GO_SERVICES)))

deploy: ## deploy everything to ENV
	$(HELMFILE) sync

template: ## render every manifest of ENV
	$(HELMFILE) template

destroy: ## remove every release from ENV
	$(HELMFILE) destroy

up: local-only images cluster-up import deploy ## build, create the local cluster and deploy

down: local-only cluster-down ## tear the local cluster down

smoke: local-only ## live checks against the local ingress
	./scripts/smoke.sh

loadtest: local-only ## load test: make loadtest N=2000
	./scripts/loadtest.sh $(N)

status: ## pods and autoscalers
	$(KUBECTL) get pods,hpa -o wide

db-status: ## database cluster, replication and backups
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) status postgres

logs: ## tail logs: make logs s=users
	$(KUBECTL) logs -l app.kubernetes.io/name=$(s) -f --tail=100

restart: ## rolling restart: make restart s=gateway
	$(KUBECTL) rollout restart deployment/$(s)
	$(KUBECTL) rollout status deployment/$(s)

psql: ## psql on the primary
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) psql postgres -- -d app

backup-now: ## take a database backup immediately
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) backup postgres -m plugin --plugin-name barman-cloud.cloudnative-pg.io

kubeconfig: ## fetch kubeconfig of a server: make kubeconfig HOST=1.2.3.4 ENV=prod
	ssh $(HOST) sudo cat /etc/rancher/k3s/k3s.yaml | sed -e "s/127.0.0.1/$(HOST)/" -e "s/: default$$/: $(CONTEXT)/" > kubeconfig-$(ENV).yaml
	@echo "KUBECONFIG=$$PWD/kubeconfig-$(ENV).yaml"

clean: ## remove build output
	rm -rf bin
