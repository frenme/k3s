#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
SUFFIX="$(date +%s)$RANDOM"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

expect_status() {
  local want="$1" got="$2" name="$3"
  [ "$want" = "$got" ] || fail "$name: expected $want, got $got"
  echo "ok   $name"
}

code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/")
expect_status 200 "$code" "web index"

code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/unknown")
expect_status 404 "$code" "unknown route"

resp=$(curl -s -w '\n%{http_code}' -X POST "$BASE/api/users" -H 'Content-Type: application/json' \
  -d "{\"email\":\"smoke-$SUFFIX@example.com\",\"name\":\"Smoke\"}")
expect_status 201 "$(tail -n1 <<<"$resp")" "create user"
user_id=$(sed -n 's/.*"id":"\([^"]*\)".*/\1/p' <<<"$(head -n1 <<<"$resp")")
[ -n "$user_id" ] || fail "user id missing"

code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/users" -H 'Content-Type: application/json' \
  -d "{\"email\":\"smoke-$SUFFIX@example.com\",\"name\":\"Smoke\"}")
expect_status 409 "$code" "duplicate user"

code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/users/$user_id")
expect_status 200 "$code" "get user"

