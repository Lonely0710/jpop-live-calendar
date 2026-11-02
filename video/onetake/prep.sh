#!/usr/bin/env bash
# Builds everything comp.html loads into build/ (gitignored): the data from the
# app's local cache, the onetake move library, the app icon and the inlined
# Pacifico face (file:// pages cannot load remote or cross-origin fonts).
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
skill="$repo/.agents/skills/onetake"
build="$here/build"

[ -f "$skill/lib/motion.js" ] || { echo "onetake skill not found at $skill" >&2; exit 1; }
mkdir -p "$build"
(cd "$repo" && go run ./video/onetake/_gen -out "$build/data.js")
cp "$skill/lib/motion.js" "$build/motion.js"
cp "$repo/resources/icon.svg" "$build/icon.svg"
cp "$repo/resources/brands/sony.svg" "$build/sony.svg"
{
  printf "@font-face{font-family:'Pacifico';src:url(data:font/ttf;base64,"
  base64 < "$repo/resources/Pacifico-Regular.ttf" | tr -d '\n'
  printf ") format('truetype');}\n"
} > "$build/fonts.css"
echo "build/ ready"
