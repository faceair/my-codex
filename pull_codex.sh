#!/usr/bin/env sh
set -eu

if command -v my-codex >/dev/null 2>&1; then
  exec my-codex pull "$@"
fi

exec go run ./cmd/my-codex pull "$@"
