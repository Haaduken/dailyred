#!/usr/bin/env bash
# Installs everything needed to build the Daily Red ROM on macOS, into
# ./.toolchain (no sudo, no Homebrew, nothing written outside this project):
#
#   - xPack GNU Arm Embedded GCC (arm-none-eabi-gcc + binutils + newlib),
#     used by pokefirered's `modern` build target via TOOLCHAIN=...
#     (Arm's own macOS x86_64 tarballs link cc1 against Homebrew's zstd and
#     crash without it; the xPack builds are self-contained.)
#   - libpng (static), needed by pokefirered/tools/gbagfx
#
# Re-running is safe: finished steps are skipped.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TC="$ROOT/.toolchain"
mkdir -p "$TC/downloads"

LIBPNG_VER="1.6.44"

if [ "$(uname -s)" != Darwin ]; then
  echo "This setup script requires macOS. On Arch Linux, run scripts/setup-arch.sh." >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64) GCC_HOST="darwin-x64" ;;
  arm64)  GCC_HOST="darwin-arm64" ;;
  *) echo "Unsupported CPU: $(uname -m)"; exit 1 ;;
esac

if ! xcode-select -p >/dev/null 2>&1; then
  echo "Xcode Command Line Tools are required. Run: xcode-select --install"
  exit 1
fi

bash "$ROOT/scripts/setup-arm-toolchain.sh" "$GCC_HOST"

# --- libpng (static, built with the system clang) ----------------------------
if [ ! -f "$TC/libpng/lib/libpng.a" ]; then
  echo ">> Building libpng $LIBPNG_VER"
  src="$TC/downloads/libpng-$LIBPNG_VER"
  rm -rf "$src"
  curl -fL --retry 3 -o "$src.tar.gz" \
    "https://github.com/pnggroup/libpng/archive/refs/tags/v$LIBPNG_VER.tar.gz"
  tar -xzf "$src.tar.gz" -C "$TC/downloads"
  cp "$src/scripts/pnglibconf.h.prebuilt" "$src/pnglibconf.h"
  mkdir -p "$TC/libpng/include" "$TC/libpng/lib" "$src/obj"
  # Disable the optional SIMD paths so no extra arch-specific sources are needed.
  for f in png pngerror pngget pngmem pngpread pngread pngrio pngrtran pngrutil \
           pngset pngtrans pngwio pngwrite pngwtran pngwutil; do
    cc -O2 -DPNG_ARM_NEON_OPT=0 -DPNG_INTEL_SSE_OPT=0 -I"$src" \
       -c "$src/$f.c" -o "$src/obj/$f.o"
  done
  ar rcs "$TC/libpng/lib/libpng.a" "$src"/obj/*.o
  cp "$src/png.h" "$src/pngconf.h" "$src/pnglibconf.h" "$TC/libpng/include/"
fi
echo ">> libpng: $TC/libpng/lib/libpng.a"

echo
echo "Toolchain ready. Build with: scripts/build-rom.sh"
