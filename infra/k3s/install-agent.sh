#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
require_root

: "${SERVER_URL:?SERVER_URL is required, e.g. https://1.2.3.4:6443}"
: "${K3S_TOKEN:?K3S_TOKEN is required}"
: "${REGION:?REGION is required, e.g. asia}"

IP="$(public_ip)"
NODE_NAME="${NODE_NAME:-edge-${REGION}}"

curl -sfL https://get.k3s.io | \
  INSTALL_K3S_VERSION="$K3S_VERSION" \
  K3S_URL="$SERVER_URL" \
  K3S_TOKEN="$K3S_TOKEN" \
  INSTALL_K3S_EXEC="agent \
    --node-name ${NODE_NAME} \
    --node-ip ${IP} \
    --node-external-ip ${IP} \
    --node-label role=edge \
    --node-label topology.kubernetes.io/region=${REGION}" \
  sh -
