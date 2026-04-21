#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/release-common.sh"

readonly TEMPLATE_PATH="${REPO_ROOT}/packaging/nfpm.yaml.tmpl"
readonly README_PATH="${REPO_ROOT}/README.md"
readonly LICENSE_PATH="${REPO_ROOT}/LICENSE"

package_arches=()
if [[ $# -gt 0 ]]; then
  package_arches=("$@")
else
  package_arches=(amd64 arm64)
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

render_nfpm_config() {
  local arch="$1"
  local binary_path="$2"
  local output_path="$3"

  sed \
    -e "s|__ARCH__|${arch}|g" \
    -e "s|__VERSION__|${VERSION_VALUE}|g" \
    -e "s|__REPO_OWNER__|${REPO_OWNER}|g" \
    -e "s|__REPO_NAME__|${REPO_NAME}|g" \
    -e "s|__BINARY_PATH__|${binary_path}|g" \
    -e "s|__README_PATH__|${README_PATH}|g" \
    -e "s|__LICENSE_PATH__|${LICENSE_PATH}|g" \
    "${TEMPLATE_PATH}" > "${output_path}"
}

for arch in "${package_arches[@]}"; do
  stage_dir="${work_dir}/${arch}"
  mkdir -p "${stage_dir}"

  binary_path="${stage_dir}/task"
  config_path="${stage_dir}/nfpm.yaml"

  (
    cd "${REPO_ROOT}"
    CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" \
      go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION_VALUE} -X main.commit=${COMMIT_VALUE} -X main.date=${DATE_VALUE}" \
      -o "${binary_path}" \
      ./cmd/task
  )

  render_nfpm_config "${arch}" "${binary_path}" "${config_path}"

  (
    cd "${REPO_ROOT}"
    go run "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VERSION}" package \
      --config "${config_path}" \
      --packager deb \
      --target "${PACKAGE_OUTPUT_DIR}"
    go run "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VERSION}" package \
      --config "${config_path}" \
      --packager rpm \
      --target "${PACKAGE_OUTPUT_DIR}"
  )
done
