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
TOOLS := go golangci-lint docker kubectl helm helmfile sops age k3d terraform kubectl-cnpg

.PHONY: help local-only remote-only doctor

help:
	@grep -hE '^[a-z0-9-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-18s %s\n", $$1, $$2}'

local-only:
	@[ "$(ENV)" = local ] || { echo "this target works only with ENV=local"; exit 1; }

remote-only:
	@[ "$(ENV)" != local ] || { echo "this target needs ENV=dev or ENV=prod"; exit 1; }

doctor: ## check that every required tool is installed
	@missing=0; for t in $(TOOLS); do command -v $$t >/dev/null && echo "ok      $$t" || { echo "missing $$t"; missing=1; }; done; exit $$missing

include make/dev.mk make/local.mk make/ops.mk make/infra.mk
