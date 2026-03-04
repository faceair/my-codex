#!/usr/bin/env bash
set -euo pipefail

SRC_ROOT="${HOME}/.codex"
REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

SRC_AGENTS="${SRC_ROOT}/agents"
SRC_AGENTS_MD="${SRC_ROOT}/AGENTS.md"
SRC_CONFIG="${SRC_ROOT}/config.toml"

DST_AGENTS="${REPO_ROOT}/agents"
DST_AGENTS_MD="${REPO_ROOT}/AGENTS.md"
DST_CONFIG="${REPO_ROOT}/config.toml"

for required in "${SRC_AGENTS}" "${SRC_AGENTS_MD}" "${SRC_CONFIG}"; do
  if [[ ! -e "${required}" ]]; then
    echo "Missing source path: ${required}" >&2
    exit 1
  fi
done

# Keep destination agents directory fully in sync with source.
rm -rf "${DST_AGENTS}"
cp -a "${SRC_AGENTS}" "${DST_AGENTS}"

cp -f "${SRC_AGENTS_MD}" "${DST_AGENTS_MD}"
cp -f "${SRC_CONFIG}" "${DST_CONFIG}"

tmp_file="$(mktemp)"
trap 'rm -f "${tmp_file}"' EXIT

awk '
  function trim(s) {
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", s)
    return s
  }

  function section_name(line,   sec) {
    sec = line
    sub(/^[[:space:]]*\[/, "", sec)
    sub(/\][[:space:]]*$/, "", sec)
    sec = trim(sec)
    return sec
  }

  BEGIN {
    skip_section = 0
  }

  /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
    sec = section_name($0)
    if (sec ~ /^model_provider(\.|$)/ || sec ~ /^model_providers(\.|$)/) {
      skip_section = 1
      next
    }
    skip_section = 0
    print
    next
  }

  {
    if (skip_section) {
      next
    }

    line = $0
    if (line ~ /^[[:space:]]*[A-Za-z0-9_.-]+[[:space:]]*=/) {
      key = line
      sub(/=.*/, "", key)
      key = trim(key)
      if (key ~ /^model_provider([._-].*|$)/ || key ~ /^model_providers([._-].*|$)/) {
        next
      }
    }

    print
  }
' "${DST_CONFIG}" > "${tmp_file}"

mv "${tmp_file}" "${DST_CONFIG}"

echo "Synced to: ${REPO_ROOT}"
echo "Updated files:"
echo "  - ${DST_AGENTS}"
echo "  - ${DST_AGENTS_MD}"
echo "  - ${DST_CONFIG} (model_provider-related entries removed)"
