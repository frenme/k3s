.PHONY: deploy template destroy status logs restart loadtest grafana db-status db-promote psql backup-now kubeconfig

deploy: ## deploy everything to ENV
	$(HELMFILE) sync

template: ## render every manifest of ENV
	$(HELMFILE) template

destroy: ## remove every release from ENV
	$(HELMFILE) destroy

status: ## pods and autoscalers
	$(KUBECTL) get pods,hpa -o wide

logs: ## tail logs: make logs s=users
	$(KUBECTL) logs -l app.kubernetes.io/name=$(s) -f --tail=100

restart: ## rolling restart: make restart s=gateway
	$(KUBECTL) rollout restart deployment/$(s)
	$(KUBECTL) rollout status deployment/$(s)

loadtest: ## k6 load test from inside the cluster: make loadtest RATE=300 DURATION=2m
	$(KUBECTL) run k6 --rm -i --restart=Never --image=grafana/k6:1.8.2 -- run -e RATE=$(RATE) -e DURATION=$(DURATION) - < test/load/api.js

grafana: ## open grafana on http://localhost:3000
	kubectl --context $(CONTEXT) -n monitoring port-forward svc/monitoring-grafana 3000:80

db-status: ## database cluster, replication and backups
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) status postgres

db-promote: ## move the primary to an instance: make db-promote ENV=prod INSTANCE=postgres-1
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) promote postgres $(INSTANCE)

psql: ## psql on the primary
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) psql postgres -- -d app

backup-now: ## take a database backup immediately
	kubectl cnpg --context $(CONTEXT) -n $(NAMESPACE) backup postgres -m plugin --plugin-name barman-cloud.cloudnative-pg.io

kubeconfig: remote-only ## fetch kubeconfig of a server: make kubeconfig ENV=prod HOST=1.2.3.4
	set -o pipefail; ssh $(SSH_USER)@$(HOST) cat /etc/rancher/k3s/k3s.yaml | sed -e "s/127.0.0.1/$(HOST)/" -e "s/: default$$/: $(CONTEXT)/" > kubeconfig-$(ENV).yaml
	@echo "KUBECONFIG=$$PWD/kubeconfig-$(ENV).yaml"
