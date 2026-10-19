.PHONY: cluster-up cluster-down import up down e2e

cluster-up: local-only ## create the local k3s cluster
	k3d cluster create --config infra/k3d/cluster.yaml --wait

cluster-down: local-only ## delete the local k3s cluster
	k3d cluster delete $(CLUSTER)

import: local-only ## load images into the local cluster
	k3d image import -c $(CLUSTER) $(addprefix $(REGISTRY)/,$(addsuffix :$(TAG),$(GO_SERVICES)))

up: local-only images cluster-up import deploy ## build, create the local cluster and deploy

down: local-only cluster-down ## tear the local cluster down

e2e: local-only ## end to end tests against the local ingress
	go test -tags e2e -count=1 ./e2e
