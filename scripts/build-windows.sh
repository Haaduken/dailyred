#!/usr/bin/env bash
# Cross-builds a self-contained Windows launcher without bundling game content.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if ! command -v go >/dev/null 2>&1; then
  echo "Go is required. On Arch Linux: sudo pacman -Syu --needed go" >&2
  exit 1
fi

VENDOR_FILES=(nostalgist.js nostalgist.LICENSE.txt mgba_libretro.js mgba_libretro.wasm
              mgba.LICENSE.txt retroarch.COPYING.txt pixelify-sans.woff2 pixelify-sans.OFL.txt)
for file in "${VENDOR_FILES[@]}"; do
  if [ ! -f "$ROOT/web/vendor/$file" ]; then
    echo "Vendor asset missing: $file. Run scripts/fetch-vendor.sh first." >&2
    exit 1
  fi
done

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cp "$ROOT/desktop/"*.go "$ROOT/desktop/go.mod" "$TMP/"
mkdir -p "$TMP/assets/vendor"
cp "$ROOT/web/"*.js "$ROOT/web/index.html" "$ROOT/web/style.css" \
   "$ROOT/web/favicon.svg" "$ROOT/web/licenses.txt" "$TMP/assets/"
for file in "${VENDOR_FILES[@]}"; do
  cp "$ROOT/web/vendor/$file" "$TMP/assets/vendor/"
done

cd "$TMP"
go test ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o DailyRed.exe .
mkdir -p "$ROOT/dist"
cp DailyRed.exe "$ROOT/dist/DailyRed.exe"
cp "$ROOT/desktop/PLAY.txt" "$ROOT/dist/PLAY.txt"
echo "Built dist/DailyRed.exe (Windows x64). Share it with dist/PLAY.txt; no ROM is bundled."
