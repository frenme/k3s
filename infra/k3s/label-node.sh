#!/usr/bin/env bash
set -euo pipefail

NODE="${1:?usage: label-node.sh <node> <role> [region]}"
ROLE="${2:?role is required: core or edge}"
REGION="${3:-}"

kubectl label node "$NODE" role="$ROLE" --overwrite
if [ -n "$REGION" ]; then
  kubectl label node "$NODE" topology.kubernetes.io/region="$REGION" --overwrite
fi
