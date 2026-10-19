.PHONY: infra-init infra-plan infra-apply infra-destroy

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
