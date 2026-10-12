#!/usr/bin/env bash
# Builds the modified FireRed ROM with the project-local toolchain
# (see scripts/setup-{macos,arch}.sh) and puts a gzipped copy in web/game/.
#
#   scripts/build-rom.sh            # normal build
#   scripts/build-rom.sh clean      # any extra args are passed to make
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TC="$ROOT/.toolchain"
GAME="$ROOT/pokefirered"

if [ ! -x "$TC/arm-none-eabi/bin/arm-none-eabi-gcc" ]; then
  echo "Toolchain missing. Run scripts/setup-macos.sh (macOS) or scripts/setup-arch.sh (Arch Linux) first." >&2
  exit 1
fi

if [ "$(uname -s)" = Darwin ]; then
  if [ ! -f "$TC/libpng/lib/libpng.a" ]; then
    echo "libpng missing. Run scripts/setup-macos.sh first." >&2
    exit 1
  fi
  export CPATH="$TC/libpng/include${CPATH:+:$CPATH}"
  export LIBRARY_PATH="$TC/libpng/lib${LIBRARY_PATH:+:$LIBRARY_PATH}"
  # macOS uses local static libpng without pkg-config/Homebrew.
  export PATH="$ROOT/scripts/shims:$PATH"
elif ! command -v pkg-config >/dev/null 2>&1 || ! pkg-config --exists libpng; then
  echo "libpng or pkg-config missing. On Arch Linux, run scripts/setup-arch.sh for instructions." >&2
  exit 1
fi

if [ ! -d "$GAME" ]; then
  echo "pokefirered/ missing. Run scripts/get-pokefirered.sh first." >&2
  exit 1
fi

JOBS="$(sysctl -n hw.ncpu 2>/dev/null || nproc)"

if [ "$#" -gt 0 ]; then
  make -C "$GAME" -j"$JOBS" TOOLCHAIN="$TC/arm-none-eabi" MODERN=1 "$@"
  exit 0
fi

make -C "$GAME" -j"$JOBS" TOOLCHAIN="$TC/arm-none-eabi" MODERN=1

ROM="$GAME/pokefirered_modern.gba"
# The website downloads a gzipped copy (16 MB -> ~5.5 MB, mostly 0xFF padding)
# and unpacks it in the browser, so it stays small on any static host.
mkdir -p "$ROOT/web/game"
rm -f "$ROOT/web/game/dailyred.gba"
gzip -9 -n -c "$ROM" > "$ROOT/web/game/dailyred.gba.gz"
# RED's battle sprite for the site's title card (from the decomp, like the ROM).
cp "$GAME/graphics/trainers/front_pics/red_front_pic.png" "$ROOT/web/game/red.png"
echo
echo "Built $ROM"
echo "Website copy: web/game/dailyred.gba.gz ($(wc -c < "$ROOT/web/game/dailyred.gba.gz" | tr -d ' ') bytes)"
