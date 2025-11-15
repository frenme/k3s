SHELL := /bin/bash

CLUSTER ?= k3s-base
NAMESPACE ?= app
TAG ?= dev
REGISTRY ?= k3s-base
GO_SERVICES := gateway users orders
SERVICES := $(GO_SERVICES) web
SECRET_VALUES = $(wildcard deploy/values/$(1).secret.yaml)

.PHONY: help fmt vet test

help:
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-14s %s\n", $$1, $$2}'

fmt: ## format go code
	gofmt -w cmd internal

vet: ## static checks
	go vet ./...

test: ## unit tests
	go test -race -count=1 ./...

