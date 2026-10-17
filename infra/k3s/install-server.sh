#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
require_root

IP="$(public_ip)"
NODE_NAME="${NODE_NAME:-core-1}"
REGION="${REGION:-core}"

MANIFESTS=/var/lib/rancher/k3s/server/manifests
mkdir -p "$MANIFESTS"
cp "$(dirname "$0")/traefik-config.yaml" "$MANIFESTS/traefik-config.yaml"

curl -sfL https://get.k3s.io | \
  INSTALL_K3S_VERSION="$K3S_VERSION" \
  INSTALL_K3S_EXEC="server \
    --node-name ${NODE_NAME} \
    --node-ip ${IP} \
    --node-external-ip ${IP} \
    --advertise-address ${IP} \
    --tls-san ${IP} \
    --flannel-backend=wireguard-native \
    --flannel-external-ip \
    --write-kubeconfig-mode 600 \
    --node-label role=core \
    --node-label topology.kubernetes.io/region=${REGION}" \
  sh -

echo
echo "join token:"
cat /var/lib/rancher/k3s/server/node-token
echo
echo "server url: https://${IP}:6443"
