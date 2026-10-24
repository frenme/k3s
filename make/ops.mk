.PHONY: deploy template destroy status logs restart grafana db-status db-promote psql backup-now kubeconfig agent-kubeconfig mcp

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
