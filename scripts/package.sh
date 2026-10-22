#!/usr/bin/env bash
# Build the app and copy the release artifacts to:
#   jpop-live-calendar-<goos>-<goarch>-<version>.<ext>
#
# usage: scripts/package.sh [goos/goarch]
#   scripts/package.sh windows/amd64
#   scripts/package.sh darwin/universal
#   scripts/package.sh linux/amd64
#
# The version comes from mygo.json. The build under build/<goos>-<goarch>/
# is kept as well; the named archive is what to upload or hand to users.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

platform=${1:-"$(go env GOOS)/$(go env GOARCH)"}
if [[ "$platform" != */* ]]; then
  echo "usage: $0 [goos/goarch] (for example windows/amd64)" >&2
  exit 2
fi
goos=${platform%%/*}
goarch=${platform#*/}

version=$(sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' mygo.json | head -n 1)
if [[ -z "$version" ]]; then
  echo "package: cannot read version from mygo.json" >&2
  exit 1
fi

slug="$goos-$goarch"
base="jpop-live-calendar-$slug-$version"
built="build/$slug"

echo "package: $base"
go tool mygo build -platform "$platform"

rm -f "build/$base.zip" "build/$base.dmg" "build/$base.tar.gz" "build/$base.deb" "build/$base-setup.exe"

case "$goos" in
  windows)
    # The executable needs the resources mygo copied next to it, so ship
    # the whole folder rather than a lone .exe.
    stage="build/$base"
    rm -rf "$stage"
    mkdir -p "$stage"
    cp -R "$built/." "$stage/"
    # The portable zip holds the app, not the optional installer.
    find "$stage" -maxdepth 1 -name '* Setup *.exe' -delete
    (cd build && zip -qry "$base.zip" "$base" -x "*.DS_Store")
    echo "→ build/$base.zip"
    setup="$built/Live Calendar Setup $version $goarch.exe"
    if [[ -f "$setup" ]]; then
      cp "$setup" "build/$base-setup.exe"
      echo "→ build/$base-setup.exe"
    fi
    ;;
  darwin)
    dmg=$(find "$built" -maxdepth 1 -name '*.dmg' -print -quit)
    if [[ -n "$dmg" ]]; then
      cp "$dmg" "build/$base.dmg"
      echo "→ build/$base.dmg"
    else
      app="$built/Live Calendar.app"
      if [[ ! -d "$app" ]]; then
        echo "package: no .app or .dmg in $built" >&2
        exit 1
      fi
      ditto -c -k --sequesterRsrc --keepParent "$app" "build/$base.zip"
      echo "→ build/$base.zip"
    fi
    ;;
  linux)
    archive=$(find "$built" -maxdepth 1 -name '*.tar.gz' -print -quit)
    if [[ -n "$archive" ]]; then
      cp "$archive" "build/$base.tar.gz"
      echo "→ build/$base.tar.gz"
    fi
    deb=$(find "$built" -maxdepth 1 -name '*.deb' -print -quit)
    if [[ -n "$deb" ]]; then
      cp "$deb" "build/$base.deb"
      echo "→ build/$base.deb"
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
