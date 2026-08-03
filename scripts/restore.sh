#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-app}"
DUMP="${1:?usage: restore.sh <dump-file-name>}"

kubectl run pg-restore -n "$NAMESPACE" --rm -i --restart=Never --image=postgres:17-alpine \
  --overrides="{\"spec\":{\"containers\":[{\"name\":\"pg-restore\",\"image\":\"postgres:17-alpine\",\"command\":[\"sh\",\"-c\",\"pg_restore --clean --if-exists -h postgres -U postgres -d app /backups/${DUMP}\"],\"env\":[{\"name\":\"PGPASSWORD\",\"valueFrom\":{\"secretKeyRef\":{\"name\":\"postgres\",\"key\":\"POSTGRES_PASSWORD\"}}}],\"volumeMounts\":[{\"name\":\"b\",\"mountPath\":\"/backups\"}]}],\"volumes\":[{\"name\":\"b\",\"persistentVolumeClaim\":{\"claimName\":\"postgres-backups\"}}]}}"
