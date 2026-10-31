SHELL := /bin/bash

ENV ?= local
CLUSTER ?= k3s-base
TAG ?= dev
REGISTRY ?= k3s-base
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
export REGISTRY TAG

APP := app
GO_SERVICES := $(notdir $(wildcard $(APP)/cmd/*))
ENV_FILE := infra/deploy/environments/$(ENV).yaml
NAMESPACE := $(shell sed -n 's/^namespace: *//p' $(ENV_FILE))
CONTEXT := $(shell sed -n 's/^kubeContext: *//p' $(ENV_FILE))
KUBECTL := kubectl --context $(CONTEXT) -n $(NAMESPACE)
CNPG := kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE)
HELMFILE := helmfile -f infra/deploy/helmfile.yaml.gotmpl -e $(ENV)
TERRAFORM := terraform -chdir=infra/terraform
SSH_USER ?= root
MCP_IMAGE := grafana/mcp-grafana:2.1.1
MCP_TOOLS := search,datasource,prometheus,loki,tempo,alerting,dashboard,navigation
TOOLS := go golangci-lint docker kubectl helm helmfile sops age k3d terraform kubectl-cnpg

define terraform
	$(TERRAFORM) init -input=false >/dev/null
	$(TERRAFORM) workspace select -or-create $(ENV)
	$(TERRAFORM) $(1) -var-file=envs/$(ENV).tfvars
endef

.PHONY: help doctor local-only remote-only
.PHONY: fmt lint openapi test test-integration e2e images
.PHONY: up down deploy template status logs restart grafana
.PHONY: db-status db-promote psql backup-now
.PHONY: infra-plan infra-apply infra-destroy kubeconfig agent-kubeconfig mcp

help:
	@grep -hE '^[a-z0-9-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-18s %s\n", $$1, $$2}'

doctor: ## check that every required tool is installed
	@missing=0; for t in $(TOOLS); do command -v $$t >/dev/null && echo "ok      $$t" || { echo "missing $$t"; missing=1; }; done; exit $$missing

local-only:
	@[ "$(ENV)" = local ] || { echo "this target works only with ENV=local"; exit 1; }

remote-only:
	@[ "$(ENV)" != local ] || { echo "this target needs ENV=dev or ENV=prod"; exit 1; }

fmt: ## format go code
	cd $(APP) && golangci-lint fmt ./...

lint: ## formatting, golangci-lint and lint every release of ENV
	cd $(APP) && golangci-lint fmt --diff ./...
	cd $(APP) && golangci-lint run ./...
	$(if $(filter local,$(ENV)),,REGISTRY=lint.invalid TAG=lint) $(HELMFILE) lint

openapi: ## regenerate openapi/*.yaml from the code
	go -C $(APP) run ./openapi

test: ## unit tests
	go -C $(APP) test -race -count=1 ./...

test-integration: ## tests against a real postgres and redis (needs docker)
	go -C $(APP) test -tags integration -count=1 ./...

e2e: local-only ## end to end tests against the local ingress
	go -C $(APP) test -tags e2e -count=1 ./e2e

images: ## build container images tagged REGISTRY/<service>:TAG
	@for s in $(GO_SERVICES); do docker build -q --build-arg SERVICE=$$s --build-arg VERSION=$(TAG) --build-arg COMMIT=$(COMMIT) -t $(REGISTRY)/$$s:$(TAG) $(APP) || exit 1; done

up: local-only images ## build, create the local cluster if needed, load images and deploy
	k3d cluster get $(CLUSTER) >/dev/null 2>&1 || k3d cluster create --config infra/k3d/cluster.yaml --volume "$(CURDIR)/infra/k3s/coredns.yaml:/var/lib/rancher/k3s/server/manifests/coredns-core.yaml@server:0" --wait
	k3d image import -c $(CLUSTER) $(addprefix $(REGISTRY)/,$(addsuffix :$(TAG),$(GO_SERVICES)))
	$(HELMFILE) sync

down: local-only ## delete the local cluster
	k3d cluster delete $(CLUSTER)

deploy: ## deploy everything to ENV
	$(HELMFILE) sync

template: ## render every manifest of ENV
	$(HELMFILE) template

status: ## pods and autoscalers
	$(KUBECTL) get pods,hpa -o wide

logs: ## tail logs: make logs s=users
	$(KUBECTL) logs -l app.kubernetes.io/name=$(s) -f --tail=100

restart: ## rolling restart: make restart s=gateway
	@r=$$($(KUBECTL) get deployment,daemonset -l app.kubernetes.io/name=$(s) -o name); \
	$(KUBECTL) rollout restart $$r && $(KUBECTL) rollout status $$r

grafana: ## open grafana on http://localhost:3000
	kubectl --context $(CONTEXT) -n monitoring port-forward svc/monitoring-grafana 3000:80

db-status: ## database cluster, replication and backups
	$(CNPG) status postgres

db-promote: ## move the primary to an instance: make db-promote ENV=prod INSTANCE=postgres-1
	$(CNPG) promote postgres $(INSTANCE)

psql: ## psql on the primary
	$(CNPG) psql postgres -- -d app

backup-now: ## take a database backup immediately
	$(CNPG) backup postgres -m plugin --plugin-name barman-cloud.cloudnative-pg.io

infra-plan: remote-only ## plan servers of ENV
	$(call terraform,plan)

infra-apply: remote-only ## create or update servers of ENV
	$(call terraform,apply)

infra-destroy: remote-only ## destroy servers of ENV
	$(call terraform,destroy)

kubeconfig: remote-only ## fetch kubeconfig of a server: make kubeconfig ENV=prod HOST=1.2.3.4
	set -o pipefail; ssh $(SSH_USER)@$(HOST) cat /etc/rancher/k3s/k3s.yaml | sed -e "s/127.0.0.1/$(HOST)/" -e "s/: default$$/: $(CONTEXT)/" > kubeconfig-$(ENV).yaml
	@echo "KUBECONFIG=$$PWD/kubeconfig-$(ENV).yaml"

agent-kubeconfig: ## read-only kubeconfig for AI agents: make agent-kubeconfig ENV=prod
	@set -euo pipefail; umask 077; out=kubeconfig-agent-$(ENV).yaml; \
	server=$$(kubectl config view --raw --minify --context $(CONTEXT) -o jsonpath='{.clusters[0].cluster.server}'); \
	ca=$$(kubectl config view --raw --minify --context $(CONTEXT) -o jsonpath='{.clusters[0].cluster.certificate-authority-data}'); \
	token=$$(kubectl --context $(CONTEXT) -n kube-system get secret agent-token -o jsonpath='{.data.token}' | base64 -d); \
	printf 'apiVersion: v1\nkind: Config\nclusters:\n  - name: agent-$(ENV)\n    cluster:\n      server: %s\n      certificate-authority-data: %s\nusers:\n  - name: agent-$(ENV)\n    user:\n      token: %s\ncontexts:\n  - name: agent-$(ENV)\n    context:\n      cluster: agent-$(ENV)\n      user: agent-$(ENV)\ncurrent-context: agent-$(ENV)\n' "$$server" "$$ca" "$$token" > $$out; \
	echo "KUBECONFIG=$$PWD/$$out"

mcp: ## grafana MCP server over stdio for AI agents: make -s mcp ENV=prod
	@set -euo pipefail; out=$$(mktemp); \
	kubectl --context $(CONTEXT) -n monitoring port-forward svc/monitoring-grafana :80 >$$out 2>&1 & pf=$$!; \
	trap 'kill $$pf 2>/dev/null; rm -f $$out' EXIT; \
	GRAFANA_PASSWORD=$$(kubectl --context $(CONTEXT) -n monitoring get secret monitoring-grafana -o jsonpath='{.data.admin-password}' | base64 -d); export GRAFANA_PASSWORD; \
	for _ in $$(seq 100); do port=$$(sed -n 's/^Forwarding from 127.0.0.1:\([0-9]*\).*/\1/p' $$out); [ -n "$$port" ] && break; sleep 0.1; done; \
	[ -n "$$port" ] || { cat $$out >&2; exit 1; }; \
	docker run --rm -i -q -e GRAFANA_URL=http://host.docker.internal:$$port -e GRAFANA_USERNAME=admin -e GRAFANA_PASSWORD \
		$(MCP_IMAGE) -t stdio --enabled-tools $(MCP_TOOLS) --disable-write --usage-stats disabled
