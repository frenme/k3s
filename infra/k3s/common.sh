#!/usr/bin/env bash
set -euo pipefail

K3S_VERSION="${K3S_VERSION:-v1.35.5+k3s1}"

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    echo "run as root" >&2
    exit 1
  fi
}

