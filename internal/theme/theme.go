// Package theme is the Spotlight design system of Live Calendar: the color
// tokens of the color spec, the MyGo theme built from them, and the layout
// metrics of the macOS window.
package theme

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/internal/domain"
)

// Brand colors (Spotlight color spec).
var (
	PaperIvory    = ui.Hex("#F7F3EA") // 主背景
	InkCharcoal   = ui.Hex("#2E2A28") // 主文字、线条、默认图标
	LiveCoral     = ui.Hex("#E85A3A") // 选中、Live 标记、关键操作（控制在 3–5%）
	WarmSpotlight = ui.Hex("#FFD9A8") // 柔和高亮
	MistGray      = ui.Hex("#D8D1C7") // 分隔线、网格
	SoftAsh       = ui.Hex("#C7C0B6") // 次要、未激活
	CloudWhite    = ui.Hex("#FFFDF8") // 卡片、面板
)

// Status colors.
var (
	StatusSuccess = ui.Hex("#22B05E")
	StatusInfo    = ui.Hex("#3D86F5")
	StatusWarning = ui.Hex("#FF9A1F")
	StatusError   = ui.Hex("#EF4444")
)

// Derived colors. Soft Ash is too light for body text on Paper Ivory, so
// secondary text mixes it into Ink Charcoal to stay readable (≈ 4.5:1).
var (
	TextSecondary = InkCharcoal.Mix(SoftAsh, 0.45)
	TextDisabled  = SoftAsh
	StatNumber    = LiveCoral.Mix(InkCharcoal, 0.3) // 统计数字：与标题区分，且 ≥ 4.5:1
	SidebarBg     = PaperIvory.Mix(MistGray, 0.22)
	Hover         = PaperIvory.Mix(MistGray, 0.45)
	Pressed       = PaperIvory.Mix(MistGray, 0.7)
	SelectedBg    = WarmSpotlight.Alpha(0.55).Over(PaperIvory)
	OutsideMonth  = PaperIvory
)

// Layout metrics in DIPs.
const (
	WindowWidth    = 1280
	WindowHeight   = 820
	MinWidth       = 1100
	MinHeight      = 680
	TitleBarHeight = 52 // the toolbar shares the band with the traffic lights
	TrafficLightX  = 20 // the close button's corner: centered in the band
	TrafficLightY  = 19 // (52 - 14) / 2, 14 being the buttons' height

	SidebarWidth   = 220
	InspectorWidth = 300
	ContentPadding = 20
	Radius         = 8
	CardRadius     = 12
	IconSize       = 18 // toolbar / sidebar; the library is drawn on 24×24
	ToolButton     = 30

	TitleSize   = 18
	BodySize    = 13
	CaptionSize = 11.5
	HourHeight  = 44 // week / day views
	TimeGutter  = 76
)

// WeekStart is the first column of the month and week views.
const WeekStart = time.Monday

// DisplayZone is the zone every time is shown in.
var DisplayZone = domain.Tokyo

// Spotlight returns the MyGo theme of the app.
func Spotlight() *ui.Theme {
	t := *ui.LightTheme()
	t.Dark = false
	t.Background = PaperIvory
	t.Surface = CloudWhite
	// MyGo's own hover, as the sidebar's items: the same light wash of ink
	// the glass controls use, so a row of the sidebar does not darken more
	// than the glass around it. The opaque cards keep theme.Hover.
	t.SurfaceHover = GlassHover
	t.SurfacePressed = Pressed
	t.Border = MistGray
	t.Text = InkCharcoal
	t.TextMuted = TextSecondary
	t.Accent = LiveCoral
	t.AccentHover = LiveCoral.Mix(InkCharcoal, 0.08)
	t.AccentPressed = LiveCoral.Mix(InkCharcoal, 0.16)
	t.AccentText = CloudWhite
	t.Danger = StatusError
	t.Warning = StatusWarning
	t.Success = StatusSuccess
	t.Selection = WarmSpotlight
	t.Focus = LiveCoral.Alpha(0.6)
	t.Inverse = InkCharcoal
	t.InverseText = CloudWhite
	t.Scrollbar = SoftAsh
	// MyGo's 6 DIP bars read heavy against the app's 1 DIP rules.
	t.ScrollbarWidth = 4
	t.Radius = Radius
	// The sidebar's item metrics and the icon the app repaints over a
	// focused item assume MyGo's 4 DIP spacing unit.
	t.Spacing = 4
	t.FontSize = BodySize
	return &t
}

// KindColor is the marker color of a calendar entry. Postponed and
// cancelled shows use the error color whatever their kind.
func KindColor(k domain.EntryKind, st domain.Status) ui.Color {
	switch st {
	case domain.StatusPostponed, domain.StatusCancelled:
		return StatusError
	}
	switch k {
	case domain.EntryLive:
		return LiveCoral
	case domain.EntryLotteryOpen, domain.EntryLotteryClose:
		return StatusWarning
	case domain.EntryResult:
		return StatusInfo
	case domain.EntrySaleOpen:
		return StatusSuccess
	default:
		return SoftAsh
	}
}

// Legend lists the markers the calendar explains, in order.
var Legend = []struct {
	Label string
	Color ui.Color
}{
	{"Live", LiveCoral},
	{"抽选受付", StatusWarning},
	{"结果公布", StatusInfo},
	{"发售", StatusSuccess},
	{"延期/中止", StatusError},
}

// Liquid Glass layout: the sidebar and the inspector are panes of glass
// floating over a warm backdrop, inset from the window's edges; the
// calendar sits on a Cloud White sheet between them.
const (
	PaneInset    = 8  // gap between a pane and the window's edge
	PaneGap      = 6  // gap between the panes and the sheet
	RailWidth    = 58 // the collapsed sidebar: a rail of icons
	NavInset     = 13 // the sidebar's lists from its edges: icons centered in the rail
	PaneRadius   = 18 // the panes' corners, close to macOS 26's sidebar
	SheetRadius  = 14 // the content sheet's corners
	CapsuleH     = 32 // toolbar capsules
	ToggleButton = 28
)

// The backdrop the glass shows: Paper Ivory warmed by two soft glows, so
// the frosted panes read as glass rather than flat gray.
var (
	BackdropTop    = ui.Hex("#F6EEDF")
	BackdropBottom = ui.Hex("#EFE3CF")
	GlowWarm       = WarmSpotlight.Alpha(0.75)
	GlowCoral      = LiveCoral.Alpha(0.22)
	GlassText      = InkCharcoal
	SheetShadow    = InkCharcoal.Alpha(0.10)
	TodayTint      = LiveCoral.Alpha(0.07).Over(CloudWhite)
	GlassHover     = InkCharcoal.Alpha(0.07)
	GlassPressed   = InkCharcoal.Alpha(0.12)
	GlassSelected  = WarmSpotlight.Alpha(0.7)
	GlassLine      = InkCharcoal.Alpha(0.08)
	// EndedVeil is the light gray laid over a show or round that is over:
	// it washes the card's colors out, as an inactive layer instead of a
	// darker one.
	EndedVeil = SoftAsh.Alpha(0.55)
)

// ArtistPalette is how an artist's theme color shows on the calendar.
type ArtistPalette struct {
	Base   ui.Color // the color itself (avatar ring, legend)
	Accent ui.Color // the chip's bar: the color, darkened if too light to see
	Tint   ui.Color // the chip's background
	Hover  ui.Color // the chip's background under the pointer
	Strong ui.Color // the chip's background when selected
	Text   ui.Color // the artist's name on Tint, at least 4.5:1
	OnBase ui.Color // text on Base (initials of an avatar without picture)
}

// Artist returns the palette of a theme color.
func Artist(col ui.Color) ArtistPalette {
	col.A = 255
	p := ArtistPalette{Base: col}
	p.Accent = col
	for i := 0; i < 8 && Luminance(p.Accent) > 0.45; i++ {
		p.Accent = p.Accent.Mix(InkCharcoal, 0.15)
	}
	p.Tint = col.Alpha(0.16).Over(CloudWhite)
	p.Hover = col.Alpha(0.26).Over(CloudWhite)
	p.Strong = col.Alpha(0.38).Over(CloudWhite)
	p.Text = p.Accent
	for i := 0; i < 10 && Contrast(p.Text, p.Strong) < 4.5; i++ {
		p.Text = p.Text.Mix(InkCharcoal, 0.2)
	}
	p.OnBase = CloudWhite
	if Contrast(CloudWhite, col) < 3 {
		p.OnBase = InkCharcoal
	}
	return p
}

// Luminance is a color's relative luminance (WCAG 2).
func Luminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Contrast is the WCAG contrast ratio of two colors.
func Contrast(a, b ui.Color) float64 {
	la, lb := Luminance(a), Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
