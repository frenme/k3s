#!/usr/bin/env bash
set -euo pipefail

SERVER="${1:?usage: kubeconfig.sh <server-ip> [ssh-user]}"
SSH_USER="${2:-root}"
OUT="${OUT:-kubeconfig.yaml}"

ssh "${SSH_USER}@${SERVER}" "cat /etc/rancher/k3s/k3s.yaml" | sed "s/127.0.0.1/${SERVER}/" > "$OUT"
chmod 600 "$OUT"
echo "export KUBECONFIG=$(pwd)/$OUT"
