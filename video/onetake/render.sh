#!/usr/bin/env bash
# Render the promo with the onetake skill in .agents/skills/onetake.
#   ./render.sh fast [lang]    no motion blur, ~1–2 min — for reviewing a cut
#   ./render.sh draft [lang]   1080p30 with shutter blur, ~8 min
#   ./render.sh final [lang]   3840×2160 at 60 fps, only after the cut is accepted (much longer)
# lang: zh (default), ja, en. Live preview without rendering: open "comp.html?play" in Chrome.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
skill="$here/../../.agents/skills/onetake"
py="$skill/.venv/bin/python"
mode="${1:-fast}" lang="${2:-zh}"
cd "$here" && ./prep.sh >/dev/null && mkdir -p out
case "$mode" in
  fast)  "$py" "$skill/scripts/render.py" "comp.html?lang=$lang" --out "out/fast-$lang.mp4" --shutter 0 ;;
  draft) "$py" "$skill/scripts/render.py" "comp.html?lang=$lang" --out "out/draft-$lang.mp4" ;;
  final) "$py" "$skill/scripts/render.py" "comp.html?lang=$lang" --out "out/promo-$lang.mp4" --final ;;
  *) echo "usage: $0 fast|draft|final [zh|ja|en]" >&2; exit 2 ;;
esac
