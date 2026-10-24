package views

import (
	"time"

	"github.com/egoist/mygo/i18n"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

// lang is the language the views say their messages in: the one MyGo
// resolved from the system or the user's choice.
func lang() string { return i18n.Locale() }

func weekdayName(w time.Weekday) string {
	names := [7]string{
		locales.WeekdayLongSun(), locales.WeekdayLongMon(), locales.WeekdayLongTue(),
		locales.WeekdayLongWed(), locales.WeekdayLongThu(), locales.WeekdayLongFri(),
		locales.WeekdayLongSat(),
	}
	return names[int(w)%7]
}

// weekdayHeaders returns the column titles from theme.WeekStart.
func weekdayHeaders() []string {
	short := [7]string{
		locales.WeekdayShortSun(), locales.WeekdayShortMon(), locales.WeekdayShortTue(),
		locales.WeekdayShortWed(), locales.WeekdayShortThu(), locales.WeekdayShortFri(),
		locales.WeekdayShortSat(),
	}
	out := make([]string, 7)
	for i := range out {
		out[i] = short[(int(theme.WeekStart)+i)%7]
	}
	return out
}

func dateOf(d domain.Date, skeleton string) string {
	return i18n.FormatDate(lang(), d.Time(theme.DisplayZone), i18n.DateOptions{Skeleton: skeleton})
}

// monthTitle is the month's name with its year. The abbreviated month
// ("Oct 2026") is what English writes; zh and ja are unchanged, and the
// short form is what leaves room for the counts in the toolbar.
func monthTitle(d domain.Date) string { return dateOf(d, "yMMM") }

func dayTitle(d domain.Date) string { return dateOf(d, "yMMMMEEEEd") }

func shortDay(d domain.Date) string { return dateOf(d, "MMMd") }

func weekTitle(d domain.Date) string {
	s := d.WeekStart(theme.WeekStart)
	e := s.AddDays(6)
	end := ""
	switch {
	case s.Year != e.Year:
		end = dateOf(e, "yMMMd")
	case s.Month != e.Month:
		end = dateOf(e, "MMMd")
	default:
		end = dateOf(e, "d")
	}
	return locales.DateRange(dateOf(s, "yMMMd"), end)
}

func (a *App) title() string {
	switch a.section {
	case SectionArtists:
		return locales.NavArtists()
	case SectionSources:
		return locales.NavSources()
	case SectionSettings:
		return locales.NavSettings()
	}
	switch Mode(a.mode) {
	case ModeWeek:
		return weekTitle(a.cursor)
	case ModeDay:
		return dayTitle(a.cursor)
	case ModeAgenda:
		return locales.CalendarFromDate(shortDay(a.cursor))
	}
	return monthTitle(a.cursor)
}

func (a *App) artistNames(ids []string) string {
	var out string
	for _, id := range ids {
		if ar, ok := a.snap.Artist(id); ok {
			if out != "" {
				out += " / "
			}
			out += ar.Name
		}
	}
	return out
}

// momentLabel is "11月3日 周二 23:59", or the catalog's "TBA".
func momentLabel(m *domain.Moment) string {
	if m == nil {
		return locales.CommonTba()
	}
	s := shortDay(m.Date) + " " + weekdayName(m.Date.Weekday())
	if m.Clock != nil {
		s += " " + m.Clock.String()
	}
	return s
}

// stamp formats a publish or fetch time.
func stamp(t time.Time) string {
	if t.IsZero() {
		return locales.CommonUnknown()
	}
	return t.In(theme.DisplayZone).Format("2006-01-02 15:04")
}

// countdown says how far a moment is from now: "还有 3 天", "今天", "已过".
func countdown(m *domain.Moment, now time.Time) string {
	if m == nil {
		return ""
	}
	at := m.Time(theme.DisplayZone)
	if m.Clock == nil {
		at = at.Add(24*time.Hour - time.Second)
	}
	if at.Before(now) {
		return locales.CalendarPast()
	}
	days := domain.DateOf(now, theme.DisplayZone).DaysUntil(m.Date)
	if days == 0 {
		if m.Clock != nil {
			return locales.CalendarCountdownHours(int(at.Sub(now).Hours() + 0.5))
		}
		return locales.CommonToday()
	}
	return locales.CalendarCountdownDays(days)
}

func eventTypeLabel(t domain.EventType) string {
	switch t {
	case domain.TypeFestival:
		return locales.EventTypeFestival()
	case domain.TypeEvent:
		return locales.EventTypeEvent()
	}
	return locales.EventTypeSolo()
}
