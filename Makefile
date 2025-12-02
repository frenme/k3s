SHELL := /bin/bash

CLUSTER ?= k3s-base
NAMESPACE ?= app
TAG ?= dev
REGISTRY ?= k3s-base
GO_SERVICES := gateway users orders
SERVICES := $(GO_SERVICES) web
SECRET_VALUES = $(wildcard deploy/values/$(1).secret.yaml)

.PHONY: help fmt vet test build images cluster-up

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

