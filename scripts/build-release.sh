#!/usr/bin/env bash
set -euo pipefail

GOOS_VALUE="${GOOS_VALUE:-${GOOS:-}}"
GOARCH_VALUE="${GOARCH_VALUE:-${GOARCH:-}}"
VERSION_VALUE="${VERSION_VALUE:-${VERSION:-dev}}"
COMMIT_VALUE="${COMMIT_VALUE:-${COMMIT:-unknown}}"
DATE_VALUE="${DATE_VALUE:-${DATE:-unknown}}"
OUTPUT_DIR="${OUTPUT_DIR:-dist/releases}"
OUTPUT_DIR="$(mkdir -p "${OUTPUT_DIR}" && cd "${OUTPUT_DIR}" && pwd)"

if [[ -z "${GOOS_VALUE}" ]]; then
  echo "GOOS_VALUE or GOOS is required" >&2
  exit 1
fi

if [[ -z "${GOARCH_VALUE}" ]]; then
  echo "GOARCH_VALUE or GOARCH is required" >&2
  exit 1
fi

base_name="grind_${VERSION_VALUE}_${GOOS_VALUE}_${GOARCH_VALUE}"
binary_name="grind"
archive_path=""

if [[ "${GOOS_VALUE}" == "windows" ]]; then
  binary_name="grind.exe"
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

binary_path="${work_dir}/${binary_name}"

CGO_ENABLED=0 GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" \
  go build \
  -trimpath \
  -ldflags "-s -w -X main.version=${VERSION_VALUE} -X main.commit=${COMMIT_VALUE} -X main.date=${DATE_VALUE}" \
  -o "${binary_path}" \
  ./cmd/grind

cp LICENSE "${work_dir}/LICENSE"
cp README.md "${work_dir}/README.md"

if [[ "${GOOS_VALUE}" == "windows" ]]; then
  archive_path="${OUTPUT_DIR}/${base_name}.zip"
  (
    cd "${work_dir}"
    zip -q "${archive_path}" "${binary_name}" LICENSE README.md
  )
else
  archive_path="${OUTPUT_DIR}/${base_name}.tar.gz"
  tar -C "${work_dir}" -czf "${archive_path}" "${binary_name}" LICENSE README.md
fi

echo "${archive_path}"
