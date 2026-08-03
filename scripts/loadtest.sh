#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
N="${1:-200}"
CONCURRENCY="${CONCURRENCY:-20}"

start=$(date +%s)
seq 1 "$N" | xargs -P "$CONCURRENCY" -I{} curl -s -o /dev/null -w '%{http_code}\n' "$BASE/api/users" | sort | uniq -c
end=$(date +%s)
echo "requests: $N, concurrency: $CONCURRENCY, seconds: $((end - start))"
