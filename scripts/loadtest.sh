#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
N="${1:-1000}"
CONCURRENCY="${CONCURRENCY:-50}"
URL="${URL:-$BASE/api/users}"

start=$(date +%s)
seq 1 "$N" | xargs -P "$CONCURRENCY" -I{} curl -s -o /dev/null -w '%{http_code}\n' "$URL" | sort | uniq -c
echo "requests: $N, concurrency: $CONCURRENCY, seconds: $(($(date +%s) - start))"
