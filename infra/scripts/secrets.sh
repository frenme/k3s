#!/usr/bin/env bash
set -euo pipefail

env_name=${1:-}
[ -n "$env_name" ] || { echo "usage: secrets.sh <env>" >&2; exit 1; }
root=$(cd "$(dirname "$0")/../.." && pwd)
file=${SECRETS_FILE:-$root/infra/deploy/environments/$env_name.secrets.yaml}

if [ -e "$file" ] && [ "${FORCE:-}" != 1 ]; then
  echo "$file exists, FORCE=1 replaces it with new passwords" >&2
  exit 1
fi

token=${HCLOUD_TOKEN:-}
if [ -z "$token" ]; then
  read -rsp "Hetzner Cloud API token: " token
  echo >&2
fi
[ "${#token}" -eq 64 ] || { echo "the Hetzner token must have 64 characters" >&2; exit 1; }

umask 077
plain=$(mktemp)
trap 'rm -f "$plain"' EXIT
cat > "$plain" <<YAML
secrets:
  hcloud:
    token: $token
  dbSecret: $(openssl rand -hex 24)
  redis: $(openssl rand -hex 24)
  grafana: $(openssl rand -hex 16)
  jwt: $(openssl rand -hex 32)
YAML
sops --encrypt --filename-override "$root/infra/deploy/environments/$env_name.secrets.yaml" "$plain" > "$file.new"
mv "$file.new" "$file"
echo "created $file"
echo "add s3 (with backups), telegram (alerts) or registry (private images) later with: sops $file"
