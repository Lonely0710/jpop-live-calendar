#!/usr/bin/env bash
# Build the Windows installer of the app with packaging/windows/installer.nsi,
# which draws the artwork of assets/installer/wizard.bmp and speaks the three
# languages of the app. mygo makes an installer too, but with the default
# NSIS artwork and an English checkbox on the finish page; this one replaces
# it (scripts/package.sh calls this script and falls back to mygo's when
# makensis is missing).
#
# usage: scripts/windows-installer.sh --exe "build/windows-amd64/Live Calendar.exe" --out build/release/x-setup.exe
#
# Needs makensis (brew install makensis); the app name, version and
# identifier come from mygo.json.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)

usage() {
  echo "usage: $0 --exe <app.exe> --out <setup.exe>" >&2
  exit 2
}

exe=""
out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --exe) exe=${2:-}; shift 2 || usage ;;
    --out) out=${2:-}; shift 2 || usage ;;
    -h|--help) usage ;;
    *) echo "windows-installer: unknown argument $1" >&2; usage ;;
  esac
done
[[ -n "$exe" && -n "$out" ]] || usage

if ! command -v makensis >/dev/null 2>&1; then
  echo "windows-installer: makensis not found (brew install makensis)" >&2
  exit 1
fi

artwork="$root/assets/installer"
script="$root/packaging/windows/installer.nsi"
for f in "$exe" "$artwork/wizard.bmp" "$artwork/app.ico" "$script"; do
  if [[ ! -f "$f" ]]; then
    echo "windows-installer: missing $f" >&2
    exit 1
  fi
done

version=$(sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$root/mygo.json" | head -n 1)
name=$(sed -n 's/^[[:space:]]*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$root/mygo.json" | head -n 1)
identifier=$(sed -n 's/^[[:space:]]*"identifier"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$root/mygo.json" | head -n 1)
if [[ -z "$version" || -z "$name" || -z "$identifier" ]]; then
  echo "windows-installer: cannot read version, name or identifier from mygo.json" >&2
  exit 1
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/winsetup.XXXXXX")
trap 'rm -rf "$work"' EXIT

# makensis runs in the work directory, so the script refers to the staged
# files by simple names and no path with a space has to be quoted.
cp "$exe" "$work/app.exe"
cp "$artwork/app.ico" "$work/app.ico"
cp "$artwork/wizard.bmp" "$work/wizard.bmp"
sed -e "s|@NAME@|$name|g" \
  -e "s|@VERSION@|$version|g" \
  -e "s|@ID@|$identifier|g" \
  -e "s|@EXE@|$(basename "$exe")|g" \
  "$script" >"$work/installer.nsi"

if ! (cd "$work" && makensis -V2 installer.nsi >makensis.log 2>&1); then
  echo "windows-installer: makensis failed" >&2
  cat "$work/makensis.log" >&2
  exit 1
fi
# A warning here is not cosmetic: "warning 5040: Unsupported format" means the
# bitmap cannot be loaded, and the page shows blank at run time. Better to
# fail the build than to ship that.
if grep -qi "warning" "$work/makensis.log"; then
  echo "windows-installer: makensis reported warnings" >&2
  grep -i "warning" "$work/makensis.log" >&2
  exit 1
fi

mkdir -p "$(dirname "$out")"
mv "$work/out.exe" "$out"
echo "windows-installer: $(basename "$out") $(du -h "$out" | cut -f1 | tr -d ' ')"
