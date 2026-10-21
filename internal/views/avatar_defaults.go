package views

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/internal/theme"
)

// The three default avatars of an artist without a picture: a guitar, a
// stage and a singer, drawn on the Spotlight palette. Each is recolored
// from the artist's theme color: a light tint for the face and the disc, a
// dark tint for the lines, and the color itself for the instrument.
var (
	//go:embed avatars/artist-guitar.svg
	avatarGuitar []byte
	//go:embed avatars/artist-stage.svg
	avatarStage []byte
	//go:embed avatars/artist-vocal.svg
	avatarVocal []byte

	defaultAvatars = [][]byte{avatarGuitar, avatarStage, avatarVocal}
)

const (
	avatarDisc   = "#FFD9A8" // Warm Spotlight: the disc and the face
	avatarInk    = "#2E2A28" // Ink Charcoal: the lines and the hair
	avatarAccent = "#E85A3A" // Live Coral: the instrument and the clothes
)

// defaultAvatarIndex is the illustration of an artist: one of the three,
// chosen by the artist's ID and the same on every frame.
func defaultAvatarIndex(id string) int {
	h := uint32(2166136261)
	for i := 0; i < len(id); i++ {
		h ^= uint32(id[i])
		h *= 16777619
	}
	return int(h % uint32(len(defaultAvatars)))
}

// defaultAvatarPalette is the three colors of an illustration for an
// artist's theme color: a light tint of it for the disc and the face, a
// dark tint for the lines, and the color itself (darkened until it shows on
// the tint) for the instrument. Text and lines keep at least 4.5:1.
func defaultAvatarPalette(col ui.Color) (disc, ink, accent ui.Color) {
	col.A = 255
	disc = col.Mix(theme.CloudWhite, 0.82)
	ink = col.Mix(theme.InkCharcoal, 0.80)
	accent = col
	for i := 0; i < 10 && theme.Contrast(accent, disc) < 3; i++ {
		accent = accent.Mix(theme.InkCharcoal, 0.18)
	}
	for i := 0; i < 8 && theme.Contrast(ink, disc) < 4.5; i++ {
		ink = ink.Mix(theme.InkCharcoal, 0.2)
	}
	return disc, ink, accent
}

func avatarHex(c ui.Color) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// defaultAvatar returns the illustration of an artist, recolored for their
// theme color and parsed once per illustration and color.
func (a *App) defaultAvatar(id string, col ui.Color) *ui.SVG {
	disc, ink, accent := defaultAvatarPalette(col)
	index := defaultAvatarIndex(id)
	key := fmt.Sprintf("%d:%s:%s:%s", index, avatarHex(disc), avatarHex(ink), avatarHex(accent))
	if s, ok := a.avatarSVGs[key]; ok {
		return s
	}
	src := strings.NewReplacer(
		avatarDisc, avatarHex(disc),
		avatarInk, avatarHex(ink),
		avatarAccent, avatarHex(accent),
	).Replace(string(defaultAvatars[index]))
	s := ui.MustParseSVG([]byte(src))
	a.avatarSVGs[key] = s
	return s
}
