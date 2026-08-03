#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
require_root

ROLE="${1:?usage: firewall.sh <server|agent> [peer-ip ...]}"
shift || true

