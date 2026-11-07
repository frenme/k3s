#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
sops_file=$root/.sops.yaml
common=$root/infra/deploy/environments/common.yaml

for tool in age-keygen sops; do
  command -v "$tool" >/dev/null || { echo "$tool is not installed" >&2; exit 1; }
done

ask() {
  local var=$1 prompt=$2 value=${!1:-}
  if [ -z "$value" ]; then
    [ -t 0 ] || { echo "$var is not set and there is no terminal to ask" >&2; exit 1; }
    read -rp "$prompt: " value
  fi
  [ -n "$value" ] || { echo "$var must not be empty" >&2; exit 1; }
  printf '%s' "$value"
}

if [ "$(uname)" = Darwin ]; then
  default_key="$HOME/Library/Application Support/sops/age/keys.txt"
else
  default_key="${XDG_CONFIG_HOME:-$HOME/.config}/sops/age/keys.txt"
fi
key_file=${SOPS_AGE_KEY_FILE:-$default_key}

if [ ! -f "$key_file" ]; then
  mkdir -p "$(dirname "$key_file")"
  (umask 077 && age-keygen -o "$key_file" 2>/dev/null)
  echo "created the age key $key_file (back it up: without it the secrets cannot be read)"
fi
public=$(age-keygen -y "$key_file")

current=$(sed -n 's/^ *age: *//p' "$sops_file" | head -1)
if [ "$current" = "$public" ]; then
  echo "sops: .sops.yaml already uses your age key"
else
  existing=$(find "$root/infra/deploy/environments" -name '*.secrets.yaml' -exec basename {} \; | tr '\n' ' ')
  echo "sops: .sops.yaml uses another age key ($current)"
  [ -z "$existing" ] || echo "      ${existing% } is encrypted with it and will be unreadable for your key; recreate it with: make secrets ENV=<env> FORCE=1"
  if [ "${YES:-}" != 1 ]; then
    [ -t 0 ] || { echo "run with YES=1 to replace it" >&2; exit 1; }
    read -rp "replace it with $public? [y/N] " answer
    [ "$answer" = y ] || { echo "left .sops.yaml as it is"; exit 0; }
  fi
  sed -i.bak "s|^\( *age: *\).*|\1$public|" "$sops_file" && rm -f "$sops_file.bak"
  echo "sops: .sops.yaml now uses $public"
fi

domain=$(sed -n 's/^domain: *//p' "$common")
if [ "$domain" = example.com ] || [ -n "${DOMAIN:-}" ]; then
  DOMAIN=$(ask DOMAIN "your domain (the hosts are dev.<domain> and prod.<domain>)")
  sed -i.bak "s|^domain: .*|domain: $DOMAIN|" "$common" && rm -f "$common.bak"
  echo "domain: $DOMAIN"
else
  echo "domain: $domain"
fi

placeholder="ssh-ed25519 REPLACE_WITH_YOUR_PUBLIC_KEY"
files=$(grep -l "$placeholder" "$root"/infra/terraform/envs/*.tfvars 2>/dev/null || true)
if [ -n "$files" ]; then
  SSH_KEY_FILE=${SSH_KEY_FILE:-}
  if [ -z "$SSH_KEY_FILE" ]; then
    for candidate in "$HOME/.ssh/id_ed25519.pub" "$HOME/.ssh/id_rsa.pub"; do
      [ -f "$candidate" ] && { SSH_KEY_FILE=$candidate; break; }
    done
  fi
  SSH_KEY_FILE=$(ask SSH_KEY_FILE "path of your SSH public key")
  ssh_key=$(awk 'NR==1{print $1" "$2}' "$SSH_KEY_FILE")
  [ -n "$ssh_key" ] || { echo "$SSH_KEY_FILE is empty" >&2; exit 1; }
  for file in $files; do
    content=$(cat "$file")
    printf '%s\n' "${content//$placeholder/$ssh_key}" > "$file"
    echo "ssh key: $(basename "$file")"
  done
else
  echo "ssh key: already set in every tfvars"
fi

echo "next: make secrets ENV=<env>, make push REGISTRY=... TAG=..., make bootstrap ENV=<env> REGISTRY=... TAG=..."
