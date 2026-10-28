#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi
if command -v go >/dev/null 2>&1; then
  training_go=go
elif [ -x ./tmp/toolchain/go/bin/go ]; then
  training_go=./tmp/toolchain/go/bin/go
else
  echo "需要Go 1.24或更高版本。" >&2
  exit 1
fi
export GOCACHE="${GOCACHE:-$PWD/tmp/go-cache}"
export GOMODCACHE="${GOMODCACHE:-$PWD/tmp/go-mod}"
exec "$training_go" run ./cmd/server
