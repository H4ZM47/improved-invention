#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/release-common.sh"

readonly WINGET_ROOT="${WINGET_ROOT:-${METADATA_OUTPUT_DIR}/winget/manifests/h/H4ZM47/Grind/${VERSION_VALUE}}"
readonly PACKAGE_IDENTIFIER="H4ZM47.Grind"
readonly DEFAULT_LOCALE="en-US"
readonly RELEASE_DATE="${DATE_VALUE%%T*}"

windows_archive="$(archive_path "${VERSION_VALUE}" windows amd64)"
if [[ ! -f "${windows_archive}" ]]; then
  echo "missing windows release archive under ${RELEASE_OUTPUT_DIR}" >&2
  exit 1
fi

mkdir -p "${WINGET_ROOT}"

windows_sha="$(sha256_file "${windows_archive}")"
installer_url="${RELEASE_BASE_URL}/$(archive_filename "${VERSION_VALUE}" windows amd64)"

cat > "${WINGET_ROOT}/${PACKAGE_IDENTIFIER}.yaml" <<EOF
PackageIdentifier: ${PACKAGE_IDENTIFIER}
PackageVersion: ${VERSION_VALUE}
DefaultLocale: ${DEFAULT_LOCALE}
ManifestType: version
ManifestVersion: 1.12.0
EOF

cat > "${WINGET_ROOT}/${PACKAGE_IDENTIFIER}.locale.${DEFAULT_LOCALE}.yaml" <<EOF
PackageIdentifier: ${PACKAGE_IDENTIFIER}
PackageVersion: ${VERSION_VALUE}
PackageLocale: ${DEFAULT_LOCALE}
Publisher: H4ZM47
PackageName: Grind
Moniker: grind
License: MIT
LicenseUrl: https://github.com/${REPO_OWNER}/${REPO_NAME}/blob/main/LICENSE
ShortDescription: Local-first task management CLI for humans and AI agents.
Homepage: https://github.com/${REPO_OWNER}/${REPO_NAME}
PackageUrl: https://github.com/${REPO_OWNER}/${REPO_NAME}
ManifestType: defaultLocale
ManifestVersion: 1.12.0
EOF

cat > "${WINGET_ROOT}/${PACKAGE_IDENTIFIER}.installer.yaml" <<EOF
PackageIdentifier: ${PACKAGE_IDENTIFIER}
PackageVersion: ${VERSION_VALUE}
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: grind.exe
    PortableCommandAlias: grind
ReleaseDate: ${RELEASE_DATE}
Installers:
  - Architecture: x64
    InstallerUrl: ${installer_url}
    InstallerSha256: ${windows_sha}
ManifestType: installer
ManifestVersion: 1.12.0
EOF

printf '%s\n' "${WINGET_ROOT}"
