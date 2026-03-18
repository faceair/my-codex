#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${CODEX_REPO_URL:-https://github.com/faceair/my-codex.git}"
DEST_ROOT="${HOME}/.codex"

SRC_REPO_DIR=""
TMP_FILES=()
TMP_DIRS=()

cleanup() {
  local f=""
  local d=""

  for f in "${TMP_FILES[@]}"; do
    if [[ -n "${f}" && -e "${f}" ]]; then
      rm -f "${f}"
    fi
  done

  for d in "${TMP_DIRS[@]}"; do
    if [[ -n "${d}" && -d "${d}" ]]; then
      rm -rf "${d}"
    fi
  done
}
trap cleanup EXIT

extract_provider_root_keys() {
  local input_file="$1"
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
      in_provider_section = 0
    }

    /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
      sec = section_name($0)
      if (sec ~ /^model_provider(\.|$)/ || sec ~ /^model_providers(\.|$)/ || sec ~ /^projects(\.|$)/) {
        in_provider_section = 1
      } else {
        in_provider_section = 0
      }
      next
    }

    {
      if (in_provider_section) {
        next
      }

      line = $0
      if (line ~ /^[[:space:]]*projects[[:space:]]*=/) {
        print line
      }

      if (line ~ /^[[:space:]]*[A-Za-z0-9_.-]+[[:space:]]*=/) {
        key = line
        sub(/=.*/, "", key)
        key = trim(key)
        if (key ~ /^model_provider([._-].*|$)/ || key ~ /^model_providers([._-].*|$)/ || key ~ /^projects([._-].*|$)/) {
          print line
        }
      }
    }
  ' "${input_file}"
}

extract_provider_sections() {
  local input_file="$1"
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
      keep_section = 0
    }

    /^[[:space:]]*\[[^]]+\][[:space:]]*$/ {
      sec = section_name($0)
      if (sec ~ /^model_provider(\.|$)/ || sec ~ /^model_providers(\.|$)/ || sec ~ /^projects(\.|$)/) {
        keep_section = 1
        print
        next
      }
      keep_section = 0
      next
    }

    {
      if (keep_section) {
        print
      }
    }
  ' "${input_file}"
}

strip_provider_blocks() {
  local input_file="$1"
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
  ' "${input_file}"
}

for required_cmd in git awk mktemp cp rm; do
  if ! command -v "${required_cmd}" >/dev/null 2>&1; then
    echo "Missing required command: ${required_cmd}" >&2
    exit 1
  fi
done

mkdir -p "${DEST_ROOT}"

provider_backup_file="$(mktemp)"
provider_section_backup_file="$(mktemp)"
TMP_FILES+=("${provider_backup_file}" "${provider_section_backup_file}")

dest_config="${DEST_ROOT}/config.toml"
if [[ -f "${dest_config}" ]]; then
  extract_provider_root_keys "${dest_config}" > "${provider_backup_file}"
  extract_provider_sections "${dest_config}" > "${provider_section_backup_file}"
fi

clone_dir="$(mktemp -d)"
TMP_DIRS+=("${clone_dir}")
SRC_REPO_DIR="${clone_dir}/repo"
clone_log_file="$(mktemp)"
TMP_FILES+=("${clone_log_file}")

if ! git clone --depth 1 "${REPO_URL}" "${SRC_REPO_DIR}" >"${clone_log_file}" 2>&1; then
  echo "Failed to clone repository: ${REPO_URL}" >&2
  echo "git clone output:" >&2
  sed -n '1,120p' "${clone_log_file}" >&2
  exit 1
fi

src_agents="${SRC_REPO_DIR}/agents"
src_config="${SRC_REPO_DIR}/config.toml"
src_prompts="${SRC_REPO_DIR}/prompts"
src_instructions="${SRC_REPO_DIR}/instructions"

for required_path in "${src_agents}" "${src_config}" "${src_prompts}"; do
  if [[ ! -e "${required_path}" ]]; then
    echo "Missing required path in repository: ${required_path}" >&2
    exit 1
  fi
done

rm -rf "${DEST_ROOT}/agents"
cp -a "${src_agents}" "${DEST_ROOT}/agents"
rm -rf "${DEST_ROOT}/prompts"
cp -a "${src_prompts}" "${DEST_ROOT}/prompts"
if [[ -d "${src_instructions}" ]]; then
  rm -rf "${DEST_ROOT}/instructions"
  cp -a "${src_instructions}" "${DEST_ROOT}/instructions"
else
  rm -rf "${DEST_ROOT}/instructions"
fi
cp -f "${src_config}" "${dest_config}"

cleaned_config_file="$(mktemp)"
config_with_root_keys_file="$(mktemp)"
merged_config_file="$(mktemp)"
TMP_FILES+=("${cleaned_config_file}" "${config_with_root_keys_file}" "${merged_config_file}")

strip_provider_blocks "${dest_config}" > "${cleaned_config_file}"

if [[ -s "${provider_backup_file}" ]]; then
  # Insert root provider keys before the first TOML section header.
  awk -v insert_file="${provider_backup_file}" '
    BEGIN {
      inserted = 0
      content = ""
      while ((getline line < insert_file) > 0) {
        content = content line "\n"
      }
      close(insert_file)
    }

    /^[[:space:]]*\[[^]]+\][[:space:]]*$/ && !inserted {
      if (length(content) > 0) {
        printf "%s\n", content
      }
      inserted = 1
    }

    {
      print
    }

    END {
      if (!inserted && length(content) > 0) {
        if (NR > 0) {
          printf "\n"
        }
        printf "%s", content
      }
    }
  ' "${cleaned_config_file}" > "${config_with_root_keys_file}"
else
  cp -f "${cleaned_config_file}" "${config_with_root_keys_file}"
fi

cat "${config_with_root_keys_file}" > "${merged_config_file}"

if [[ -s "${provider_section_backup_file}" ]]; then
  if [[ -s "${merged_config_file}" ]]; then
    printf "\n" >> "${merged_config_file}"
  fi
  cat "${provider_section_backup_file}" >> "${merged_config_file}"
fi

mv "${merged_config_file}" "${dest_config}"

echo "Pulled from: ${REPO_URL}"
echo "Updated:"
echo "  - ${DEST_ROOT}/agents"
echo "  - ${DEST_ROOT}/prompts"
echo "  - ${DEST_ROOT}/instructions (synced when repository path exists, removed when absent)"
echo "  - ${DEST_ROOT}/config.toml (kept local model_provider/projects-related entries)"
