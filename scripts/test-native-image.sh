#!/usr/bin/env bash
# Run on a native runner; inspect ELF separately before attempting execution.
set -euo pipefail
image=$1
arch=$2
case "$arch:$(uname -m)" in amd64:x86_64|arm64:aarch64|arm64:arm64) ;; *) echo 'Native runner architecture mismatch' >&2; exit 1 ;; esac
work=$(mktemp -d)
name="nexspence-smoke-$RANDOM"
cleanup() {
  docker logs "$name" > "$work/server.log" 2>&1 || true
  docker rm -f "$name" "$name-db" "$name-extract" >/dev/null 2>&1 || true
  docker network rm "$name" >/dev/null 2>&1 || true
}
trap cleanup EXIT
[ "$(docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}')" = "linux/$arch" ]
docker create --name "$name-extract" "$image" >/dev/null
docker cp "$name-extract:/app/nexspence" "$work/nexspence"
python3 scripts/check-image-elf.py "$work/nexspence" "$arch"
docker run --rm "$image" --help
docker network create "$name" >/dev/null
docker run -d --name "$name-db" --network "$name" -e POSTGRES_USER=test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=nexspence postgres:16-alpine >/dev/null
# The temporary init server accepts Unix sockets before restarting. Require
# TCP readiness so it cannot be mistaken for the final PostgreSQL process.
db_ready=false
for i in {1..60}; do
  if docker exec "$name-db" pg_isready -h 127.0.0.1 -U test -d nexspence >/dev/null 2>&1; then db_ready=true; break; fi
  sleep 1
done
if [ "$db_ready" != true ]; then docker logs "$name-db"; exit 1; fi
docker run -d --name "$name" --network "$name" \
  -e "NEXSPENCE_DATABASE_DSN=postgres://test:test@$name-db:5432/nexspence?sslmode=disable" \
  -e NEXSPENCE_BOOTSTRAP_ENABLED=false "$image" >/dev/null
for i in {1..90}; do
  if docker exec "$name" wget -q -O /dev/null http://localhost:8081/service/rest/v1/status/check; then
    echo "Native linux/$arch HTTP health check passed"
    exit 0
  fi
  if [ "$(docker inspect "$name" --format '{{.State.Running}}')" != true ]; then
    docker logs "$name"
    exit 1
  fi
  sleep 1
done
docker logs "$name"
exit 1
