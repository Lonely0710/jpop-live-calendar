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

# The stickers of the page are drawn from BUILD: this commit, and the share of
# Go the GitHub API reports for the repository. A visitor whose network cannot
# reach api.github.com keeps them; one who can gets the numbers of that moment.
# Without the stamp a sticker would still say "yesterday" a fortnight later.
stamp=$(git log -1 --format=%cI 2>/dev/null || true)
repo="Lonely0710/jpop-live-calendar"
share=$(curl -fsS --max-time 10 "https://api.github.com/repos/$repo/languages" 2>/dev/null |
	tr -d ' \n' | tr ',' '\n' | awk -F: '{ n = $2 + 0; total += n; if ($1 ~ /"Go"/) go = n }
		END { if (total > 0 && go > 0) printf "%.1f", 100*go/total }' || true)
if [ -n "$stamp" ] && [ -n "$share" ]; then
	perl -0pi -e "s/const BUILD = \{commit:'[^']*', go:[0-9.]+\}/const BUILD = {commit:'$stamp', go:$share}/" _site/index.html
	echo "site: stamped $stamp, go $share%"
else
	echo "site: kept the page's own stamp (no git or no GitHub API)"
fi

echo "site: $(du -sh _site | cut -f1) in _site/"
