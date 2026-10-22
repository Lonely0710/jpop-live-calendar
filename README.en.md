# Live Calendar · Japanese Music Live Calendar

<p align="center">
  <a href="#changelog"><img src="https://img.shields.io/badge/version-1.0.1-E85A3A?style=flat-square&labelColor=2E2A28" alt="Version 1.0.1"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-2E2A28?style=flat-square" alt="License MIT"></a>
  <a href="https://github.com/egoist/mygo"><img src="https://img.shields.io/badge/MyGo-v0.5.5-3D86F5?style=flat-square&labelColor=2E2A28&logo=go&logoColor=white" alt="MyGo v0.5.5"></a>
</p>

<p align="center">
  <a href="README.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <b>English</b>
</p>

<p align="center">
  <img src="resources/icon.png" width="112" alt="Live Calendar app icon">
</p>

A native macOS / Windows desktop app (**MyGo Native UI + Go**) that turns public information about Japanese music lives — lottery and presale applications, result announcements, general sales — into a calendar. No database, no login.

![Month view (real data)](assets/screenshots/month-live.png)

## Features

- **Four views**: month / week / day / list. Use ⌘← / ⌘→ to page and ⌘T for today. A live show is a solid bar with a color stripe; application start / deadline markers, result announcements, and general sales are outlined pill badges with a dot — deadlines in bold, postponed/cancelled in red.
- **Ticket rounds at a glance**: each application round, result announcement, and general sale for a show is its own node on the calendar, with its own countdown and application / purchase link.
- **Everything traceable**: every show and ticket node links back to the source announcement — publish time, fetch time, revision, and the parsed text, plus a button to open the original.
- **Public information only**: 21 artists and 34 official / public sources by default (official sites, RSS, Sony Music public JSON). No login, no members-only pages, no ticket-platform scraping; robots.txt is respected, and requests to the same site are serialized with at least 1 second between them.
- **No guessing missing fields**: an unpublished time reads "Time not announced", a missing venue reads "Venue not announced", and uncertain inferences are marked "Needs review". Postponements, cancellations, and deadline changes go into the change log instead of silently overwriting the old value.
- **Local snapshots + background sync**: public announcements and avatars are cached in `cache.json` / `avatars.json`, so a restart shows the previous result immediately. A failing source only records its own failure; other sources and existing data are untouched.
- **Always available**: closing the window only hides it, and the menu bar (macOS) / system tray (Windows) keeps syncing. One reminder appears 3 days before a lottery deadline (`notified.json`); an extended deadline triggers another at the new time.
- **Read-only, no account**: no login, favorites, personal ticket status, or cloud sync. The account entry at the bottom of the sidebar is a placeholder.

| Details and ticket rounds | Data sources |
| --- | --- |
| ![Details panel: date, doors open / show starts, venue, ticket rounds, change log](assets/screenshots/inspector.png) | ![Data sources: grouped by artist, with per-source sync, disable, and delete](assets/screenshots/sources.png) |

## Getting started

### Requirements

| Item | Version |
| --- | --- |
| macOS | 13+ (Apple Silicon or Intel) |
| Windows | 10 / 11 x64 (cross-compiles from macOS / Linux) |
| Go | **1.27 or later** (MyGo v0.5.5 requires Go 1.27) |
| Xcode Command Line Tools | Required to package `.app` / `.dmg` on macOS: `xcode-select --install` |
| NSIS (optional) | Required for the Windows installer: `makensis`. A portable zip is produced without it. |

```bash
brew install go        # or install from https://go.dev/dl/
go version             # should print go1.27.x or later
```

### Run

```bash
go mod download        # first run: fetch MyGo and other dependencies
go tool mygo dev       # build and open the native window; restarts on save
go run .               # or run directly
```

### Test

```bash
go test ./...
go vet ./... && go tool mygo vet .
```

### Package

`scripts/package.sh` reads the version from `mygo.json`, builds with MyGo, and names the artifacts `jpop-live-calendar-{platform}-{version}`:

```bash
scripts/package.sh windows/amd64   # → build/jpop-live-calendar-windows-amd64-1.0.1.zip
scripts/package.sh darwin/arm64    # → build/jpop-live-calendar-darwin-arm64-1.0.1.dmg (or .zip)
scripts/package.sh darwin/universal
```

The Windows zip unpacks to a complete folder; double-click `Live Calendar.exe` inside it. Don't take the exe out on its own — keep the resource files next to it. With NSIS installed, the script also writes `...-setup.exe`. You can also run `go tool mygo build -platform windows/amd64`; artifacts go to `build/windows-amd64/` under MyGo's default names.

## Keyboard shortcuts

Use ⌘ on macOS and Ctrl on Windows:

| Action | macOS | Windows |
| --- | --- | --- |
| Previous / next month (week, day) | ⌘← / ⌘→ | Ctrl+← / Ctrl+→ |
| Back to today | ⌘T | Ctrl+T |
| Month / week / day / list view | ⌘1 – ⌘4 | Ctrl+1 – Ctrl+4 |
| Sync all enabled sources now | ⌘R | Ctrl+R |
| Return from the details panel to the day list | Esc | Esc |
| Show / hide the left sidebar | ⌃⌘S | Ctrl+Shift+S |
| Show / hide the right details panel | ⌥⌘I | Ctrl+Alt+I |
| Clear search | Esc (in the search field) | Esc (in the search field) |

## Data source configuration

Lookup order (first match wins): the `LIVE_CALENDAR_CONFIG` environment variable → the user config directory (macOS: `~/Library/Application Support/Live Calendar/sources.yaml`; Windows: `%AppData%\Live Calendar\sources.yaml`) → `config/sources.yaml` in the current directory → the copy embedded in the app. If the config is invalid, the app still starts with the built-in defaults and shows the error on the Data sources page.

```yaml
sources:
  - id: my-band-live            # unique ID
    name: Example Band official LIVE  # display name
    kind: website               # demo | rss | website | api (Sony Music public JSON)
    layer: live                 # live = LIVE/SCHEDULE (schedule layer); news = NEWS/TICKET (announcement layer)
    url: https://example.com/live/
    enabled: true
    interval: 12h               # auto-sync interval, minimum 10m; 0 = manual sync only
    artist: Example Band        # optional: every show on the page belongs to this artist (official sites)

artists:
  - name: ステラリウム
    aliases: [Stellarium]       # different spellings merge into one artist
    official_url: https://example.com/
    avatar_url: https://example.com/a.jpg  # optional; none = no avatar
    color: "#3975AB"            # optional: force the accent color, overriding the avatar color
    follow: true                # default true: always shown on the Artists page; false = alias merging only

avatars: deezer                 # deezer (default) | off: don't fetch avatars from the network
```

- **Avatars and accent colors**: looked up in the background through the public Deezer API, throttled to one request per second. The accent color is the largest color cluster in the avatar; text contrast stays at or above 4.5:1. When no avatar is found, a default illustration and a fixed palette are used.
- **More filtering**: the `article:` section supports `keywords` / `exclude` / `include` / `title_only` / `max_age` / `body_start` / `body_end` / `max_per_sync` / `delay`; `follow` and `feature` are regexes that discover detail pages and tour special pages from list pages / announcements. See the comments in `config/sources.yaml`.
- **Default coverage**: 米津玄師, Official髭男dism, Mrs. GREEN APPLE, RADWIMPS, back number, 緑黄色社会, YOASOBI, 優里, Aimer, なとり, milet, King Gnu, 藤井風, 星野源, Vaundy, Ado, ZUTOMAYO, ヨルシカ, あたらよ, 『ユイカ』, 美波. 34 sources are enabled by default; 5 backup sources (demo data, two label NEWS feeds, Ado's SPA site, and 『ユイカ』's official site) are off by default.
- **Manage in the app**: add, disable, and delete sources on the Data sources page; comments in the config file are preserved when writing back. Only public, permitted sources are accepted.
- **robots.txt**: RFC 9309 is respected and disallowed paths are not fetched. Ticket links come only from official announcements; ticket platforms are never crawled directly.

## Project layout

```
jpop-live-calendar/
├── main.go                 # entry point: window, menu bar / system tray, background sync
├── mygo.json               # app name, bundle ID, version, output directory
├── resources/              # app icon, tray icons, embedded Pacifico font
├── config/                 # sources.yaml + load / validate / write back
├── internal/
│   ├── domain/             # public data model (shows, ticket rounds, announcements, artists, venues)
│   ├── sources/ crawler/   # data source adapters + polite fetcher / per-source scheduler
│   ├── parser/ normalize/  # Japanese announcement parsing, date / time / prefecture / alias normalization
│   ├── dedup/ catalog/     # dedup and merge, change log, thread-safe in-memory snapshot
│   ├── views/ theme/       # MyGo UI and Spotlight theme, colors, sizes
│   └── avatar/ cache/      # artist avatars / accent colors, local JSON snapshots
├── packages/spotlight-icons/  # icon library (an ordinary package in the main module)
├── scripts/package.sh      # package as jpop-live-calendar-{platform}-{version}
├── assets/screenshots/     # preview images for the README
├── LICENSE                 # MIT license
└── AGENTS.md               # working notes for coding agents (data sources / parsing / UI have their own files)
```

## Icon library

Spotlight Icons is an ordinary package in this project's **main module** — just import it; there is nothing to download or publish. 63 icons on a 24×24 grid, in 6 groups, with a 1.75–2 px stroke width, strictly limited to the three Spotlight colors.

```go
import (
	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"
)

mygoicons.Image(c, icons.Calendar, 18) // keep the original colors (Ink Charcoal + Live Coral)
mygoicons.Mono(c, icons.Calendar, 18)  // monochrome, follows the text color
```

<p align="center">
  <a href="assets/spotlight-icons-overview.png">
    <img src="assets/spotlight-icons-overview.png" width="820" alt="Spotlight Icons overview: 63 icons, 6 groups">
  </a>
</p>

See [packages/spotlight-icons/README.md](packages/spotlight-icons/README.md) for adding or replacing icons, regenerating constants, and running the tests.

## Known limitations

- Parsing is rule-based and covers common Japanese press-release phrasing; anything uncertain is marked "Needs review" — always check the original.
- Data is stored only in local JSON snapshots: macOS `~/Library/Application Support/Live Calendar/`, Windows `%AppData%\Live Calendar\`. Deleting them only makes the next launch re-sync / re-download; the data source config is unaffected.
- The Windows build cross-compiles, but shortcuts, tray, notifications, and title-bar buttons should still be checked by hand on Windows 10 / 11.
- The UI has been verified with screenshots; if something looks wrong on the first run on a Mac, a screenshot is welcome.

## Roadmap (after Phase 5)

- (Optional) Show Chinese / Japanese holidays in calendar cells (e.g. "CN · National Day", "JP · Sports Day").
- Keep improving ticket-round parsing (e.g. dates written on the line after "●エントリー期間").
- Let the add-source form handle `article` filters and `follow` regexes (for now these still require editing `sources.yaml` directly).

## Changelog

- **1.0.1** (2026-10-22)
  - Windows 10 / 11 x64 support: colored tray icon, Ctrl shortcuts, `%AppData%` data directory, and consistently named zips / installers from `scripts/package.sh`.
  - The Artists page groups shows by tour / month, with collapsible tours, a tour filter, and hiding tours whose shows have all ended. Ended shows and ticket rounds fade out, and the list and details scroll independently.
  - Counts move into the toolbar in narrow windows; scrollbars are 4 DIP; dates in announcement lists followed by a "NEW" badge are no longer parsed as shows.
  - Icon library grows to 63 icons (new: `view-disabled`, `map-route`, `map-japan`).
- **1.0.0** — Phase 5: menu bar / system tray presence, lottery deadline reminders, in-app source creation, default avatars, and avatar caching.
- **0.7.0** — Local JSON snapshot cache; a restart no longer starts from an empty calendar.
- **0.6.0** — Data sources rebuilt: schedule / announcement layers, Sony public JSON, tour special pages, and parsing fixes.
- **0.5.0** — Moved to native MyGo controls, a collapsible icon rail for the sidebar, and in-app source deletion.
- **0.4.0** — Liquid Glass panels, collapsible sidebars, artist avatars, and accent colors.
- **Phase 1–3** — Calendar UI, real public data sources, multi-announcement merging, and ticket rounds.
