.PHONY: fmt vet test test-integration lint api-lint build images clean

fmt: ## format go code
	golangci-lint fmt ./...

vet: ## formatting and static checks
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; exit 1; }
	go vet ./...

test: ## unit tests
	go test -race -count=1 ./...

test-integration: ## store and migration tests against a real postgres (needs docker)
	go test -tags integration -count=1 ./...

lint: vet ## vet, golangci-lint and lint every release of ENV
	golangci-lint run ./...
	$(if $(filter local,$(ENV)),,REGISTRY=lint.invalid TAG=lint) $(HELMFILE) lint

api-lint: ## lint the OpenAPI spec
	cd api && npx -y @redocly/cli@2.65.0 lint openapi.yaml

build: ## build go binaries into bin/
	@for s in $(GO_SERVICES); do CGO_ENABLED=0 go build -o bin/$$s ./cmd/$$s || exit 1; done

images: ## build container images tagged REGISTRY/<service>:TAG
	@for s in $(GO_SERVICES); do docker build -q --build-arg SERVICE=$$s --build-arg VERSION=$(TAG) --build-arg COMMIT=$(COMMIT) -t $(REGISTRY)/$$s:$(TAG) . || exit 1; done

clean: ## remove build output
	rm -rf bin
