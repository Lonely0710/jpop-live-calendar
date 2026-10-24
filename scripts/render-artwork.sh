#!/usr/bin/env bash
# Render the artwork the packages show, from the SVG sources:
#
#   assets/dmg/background.svg     → assets/dmg/background.png (1x) and
#                                   assets/dmg/background@2x.png — the Finder
#                                   background of the .dmg (scripts/dmg-layout.py)
#   assets/installer/wizard.svg   → assets/installer/wizard.bmp (164x314, the
#                                   wizard bitmap of the Windows installer)
#   resources/icon.png            → assets/installer/app.ico, the installer's
#                                   icon (only with ImageMagick)
#
# The packages only read the rendered files, so this is needed after editing
# the artwork, not to build the app. Needs rsvg-convert (brew install librsvg);
# the .bmp goes through sips, which ships with macOS.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)

if ! command -v rsvg-convert >/dev/null 2>&1; then
  echo "render-artwork: rsvg-convert not found (brew install librsvg)" >&2
  exit 1
fi
if ! command -v magick >/dev/null 2>&1; then
  echo "render-artwork: ImageMagick not found (brew install imagemagick)" >&2
  exit 1
fi

# Both sizes come from the artwork, so the window keeps its proportions.
size() {
  local svg=$1 dim=$2
  grep -Eo "$dim=\"[0-9]+\"" "$svg" | head -n 1 | tr -dc '0-9'
}

dmg_svg="$root/assets/dmg/background.svg"
width=$(size "$dmg_svg" width)
height=$(size "$dmg_svg" height)
if [[ -z "$width" || -z "$height" ]]; then
  echo "render-artwork: no width/height in $dmg_svg" >&2
  exit 1
fi
rsvg-convert -w "$width" -h "$height" "$dmg_svg" -o "$root/assets/dmg/background.png"
rsvg-convert -w "$((width * 2))" -h "$((height * 2))" "$dmg_svg" -o "$root/assets/dmg/background@2x.png"
echo "→ assets/dmg/background.png (${width}x${height}), background@2x.png ($((width * 2))x$((height * 2)))"

wizard_svg="$root/assets/installer/wizard.svg"
wizard_width=$(size "$wizard_svg" width)
wizard_height=$(size "$wizard_svg" height)
if [[ -z "$wizard_width" || -z "$wizard_height" ]]; then
  echo "render-artwork: no width/height in $wizard_svg" >&2
  exit 1
fi
png=$(mktemp -t wizard).png
trap 'rm -f "$png"' EXIT
rsvg-convert -w "$wizard_width" -h "$wizard_height" "$wizard_svg" -o "$png"
# NSIS's image loader takes a plain 24-bit bottom-up bitmap only: sips writes
# a top-down DIB (negative height) which makensis rejects with "warning 5040:
# Unsupported format", leaving the page blank at run time. BMP3 is 24-bit,
# uncompressed and bottom-up.
magick "$png" -type TrueColor -depth 8 "BMP3:$root/assets/installer/wizard.bmp"
echo "→ assets/installer/wizard.bmp (${wizard_width}x${wizard_height}, 24-bit)"

magick "$root/resources/icon.png" -define icon:auto-resize=256,128,64,48,32,16 \
  "$root/assets/installer/app.ico"
echo "→ assets/installer/app.ico"
