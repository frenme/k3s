#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-app}"
PSQL="kubectl ${KUBE_CONTEXT:+--context $KUBE_CONTEXT} exec -n $NAMESPACE postgres-replica-0 -- psql -U postgres -d app -tA"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

[ "$($PSQL -c 'select pg_is_in_recovery()')" = "t" ] || fail "replica is not in recovery"
echo "ok   replica is in recovery"

if $PSQL -c "create table replica_write_probe(id int)" >/dev/null 2>&1; then
  fail "replica accepted a write"
fi
echo "ok   replica rejects writes"

streaming=$(kubectl ${KUBE_CONTEXT:+--context $KUBE_CONTEXT} exec -n "$NAMESPACE" postgres-0 -- psql -U postgres -d app -tA -c "select count(*) from pg_stat_replication where state = 'streaming'")
[ "$streaming" -ge 1 ] || fail "no streaming replicas on primary"
echo "ok   primary streams to $streaming replica(s)"

primary=$(kubectl ${KUBE_CONTEXT:+--context $KUBE_CONTEXT} exec -n "$NAMESPACE" postgres-0 -- psql -U postgres -d app -tA -c "select count(*) from users.users")
sleep 1
replica=$($PSQL -c "select count(*) from users.users")
[ "$primary" = "$replica" ] || fail "row count differs: primary $primary, replica $replica"
echo "ok   users row count matches ($primary)"
