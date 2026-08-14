#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
require_root

: "${SERVER_URL:?SERVER_URL is required, e.g. https://1.2.3.4:6443}"
: "${K3S_TOKEN:?K3S_TOKEN is required}"
: "${REGION:?REGION is required, e.g. asia}"

