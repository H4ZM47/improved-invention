#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/release-common.sh"

readonly FORMULA_OUTPUT="${FORMULA_OUTPUT:-${METADATA_OUTPUT_DIR}/homebrew-tap/Formula/task.rb}"
readonly HOMEBREW_TAP_DIR="${HOMEBREW_TAP_DIR:-}"

if [[ -z "${HOMEBREW_TAP_DIR}" ]]; then
  echo "HOMEBREW_TAP_DIR is required" >&2
  exit 1
fi

"${SCRIPT_DIR}/generate-homebrew-formula.sh" >/dev/null

mkdir -p "${HOMEBREW_TAP_DIR}/Formula"
cp "${FORMULA_OUTPUT}" "${HOMEBREW_TAP_DIR}/Formula/task.rb"

printf '%s\n' "${HOMEBREW_TAP_DIR}/Formula/task.rb"
