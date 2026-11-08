#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
templates=$root/infra/terraform/templates
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cat > "$work/main.tf" <<HCL
locals {
  server = { token = "x", agent_token = "x", k3s_version = "v", s3 = null, registry = null }
  agent  = { token = "x", k3s_version = "v", server_ip = "1.1.1.1", registry = null }
}
output "server" { value = templatefile("$templates/server.yaml.tftpl", merge(local.server, { edge = false })) }
output "single" { value = templatefile("$templates/server.yaml.tftpl", merge(local.server, { edge = true })) }
output "edge"   { value = templatefile("$templates/agent.yaml.tftpl", merge(local.agent, { role = "edge" })) }
output "core"   { value = templatefile("$templates/agent.yaml.tftpl", merge(local.agent, { role = "core" })) }
HCL
terraform -chdir="$work" init -input=false >/dev/null
terraform -chdir="$work" apply -auto-approve -input=false >/dev/null

normalize() {
  sed -E 's/^ +//; s/^(token|agent-token): .*/\1: x/; s/INSTALL_K3S_VERSION=[^ ]+/INSTALL_K3S_VERSION=v/' | sort
}

status=0
for role in server single edge core; do
  terraform -chdir="$work" output -raw "$role" | awk '
    /path: \/etc\/rancher\/k3s\/config.yaml/ {inside = 1; next}
    /^  - path|^runcmd/ {inside = 0}
    inside && !/permissions:|content:/ {print}
    /curl -sfL/ {print}' | sed 's/^ *- curl/curl/' | normalize > "$work/$role.cloud-init"
  DRY_RUN=1 "$root/infra/scripts/install-host.sh" "$role" 1.1.1.1 1.1.1.1 | sed -E '/^# on /d; s/^# command: .*(curl)/\1/' | normalize > "$work/$role.install-host"
  if ! diff "$work/$role.cloud-init" "$work/$role.install-host" > "$work/$role.diff"; then
    echo "k3s config of role $role differs between terraform/templates and scripts/install-host.sh:" >&2
    cat "$work/$role.diff" >&2
    status=1
  fi
done
[ "$status" -eq 0 ] && echo "k3s config: terraform and install-host.sh agree"
exit "$status"
