#!/usr/bin/env bash
# Build the app and collect what ships, per version, in one place:
#
#   build/release/<version>/
#     jpop-live-calendar-<goos>-<goarch>-<version>.<ext>
#     SHA256SUMS.txt
#
# Everything else under build/ is an intermediate: the builds mygo and this
# script make are removed once the packages are in the release folder.
#
# usage: scripts/package.sh [platform|all|clean]
#   scripts/package.sh                     # the host platform
#   scripts/package.sh all                 # macOS + Windows in one go
#   scripts/package.sh darwin/arm64
#   scripts/package.sh darwin/universal
#   scripts/package.sh windows/amd64
#   scripts/package.sh clean               # drop intermediates, keep release/
#
# The version comes from mygo.json.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

usage() {
  echo "usage: $0 [goos/goarch|all|clean] (for example windows/amd64)" >&2
  exit 2
}

version=$(sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' mygo.json | head -n 1)
if [[ -z "$version" ]]; then
  echo "package: cannot read version from mygo.json" >&2
  exit 1
fi

release="build/release/$version"

# Whether this run put a package in the release folder; the checksums are
# written once every platform of the run is in.
count=0

# built_dirs collects the per-platform build directories of this run, which
# are intermediates once their packages are in the release folder.
built_dirs=()

# add_artifact counts a file this run produced.
add_artifact() {
  count=$((count + 1))
}

# human-readable size, for the summary
sizeof() {
  du -h "$1" | cut -f1 | tr -d ' '
}

# package_platform goos/goarch builds one platform and moves its artifact
# into the release folder.
package_platform() {
  local platform=$1 goos goarch slug base built
  goos=${platform%%/*}
  goarch=${platform#*/}
  slug="$goos-$goarch"
  base="jpop-live-calendar-$slug-$version"
  built="build/$slug"
  built_dirs[${#built_dirs[@]}]=$built

  echo "package: $base"
  go tool mygo build -platform "$platform"

  mkdir -p "$release"
  rm -f "$release/$base.exe" "$release/$base.zip" "$release/$base.dmg" \
    "$release/$base.tar.gz" "$release/$base.deb" "$release/$base-setup.exe"

  case "$goos" in
    windows)
      # The executable stands alone: main.go embeds the tray icons and the
      # script font with go:embed, and mygo embeds the window icon in the
      # executable, so the copies of resources/ that mygo puts next to it are
      # not read at run time. It therefore ships as itself, one file, with no
      # zip to unpack first. If the app ever reads resources/ at run time, it
      # is this that has to ship them too.
      local exe
      exe=$(find "$built" -maxdepth 1 -name '*.exe' ! -name '* Setup *' -print -quit)
      if [[ -z "$exe" ]]; then
        echo "package: no .exe in $built" >&2
        exit 1
      fi
      cp "$exe" "$release/$base.exe"
      add_artifact "$release/$base.exe"
      # Our own installer, not mygo's: it draws assets/installer/wizard.bmp
      # instead of the NSIS default artwork, says the finish-page checkbox in
      # the app's three languages, and installs the executable alone (it
      # needs no resource files next to it). mygo's is the fallback.
      if command -v makensis >/dev/null 2>&1; then
        scripts/windows-installer.sh --exe "$exe" --out "$release/$base-setup.exe"
        add_artifact "$release/$base-setup.exe"
      else
        local setup="$built/Live Calendar Setup $version $goarch.exe"
        if [[ -f "$setup" ]]; then
          echo "package: no makensis, shipping mygo's installer with the NSIS artwork" >&2
          cp "$setup" "$release/$base-setup.exe"
          add_artifact "$release/$base-setup.exe"
        fi
      fi
      ;;
    darwin)
      local dmg app
      dmg=$(find "$built" -maxdepth 1 -name '*.dmg' -print -quit)
      app=$(find "$built" -maxdepth 1 -name '*.app' -print -quit)
      if [[ -n "$dmg" ]]; then
        cp "$dmg" "$release/$base.dmg"
        # The window of mygo's image is white; put the artwork of
        # assets/dmg/background.png behind the app and the Applications link.
        if [[ -n "$app" ]] && command -v python3 >/dev/null 2>&1; then
          python3 scripts/dmg-layout.py "$release/$base.dmg" \
            --background assets/dmg/background.png \
            --background-2x "assets/dmg/background@2x.png" \
            --app-name "$(basename "$app")"
        else
          echo "package: no python3 or no .app, keeping the plain Finder window" >&2
        fi
        add_artifact "$release/$base.dmg"
      else
        app="$built/Live Calendar.app"
        if [[ ! -d "$app" ]]; then
          echo "package: no .app or .dmg in $built" >&2
          exit 1
        fi
        ditto -c -k --sequesterRsrc --keepParent "$app" "$release/$base.zip"
        add_artifact "$release/$base.zip"
      fi
      ;;
    linux)
      local archive deb
      archive=$(find "$built" -maxdepth 1 -name '*.tar.gz' -print -quit)
      if [[ -n "$archive" ]]; then
        cp "$archive" "$release/$base.tar.gz"
        add_artifact "$release/$base.tar.gz"
      fi
      deb=$(find "$built" -maxdepth 1 -name '*.deb' -print -quit)
      if [[ -n "$deb" ]]; then
        cp "$deb" "$release/$base.deb"
        add_artifact "$release/$base.deb"
      fi
      if [[ -z "${archive:-}${deb:-}" ]]; then
        echo "package: no .tar.gz or .deb in $built" >&2
        exit 1
      fi
      ;;
    *)
      echo "package: unsupported platform $platform" >&2
      exit 2
      ;;
  esac
}

# finish writes the checksums of every package in the release folder — from
# the folder, not from this run, so packaging one platform after the other
# keeps them complete — then drops the intermediate builds.
finish() {
  [[ $count -gt 0 ]] || return 0

  local f
  local parts=()
  for f in "$release"/jpop-live-calendar-*; do
    [[ -f "$f" ]] || continue
    parts[${#parts[@]}]=$(basename "$f")
  done
  [[ ${#parts[@]} -gt 0 ]] || return 0

  local sum
  if command -v shasum >/dev/null 2>&1; then
    sum="shasum -a 256"
  else
    sum="sha256sum"
  fi
  (cd "$release" && $sum "${parts[@]}" >SHA256SUMS.txt)

  rm -rf build/.DS_Store "$release/.DS_Store"
  local dir
  for dir in "${built_dirs[@]}"; do
    rm -rf "$dir"
  done

  echo
  echo "release: $release/"
  for f in "$release"/*; do
    printf '  %-48s %s\n' "$(basename "$f")" "$(sizeof "$f")"
  done
}

case "${1:-}" in
  clean)
    # Intermediates only: the per-platform builds of mygo and the staging of
    # this script. build/release/ holds the packages and stays.
    rm -rf build/.staging-* build/.DS_Store
    for dir in build/*/; do
      [[ -d "$dir" ]] || continue
      [[ "$(basename "$dir")" == release ]] || rm -rf "$dir"
    done
    echo "package: removed intermediates; kept build/release/"
    exit 0
    ;;
  all)
    platforms=(windows/amd64)
    if [[ "$(uname -s)" == Darwin ]]; then
      platforms=(darwin/arm64 windows/amd64)
    else
      echo "package: not macOS, so no .dmg; packaging Windows only" >&2
    fi
    for platform in "${platforms[@]}"; do
      package_platform "$platform"
    done
    finish
    ;;
  "")
    package_platform "$(go env GOOS)/$(go env GOARCH)"
    finish
    ;;
  */*)
    package_platform "$1"
    finish
    ;;
  *)
    usage
    ;;
esac
