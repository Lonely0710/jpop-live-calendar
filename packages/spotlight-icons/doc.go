// Package spotlighticons embeds the Spotlight icon library (24×24 SVG,
// Live Calendar palette) and exposes it by name.
//
// The package itself has no UI dependency: it only hands out SVG bytes, so it
// can be used by any Go program (server-side rendering, tests, other UI kits).
// The subpackage mygoicons adapts the icons to MyGo (github.com/egoist/mygo/ui).
//
// To add or replace icons: drop the SVG files into svg/, list them in
// icons.json, then run `go generate ./...` to refresh names_gen.go.
package spotlighticons

//go:generate go run ./internal/gennames
