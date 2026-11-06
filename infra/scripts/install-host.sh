#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: install-host.sh <server|single|edge|core> <host> [server host for edge and core]" >&2
  exit 1
}

role=${1:-}
host=${2:-}
server=${3:-}
[ -n "$role" ] && [ -n "$host" ] || usage
case $role in
  server|single) ;;
  edge|core) [ -n "$server" ] || usage ;;
  *) usage ;;
esac

root=$(cd "$(dirname "$0")/../.." && pwd)
version=$(sed -n 's/^k3sVersion: *//p' "$root/infra/deploy/environments/common.yaml")
user=${SSH_USER:-root}
ssh_opts=(-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)

case $role in
  server|single)
    node_labels="  - role=core"
    [ "$role" = single ] && node_labels=$(printf '  - role=edge\n  - svccontroller.k3s.cattle.io/enablelb=true')
    config=$(cat <<YAML
token: $(openssl rand -hex 24)
agent-token: $(openssl rand -hex 24)
cluster-init: true
flannel-backend: wireguard-native
write-kubeconfig-mode: "0600"
secrets-encryption: true
node-label:
$node_labels
etcd-snapshot-schedule-cron: "0 */6 * * *"
etcd-snapshot-retention: 28
YAML
)
    mode=server
    ;;
  edge|core)
    agent_token="<hidden>"
    [ "${DRY_RUN:-}" = 1 ] || agent_token=$(ssh "${ssh_opts[@]}" "$user@$server" cat /var/lib/rancher/k3s/server/agent-token)
    if [ "$role" = edge ]; then
      node_extra=$(printf 'node-label:\n  - role=edge\n  - svccontroller.k3s.cattle.io/enablelb=true\nnode-taint:\n  - role=edge:NoSchedule')
    else
      node_extra=$(printf 'node-label:\n  - role=core')
    fi
    config=$(printf 'server: https://%s:6443\ntoken: %s\n%s\n' "$server" "$agent_token" "$node_extra")
    mode=agent
    ;;
esac

remote="mkdir -p /etc/rancher/k3s && umask 077 && cat > /etc/rancher/k3s/config.yaml"
remote="$remote && curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=$version sh -s - $mode"

if [ "${DRY_RUN:-}" = 1 ]; then
  echo "# on $user@$host, role $role, k3s $version"
  echo "$config" | sed -E 's/^(token|agent-token): .*/\1: <hidden>/'
  echo "# command: $remote"
  exit 0
fi

echo "$config" | ssh "${ssh_opts[@]}" "$user@$host" "$remote"
echo "k3s installed on $host as $role"
if [ "$mode" = server ]; then
  echo "next: make kubeconfig ENV=<env> HOST=$host"
fi
