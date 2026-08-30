#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
require_root

if [ -x /usr/local/bin/k3s-uninstall.sh ]; then
  /usr/local/bin/k3s-uninstall.sh
elif [ -x /usr/local/bin/k3s-agent-uninstall.sh ]; then
  /usr/local/bin/k3s-agent-uninstall.sh
else
  echo "k3s is not installed" >&2
  exit 1
fi
