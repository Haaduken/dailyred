#!/usr/bin/env bash
# Uses Arch's native build dependencies and installs pinned ARM GCC locally.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ "$(uname -s)" != Linux ] || [ "$(uname -m)" != x86_64 ]; then
  echo "This setup script requires x86_64 Linux (Arch Linux)." >&2
  exit 1
fi

missing=0
for tool in cc c++ make ar git curl pkg-config unzip node npm sha256sum gzip tar; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Missing dependency: $tool" >&2
    missing=1
  fi
done
if ! command -v pkg-config >/dev/null 2>&1 || ! pkg-config --exists libpng; then
  echo "Missing dependency: libpng (including pkg-config metadata)" >&2
  missing=1
fi
if [ "$missing" -ne 0 ]; then
  echo "Install dependencies, then rerun this script:" >&2
  echo "  sudo pacman -Syu --needed base-devel curl git libpng nodejs npm unzip" >&2
  exit 1
fi

bash "$ROOT/scripts/setup-arm-toolchain.sh" linux-x64
echo
echo "Toolchain ready. Build with: scripts/build-rom.sh"
