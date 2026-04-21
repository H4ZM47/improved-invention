#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/release-common.sh"

readonly FORMULA_OUTPUT="${FORMULA_OUTPUT:-${METADATA_OUTPUT_DIR}/homebrew-tap/Formula/task.rb}"

darwin_amd64_archive="$(archive_path "${VERSION_VALUE}" darwin amd64)"
darwin_arm64_archive="$(archive_path "${VERSION_VALUE}" darwin arm64)"

if [[ ! -f "${darwin_amd64_archive}" || ! -f "${darwin_arm64_archive}" ]]; then
  echo "missing darwin release archives under ${RELEASE_OUTPUT_DIR}" >&2
  exit 1
fi

mkdir -p "$(dirname "${FORMULA_OUTPUT}")"

darwin_amd64_sha="$(sha256_file "${darwin_amd64_archive}")"
darwin_arm64_sha="$(sha256_file "${darwin_arm64_archive}")"

cat > "${FORMULA_OUTPUT}" <<EOF
class Task < Formula
  desc "Local-first task management CLI for humans and AI agents"
  homepage "https://github.com/${REPO_OWNER}/${REPO_NAME}"
  license "MIT"
  version "${VERSION_VALUE}"

  if Hardware::CPU.arm?
    url "${RELEASE_BASE_URL}/$(archive_filename "${VERSION_VALUE}" darwin arm64)"
    sha256 "${darwin_arm64_sha}"
  else
    url "${RELEASE_BASE_URL}/$(archive_filename "${VERSION_VALUE}" darwin amd64)"
    sha256 "${darwin_amd64_sha}"
  end

  def install
    bin.install "task"
  end

  test do
    assert_match "version=", shell_output("#{bin}/task version")
  end
end
EOF

printf '%s\n' "${FORMULA_OUTPUT}"
