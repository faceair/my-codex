#!/usr/bin/env sh
set -eu

if command -v my-codex >/dev/null 2>&1; then
  exec my-codex sync "$@"
fi

go install ./cmd/my-codex
gobin="$(go env GOBIN)"
if [ -z "$gobin" ]; then
  gobin="$(go env GOPATH)/bin"
fi

exec "$gobin/my-codex" sync "$@"
