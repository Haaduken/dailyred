#!/usr/bin/env bash
# Shared, pinned ARM compiler installation for the platform setup scripts.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TC="$ROOT/.toolchain"
GCC_VER="14.2.1-1.1"
GCC_NAME="xpack-arm-none-eabi-gcc-$GCC_VER"
GCC_HOST="${1:?Pass the xPack host platform}"
case "$GCC_HOST" in
  darwin-x64|darwin-arm64|linux-x64) ;;
  *) echo "Unsupported toolchain platform: $GCC_HOST" >&2; exit 1 ;;
esac

mkdir -p "$TC/downloads"
if [ ! -x "$TC/arm-none-eabi/bin/arm-none-eabi-gcc" ]; then
  tarball="$GCC_NAME-$GCC_HOST.tar.gz"
  url="https://github.com/xpack-dev-tools/arm-none-eabi-gcc-xpack/releases/download/v$GCC_VER/$tarball"
  echo ">> Downloading $tarball"
  curl -fL --retry 3 -o "$TC/downloads/$tarball" "$url"
  curl -fsSL --retry 3 -o "$TC/downloads/$tarball.sha" "$url.sha"
  expected="$(awk '{print $1}' "$TC/downloads/$tarball.sha")"
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$TC/downloads/$tarball" | awk '{print $1}')"
  else
    actual="$(shasum -a 256 "$TC/downloads/$tarball" | awk '{print $1}')"
  fi
  [ "$expected" = "$actual" ] || { echo "Checksum mismatch for $tarball" >&2; exit 1; }
  echo ">> Checksum OK, extracting"
  tar -xzf "$TC/downloads/$tarball" -C "$TC"
  rm -f "$TC/arm-none-eabi"
  ln -s "$GCC_NAME" "$TC/arm-none-eabi"
  rm -f "$TC/downloads/$tarball"
fi
"$TC/arm-none-eabi/bin/arm-none-eabi-gcc" --version
# Check that the compiler can run, not just that its executable exists.
echo 'int f(void){return 0;}' | "$TC/arm-none-eabi/bin/arm-none-eabi-gcc" -x c -c -o /dev/null -
