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
TERRAFORM := terraform -chdir=infra/terraform
RATE ?= 200
DURATION ?= 1m
SSH_USER ?= root
TOOLS := go docker kubectl helm helmfile sops age k3d terraform kubectl-cnpg

.PHONY: help doctor remote-only infra-init infra-plan infra-apply infra-destroy e2e grafana local-only fmt vet test lint build images cluster-up cluster-down import deploy template destroy up down loadtest status db-status db-promote logs restart psql backup-now kubeconfig clean

help:
	@grep -E '^[a-z0-9-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-14s %s\n", $$1, $$2}'

local-only:
	@[ "$(ENV)" = local ] || { echo "this target works only with ENV=local"; exit 1; }

remote-only:
	@[ "$(ENV)" != local ] || { echo "this target needs ENV=dev or ENV=prod"; exit 1; }

doctor: ## check that every required tool is installed
	@missing=0; for t in $(TOOLS); do command -v $$t >/dev/null && echo "ok      $$t" || { echo "missing $$t"; missing=1; }; done; exit $$missing

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

e2e: local-only ## end to end tests against the local ingress
	go test -tags e2e -count=1 ./e2e

loadtest: ## k6 load test from inside the cluster: make loadtest RATE=300 DURATION=2m
	$(KUBECTL) run k6 --rm -i --restart=Never --image=grafana/k6:1.8.2 -- run -e RATE=$(RATE) -e DURATION=$(DURATION) - < test/load/api.js

grafana: ## open grafana on http://localhost:3000
	kubectl --context $(CONTEXT) -n monitoring port-forward svc/monitoring-grafana 3000:80

infra-init: ## init terraform providers
	$(TERRAFORM) init

infra-plan: remote-only ## plan servers of ENV
	$(TERRAFORM) workspace select -or-create $(ENV)
	$(TERRAFORM) plan -var-file=envs/$(ENV).tfvars

infra-apply: remote-only ## create or update servers of ENV
	$(TERRAFORM) workspace select -or-create $(ENV)
	$(TERRAFORM) apply -var-file=envs/$(ENV).tfvars

infra-destroy: remote-only ## destroy servers of ENV
	$(TERRAFORM) workspace select -or-create $(ENV)
	$(TERRAFORM) destroy -var-file=envs/$(ENV).tfvars

status: ## pods and autoscalers
	$(KUBECTL) get pods,hpa -o wide

db-status: ## database cluster, replication and backups
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) status postgres

logs: ## tail logs: make logs s=users
	$(KUBECTL) logs -l app.kubernetes.io/name=$(s) -f --tail=100

restart: ## rolling restart: make restart s=gateway
	$(KUBECTL) rollout restart deployment/$(s)
	$(KUBECTL) rollout status deployment/$(s)

db-promote: ## move the primary to an instance: make db-promote ENV=prod INSTANCE=postgres-1
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) promote postgres $(INSTANCE)

psql: ## psql on the primary
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) psql postgres -- -d app

backup-now: ## take a database backup immediately
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) backup postgres -m plugin --plugin-name barman-cloud.cloudnative-pg.io

kubeconfig: remote-only ## fetch kubeconfig of a server: make kubeconfig ENV=prod HOST=1.2.3.4
	set -o pipefail; ssh $(SSH_USER)@$(HOST) cat /etc/rancher/k3s/k3s.yaml | sed -e "s/127.0.0.1/$(HOST)/" -e "s/: default$$/: $(CONTEXT)/" > kubeconfig-$(ENV).yaml
	@echo "KUBECONFIG=$$PWD/kubeconfig-$(ENV).yaml"

clean: ## remove build output
	rm -rf bin
