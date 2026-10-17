#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/common.sh"
require_root

ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp

ufw allow 80/tcp
ufw allow 443/tcp

for peer in "$@"; do
  ufw allow from "$peer" to any port 6443 proto tcp
  ufw allow from "$peer" to any port 10250 proto tcp
  ufw allow from "$peer" to any port 51820 proto udp
  ufw allow from "$peer" to any port 51821 proto udp
done

ufw --force enable
ufw status verbose
