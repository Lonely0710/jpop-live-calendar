package views

import (
	"fmt"
	"time"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
)

var weekdayNames = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

func weekdayName(w time.Weekday) string { return weekdayNames[w] }

// weekdayHeaders returns the column titles from theme.WeekStart.
func weekdayHeaders() []string {
	short := [...]string{"日", "一", "二", "三", "四", "五", "六"}
	out := make([]string, 7)
	for i := range out {
		out[i] = short[(int(theme.WeekStart)+i)%7]
	}
	return out
}

func monthTitle(d domain.Date) string { return fmt.Sprintf("%d年%d月", d.Year, int(d.Month)) }

func dayTitle(d domain.Date) string {
	return fmt.Sprintf("%d年%d月%d日 %s", d.Year, int(d.Month), d.Day, weekdayName(d.Weekday()))
}

func shortDay(d domain.Date) string { return fmt.Sprintf("%d月%d日", int(d.Month), d.Day) }

func weekTitle(d domain.Date) string {
	s := d.WeekStart(theme.WeekStart)
	e := s.AddDays(6)
	switch {
	case s.Year != e.Year:
		return fmt.Sprintf("%d年%d月%d日 – %d年%d月%d日", s.Year, int(s.Month), s.Day, e.Year, int(e.Month), e.Day)
	case s.Month != e.Month:
		return fmt.Sprintf("%d年%d月%d日 – %d月%d日", s.Year, int(s.Month), s.Day, int(e.Month), e.Day)
	default:
		return fmt.Sprintf("%d年%d月%d日 – %d日", s.Year, int(s.Month), s.Day, e.Day)
	}
}

func (a *App) title() string {
	switch a.section {
	case SectionArtists:
		return "艺人"
	case SectionSources:
		return "数据源"
	case SectionSettings:
		return "设置"
	}
	switch Mode(a.mode) {
	case ModeWeek:
		return weekTitle(a.cursor)
	case ModeDay:
		return dayTitle(a.cursor)
	case ModeAgenda:
		return "从 " + shortDay(a.cursor) + " 起"
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

// momentLabel is "11月3日 周二 23:59", or "未公布".
func momentLabel(m *domain.Moment) string {
	if m == nil {
		return "未公布"
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
		return "未知"
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
		return "已过"
	}
	days := domain.DateOf(now, theme.DisplayZone).DaysUntil(m.Date)
	if days == 0 {
		if m.Clock != nil {
			return fmt.Sprintf("还有 %.0f 小时", at.Sub(now).Hours())
		}
		return "今天"
	}
	return fmt.Sprintf("还有 %d 天", days)
}

func eventTypeLabel(t domain.EventType) string {
	switch t {
	case domain.TypeFestival:
		return "Fes / 对拼 / 活动出演"
	case domain.TypeEvent:
		return "活动"
	}
	return "单独公演 / 巡演"
}
