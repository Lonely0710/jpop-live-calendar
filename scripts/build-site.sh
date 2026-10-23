#!/usr/bin/env bash
# Builds the website into _site/ with live data: syncs the official sources
# of config/sources.yaml once and writes _site/data/events.json.
# Cloudflare Pages: build command `bash scripts/build-site.sh`, output `_site`.
set -euo pipefail
cd "$(dirname "$0")/.."
rm -rf _site
mkdir -p _site/assets
go run ./cmd/export -o _site/data/events.json -cache .cache/cache.json
cp -r docs/. _site/
cp -r assets/screenshots _site/assets/
echo "site: $(du -sh _site | cut -f1) in _site/"
