#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/release-common.sh"

build_release_archive linux amd64
build_release_archive linux arm64
build_release_archive darwin amd64
build_release_archive darwin arm64
build_release_archive windows amd64

write_checksums_file "${RELEASE_OUTPUT_DIR}/checksums.txt"

"${SCRIPT_DIR}/build-linux-packages.sh" amd64 arm64
"${SCRIPT_DIR}/generate-homebrew-formula.sh"
"${SCRIPT_DIR}/generate-winget-manifests.sh"

printf 'release_dir=%s\npackages_dir=%s\nmetadata_dir=%s\n' \
  "${RELEASE_OUTPUT_DIR}" \
  "${PACKAGE_OUTPUT_DIR}" \
  "${METADATA_OUTPUT_DIR}"
