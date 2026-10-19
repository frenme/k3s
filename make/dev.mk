.PHONY: fmt vet test lint build images clean

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

clean: ## remove build output
	rm -rf bin
