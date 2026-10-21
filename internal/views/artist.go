package views

import (
	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/internal/avatar"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
)

// artistColor is an artist's theme color: the largest block of color of
// their avatar, the configured color, or a hue chosen by the name.
func (a *App) artistColor(id, name string) ui.Color {
	if av, ok := a.opt.Avatars.Get(id); ok {
		return ui.RGB(av.Color.R, av.Color.G, av.Color.B)
	}
	if col, ok := a.colors[id]; ok {
		return col
	}
	f := avatar.Fallback(name)
	return ui.RGB(f.R, f.G, f.B)
}

// entryArtist is the artist an entry is drawn for: its first.
func (a *App) entryArtist(e domain.Entry) (id, name string) {
	for _, id := range e.ArtistIDs {
		if ar, ok := a.snap.Artist(id); ok {
			return id, ar.Name
		}
	}
	return "", ""
}

// entryPalette colors an entry by its artist; entries without one keep
// the color of their kind.
func (a *App) entryPalette(e domain.Entry) theme.ArtistPalette {
	if id, name := a.entryArtist(e); id != "" {
		return theme.Artist(a.artistColor(id, name))
	}
	return theme.Artist(theme.KindColor(e.Kind, e.Status))
}

// bitmap is an artist's avatar picture, made into a texture once.
func (a *App) bitmap(id string) *ui.Bitmap {
	if b, ok := a.bitmaps[id]; ok {
		return b
	}
	av, ok := a.opt.Avatars.Get(id)
	if !ok || av.Image == nil {
		return nil
	}
	b := ui.NewBitmap(av.Image)
	a.bitmaps[id] = b
	return b
}

// avatarView is an artist's avatar: the picture in a circle ringed by the
// theme color, or one of the default illustrations in the theme color
// while there is no picture (or it has not arrived yet).
func (a *App) avatarView(c *ui.Context, id, name string, size float32) ui.Element {
	col := a.artistColor(id, name)
	ring := max(1.5, size/22)
	box := ui.Box(c).Size(size, size).Radius(size / 2).Shrink(0).Center().Background(col).
		Role(ui.RoleImage).Label(name + " 的头像")
	if bmp := a.bitmap(id); bmp != nil {
		in := size - 2*ring
		box.Children(func() {
			ui.Box(c).Size(in, in).Radius(in / 2).Clip().Children(func() {
				ui.Image(c, bmp).Fit(ui.Cover).Size(in, in)
			})
		})
		return box
	}
	svg := a.defaultAvatar(id, col)
	box.Children(func() {
		ui.Image(c, svg).Size(size, size)
	})
	return box
}
