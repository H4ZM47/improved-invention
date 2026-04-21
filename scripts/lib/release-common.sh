#!/usr/bin/env bash
set -euo pipefail

readonly RELEASE_COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "${RELEASE_COMMON_DIR}/../.." && pwd)"

default_repo_owner="H4ZM47"
default_repo_name="improved-invention"

repo_owner="${REPO_OWNER:-}"
repo_name="${REPO_NAME:-}"

if [[ -z "${repo_owner}" || -z "${repo_name}" ]]; then
  remote_url="$(git -C "${REPO_ROOT}" config --get remote.origin.url || true)"
  if [[ "${remote_url}" =~ github\.com[:/]([^/]+)/([^/.]+)(\.git)?$ ]]; then
    repo_owner="${repo_owner:-${BASH_REMATCH[1]}}"
    repo_name="${repo_name:-${BASH_REMATCH[2]}}"
  fi
fi

readonly REPO_OWNER="${repo_owner:-${default_repo_owner}}"
readonly REPO_NAME="${repo_name:-${default_repo_name}}"

VERSION_VALUE="${VERSION_VALUE:-${VERSION:-}}"
if [[ -z "${VERSION_VALUE}" ]]; then
  short_sha="$(git -C "${REPO_ROOT}" rev-parse --short HEAD)"
  VERSION_VALUE="0.0.0-dev.${short_sha}"
fi

if [[ "${VERSION_VALUE}" == v* ]]; then
  VERSION_VALUE="${VERSION_VALUE#v}"
fi

readonly VERSION_VALUE
readonly TAG_VALUE="${TAG_VALUE:-v${VERSION_VALUE}}"
readonly COMMIT_VALUE="${COMMIT_VALUE:-${COMMIT:-$(git -C "${REPO_ROOT}" rev-parse HEAD)}}"
readonly DATE_VALUE="${DATE_VALUE:-${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}}"
readonly RELEASE_OUTPUT_DIR="${RELEASE_OUTPUT_DIR:-${REPO_ROOT}/dist/releases}"
readonly PACKAGE_OUTPUT_DIR="${PACKAGE_OUTPUT_DIR:-${REPO_ROOT}/dist/packages}"
readonly METADATA_OUTPUT_DIR="${METADATA_OUTPUT_DIR:-${REPO_ROOT}/dist/metadata}"
readonly RELEASE_BASE_URL="${RELEASE_BASE_URL:-https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${TAG_VALUE}}"
readonly NFPM_VERSION="${NFPM_VERSION:-v2.46.1}"

mkdir -p "${RELEASE_OUTPUT_DIR}" "${PACKAGE_OUTPUT_DIR}" "${METADATA_OUTPUT_DIR}"

archive_extension() {
  local goos="$1"
  if [[ "${goos}" == "windows" ]]; then
    printf '%s\n' "zip"
    return
  fi

  printf '%s\n' "tar.gz"
}

archive_filename() {
  local version="$1"
  local goos="$2"
  local goarch="$3"
  printf 'task_%s_%s_%s.%s\n' "${version}" "${goos}" "${goarch}" "$(archive_extension "${goos}")"
}

archive_path() {
  local version="$1"
  local goos="$2"
  local goarch="$3"
  printf '%s/%s\n' "${RELEASE_OUTPUT_DIR}" "$(archive_filename "${version}" "${goos}" "${goarch}")"
}

sha256_file() {
  local file="$1"
  shasum -a 256 "${file}" | awk '{print $1}'
}

write_checksums_file() {
  local output_path="$1"

  : > "${output_path}"
  while IFS= read -r artifact; do
    printf '%s  %s\n' "$(sha256_file "${artifact}")" "$(basename "${artifact}")" >> "${output_path}"
  done < <(find "${RELEASE_OUTPUT_DIR}" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) | sort)
}

build_release_archive() {
  local goos="$1"
  local goarch="$2"

  (
    cd "${REPO_ROOT}"
    env \
      GOOS_VALUE="${goos}" \
      GOARCH_VALUE="${goarch}" \
      VERSION="${VERSION_VALUE}" \
      COMMIT="${COMMIT_VALUE}" \
      DATE="${DATE_VALUE}" \
      OUTPUT_DIR="${RELEASE_OUTPUT_DIR}" \
      ./scripts/build-release.sh
  )
}
