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

cleanup_files=()
cleanup() {
  local f
  for f in "${cleanup_files[@]}"; do
    if [[ -n "${f}" && -e "${f}" ]]; then
      rm -f "${f}"
    fi
  done
}
trap cleanup EXIT

if ! git -C "${REPO_ROOT}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Current directory is not a git repository: ${REPO_ROOT}" >&2
  exit 1
fi

if ! command -v codex >/dev/null 2>&1; then
  echo "Required command not found: codex" >&2
  exit 1
fi

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
cleanup_files+=("${tmp_file}")

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

cd "${REPO_ROOT}"
git add -- agents AGENTS.md config.toml

if git diff --cached --quiet -- agents AGENTS.md config.toml; then
  echo "Synced to: ${REPO_ROOT}"
  echo "No changes in sync targets. Skip commit and push."
  exit 0
fi

diff_file="$(mktemp)"
message_file="$(mktemp)"
cleanup_files+=("${diff_file}" "${message_file}")

git diff --cached -- agents AGENTS.md config.toml > "${diff_file}"

{
  echo "Write a concise English git commit subject line for the staged diff."
  echo "Rules:"
  echo "- Output exactly one line."
  echo "- Use imperative mood."
  echo "- Maximum 72 characters."
  echo "- Do not include quotes, markdown, or explanation."
  echo
  echo "Staged diff:"
  cat "${diff_file}"
} | codex exec --color never -o "${message_file}" -

commit_message="$(awk 'NF {gsub(/\r/, "", $0); print; exit}' "${message_file}")"

if [[ -z "${commit_message}" ]]; then
  echo "Failed to generate commit message from codex output." >&2
  exit 1
fi

git commit -m "${commit_message}"
git push

echo "Synced to: ${REPO_ROOT}"
echo "Updated files:"
echo "  - ${DST_AGENTS}"
echo "  - ${DST_AGENTS_MD}"
echo "  - ${DST_CONFIG} (model_provider-related entries removed)"
echo "Committed and pushed with message: ${commit_message}"
