#!/usr/bin/env bash
set -euo pipefail

SRC_ROOT="${HOME}/.codex"
REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

SRC_AGENTS="${SRC_ROOT}/agents"
SRC_CONFIG="${SRC_ROOT}/config.toml"
SRC_PROMPTS="${SRC_ROOT}/prompts"
SRC_INSTRUCTIONS="${SRC_ROOT}/instructions"

DST_AGENTS="${REPO_ROOT}/agents"
DST_CONFIG="${REPO_ROOT}/config.toml"
DST_PROMPTS="${REPO_ROOT}/prompts"
DST_INSTRUCTIONS="${REPO_ROOT}/instructions"

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

for required in "${SRC_AGENTS}" "${SRC_CONFIG}" "${SRC_PROMPTS}"; do
  if [[ ! -e "${required}" ]]; then
    echo "Missing source path: ${required}" >&2
    exit 1
  fi
done

# Keep destination agents directory fully in sync with source.
rm -rf "${DST_AGENTS}"
cp -a "${SRC_AGENTS}" "${DST_AGENTS}"
rm -rf "${DST_PROMPTS}"
cp -a "${SRC_PROMPTS}" "${DST_PROMPTS}"
if [[ -d "${SRC_INSTRUCTIONS}" ]]; then
  rm -rf "${DST_INSTRUCTIONS}"
  cp -a "${SRC_INSTRUCTIONS}" "${DST_INSTRUCTIONS}"
else
  rm -rf "${DST_INSTRUCTIONS}"
fi

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
    if (sec ~ /^model_provider(\.|$)/ || sec ~ /^model_providers(\.|$)/ || sec ~ /^projects(\.|$)/) {
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
    if (line ~ /^[[:space:]]*projects[[:space:]]*=/) {
      next
    }

    if (line ~ /^[[:space:]]*[A-Za-z0-9_.-]+[[:space:]]*=/) {
      key = line
      sub(/=.*/, "", key)
      key = trim(key)
      if (key ~ /^model_provider([._-].*|$)/ || key ~ /^model_providers([._-].*|$)/ || key ~ /^projects([._-].*|$)/) {
        next
      }
    }

    print
  }
' "${DST_CONFIG}" > "${tmp_file}"

mv "${tmp_file}" "${DST_CONFIG}"

cd "${REPO_ROOT}"
git add -- agents prompts instructions config.toml

if git diff --cached --quiet -- agents prompts instructions config.toml; then
  echo "Synced to: ${REPO_ROOT}"
  echo "No changes in sync targets. Skip commit and push."
  exit 0
fi

diff_file="$(mktemp)"
raw_message_file="$(mktemp)"
commit_message_file="$(mktemp)"
cleanup_files+=("${diff_file}" "${raw_message_file}" "${commit_message_file}")

git diff --cached -- agents prompts instructions config.toml > "${diff_file}"

smart_commit_prompt_file="${DST_PROMPTS}/smart-commit.md"
if [[ ! -f "${smart_commit_prompt_file}" ]]; then
  echo "Missing smart commit prompt: ${smart_commit_prompt_file}" >&2
  exit 1
fi

{
  cat "${smart_commit_prompt_file}"
  echo
  echo "Automation-specific overrides:"
  echo "- Do not run git status, git diff, git add, git commit, or git push."
  echo "- The staged diff is provided below, so do not ask for more input."
  echo "- Output only the git commit message text."
  echo "- The first line must be the summary line."
  echo "- If a body is needed, separate it from the summary with a blank line."
  echo "- Do not include code fences, quotes, markdown, or explanations."
  echo
  echo "Staged diff:"
  cat "${diff_file}"
} | codex exec --color never -o "${raw_message_file}" -

awk '
  {
    gsub(/\r/, "", $0)
    lines[++count] = $0
  }

  END {
    start = 1
    while (start <= count && lines[start] ~ /^[[:space:]]*$/) {
      start++
    }

    end = count
    while (end >= start && lines[end] ~ /^[[:space:]]*$/) {
      end--
    }

    for (i = start; i <= end; i++) {
      print lines[i]
    }
  }
' "${raw_message_file}" > "${commit_message_file}"

commit_message="$(awk 'NF {print; exit}' "${commit_message_file}")"

if [[ -z "${commit_message}" ]]; then
  echo "Failed to generate commit message from codex output." >&2
  exit 1
fi

git commit -F "${commit_message_file}"
git push

echo "Synced to: ${REPO_ROOT}"
echo "Updated files:"
echo "  - ${DST_AGENTS}"
echo "  - ${DST_PROMPTS}"
echo "  - ${DST_INSTRUCTIONS} (synced when source exists, removed when absent)"
echo "  - ${DST_CONFIG} (model_provider/projects-related entries removed)"
echo "Committed and pushed with message:"
cat "${commit_message_file}"
