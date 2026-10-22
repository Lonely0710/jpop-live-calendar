package views

import (
	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
)

// endedEntry reports whether a calendar entry is over. A lottery's opening
// marker follows its round, so it stays active while the round still takes
// applications; every other entry is over once its day has passed. The
// calendar works in whole days, so today's entries stay active until the day
// ends.
func (a *App) endedEntry(e domain.Entry) bool {
	switch e.Kind {
	case domain.EntryLotteryOpen, domain.EntryLotteryClose:
		if r, ok := a.snap.Round(e.RoundID); ok {
			return a.endedRound(r)
		}
	}
	return e.Date.Before(a.today())
}

// endedEvent reports whether a show's day is over.
func (a *App) endedEvent(ev domain.PublicEvent) bool { return ev.StartDate.Before(a.today()) }

// endedRound reports whether a ticket round has closed. A round with a
// result out but no stated close is over too; one with neither time stays
// open rather than guessing.
func (a *App) endedRound(r domain.PublicTicketRound) bool {
	now := a.opt.Now().In(theme.DisplayZone)
	switch {
	case r.ClosesAt != nil:
		return !now.Before(r.ClosesAt.Time(theme.DisplayZone))
	case r.ResultAt != nil:
		return !now.Before(r.ResultAt.Time(theme.DisplayZone))
	}
	return false
}

// endedVeil is the gray layer over a finished card: it covers the card's
// own colors without changing its shape, and PassThrough keeps it
// clickable.
func endedVeil(c *ui.Context, radius float32) {
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Radius(radius).
		Background(theme.EndedVeil).PassThrough()
}
