#!/usr/bin/env bash
# Updates hatchet's two pinned third-party artifacts:
#   1. axe-core (third_party/axe/): axe.min.js + LICENSE + VERSION
#   2. chrome-headless-shell (internal/browser/install.go): version + sha256s
#
# Usage:
#   scripts/update-pins.sh axe <version>       e.g. scripts/update-pins.sh axe 4.11.0
#   scripts/update-pins.sh shell <version>     e.g. scripts/update-pins.sh shell 132.0.6834.83
#
# Version bumps change rule results and invalidate committed baselines —
# review the diff of a run against the fixture suite before merging.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"

update_axe() {
  local ver="$1"
  echo "Updating axe-core to ${ver}..."
  curl -sfL -o "${root}/third_party/axe/axe.min.js" \
    "https://cdn.jsdelivr.net/npm/axe-core@${ver}/axe.min.js"
  curl -sfL -o "${root}/third_party/axe/LICENSE" \
    "https://raw.githubusercontent.com/dequelabs/axe-core/v${ver}/LICENSE"
  printf '%s\n' "${ver}" > "${root}/third_party/axe/VERSION"
  echo "Done. Run 'go test ./...' and review rule-result changes."
}

update_shell() {
  local ver="$1"
  echo "Computing checksums for chrome-headless-shell ${ver} (4 downloads)..."
  local platforms=(linux64 mac-x64 mac-arm64 win64)
  local sums=()
  for p in "${platforms[@]}"; do
    local url="https://storage.googleapis.com/chrome-for-testing-public/${ver}/${p}/chrome-headless-shell-${p}.zip"
    echo "  ${p}..."
    sums+=("\"${p}\": \"$(curl -sfL "${url}" | sha256sum | cut -d' ' -f1)\",")
  done
  echo
  echo "Update internal/browser/install.go:"
  echo "  const ShellVersion = \"${ver}\""
  echo "  var shellChecksums = map[string]string{"
  for s in "${sums[@]}"; do echo "      ${s}"; done
  echo "  }"
}

case "${1:-}" in
  axe)   update_axe "${2:?usage: update-pins.sh axe <version>}" ;;
  shell) update_shell "${2:?usage: update-pins.sh shell <version>}" ;;
  *)     echo "usage: update-pins.sh {axe|shell} <version>" >&2; exit 1 ;;
esac
