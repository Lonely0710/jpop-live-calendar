package domain

import (
	"fmt"
	"time"
)

// Date is a calendar day without a time or zone.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the day t falls on in loc.
func DateOf(t time.Time, loc *time.Location) Date {
	y, m, d := t.In(loc).Date()
	return Date{y, m, d}
}

// Time returns midnight of the day in loc.
func (d Date) Time(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// AddDays returns the day n days later (earlier when negative).
func (d Date) AddDays(n int) Date {
	y, m, dd := time.Date(d.Year, d.Month, d.Day+n, 12, 0, 0, 0, time.UTC).Date()
	return Date{y, m, dd}
}

// AddMonths moves by n months, clamping the day to the target month's length.
func (d Date) AddMonths(n int) Date {
	first := time.Date(d.Year, d.Month+time.Month(n), 1, 12, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	day := d.Day
	if day > last {
		day = last
	}
	return Date{first.Year(), first.Month(), day}
}

// Weekday returns the day of the week.
func (d Date) Weekday() time.Weekday { return d.Time(time.UTC).Weekday() }

// Before reports whether d is earlier than o.
func (d Date) Before(o Date) bool {
	if d.Year != o.Year {
		return d.Year < o.Year
	}
	if d.Month != o.Month {
		return d.Month < o.Month
	}
	return d.Day < o.Day
}

// FirstOfMonth returns the first day of d's month.
func (d Date) FirstOfMonth() Date { return Date{d.Year, d.Month, 1} }

// WeekStart returns the first day of d's week, weeks starting on start.
func (d Date) WeekStart(start time.Weekday) Date {
	off := (int(d.Weekday()) - int(start) + 7) % 7
	return d.AddDays(-off)
}

// MonthGrid returns the 6×7 = 42 days a month view shows for d's month.
func (d Date) MonthGrid(start time.Weekday) [42]Date {
	var out [42]Date
	first := d.FirstOfMonth().WeekStart(start)
	for i := range out {
		out[i] = first.AddDays(i)
	}
	return out
}

func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day) }

// Clock is a wall-clock time in Tokyo. Hours may pass 24 (深夜公演 25:00).
type Clock struct{ Hour, Minute int }

func (c Clock) String() string { return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute) }

// Minutes returns the minutes since midnight.
func (c Clock) Minutes() int { return c.Hour*60 + c.Minute }

// Moment is a day with an optional time. A Moment without Clock is a
// date-only announcement; it is not midnight.
type Moment struct {
	Date  Date
	Clock *Clock
}

// Time returns the instant in loc; a date-only Moment gives the start of
// the day, for ordering only.
func (m Moment) Time(loc *time.Location) time.Time {
	t := m.Date.Time(loc)
	if m.Clock != nil {
		t = t.Add(time.Duration(m.Clock.Minutes()) * time.Minute)
	}
	return t
}

// Equal reports whether both moments say the same.
func (m Moment) Equal(o Moment) bool {
	if m.Date != o.Date || (m.Clock == nil) != (o.Clock == nil) {
		return false
	}
	return m.Clock == nil || *m.Clock == *o.Clock
}

func (m Moment) String() string {
	if m.Clock == nil {
		return m.Date.String()
	}
	return m.Date.String() + " " + m.Clock.String()
}

// After reports whether d is later than o.
func (d Date) After(o Date) bool { return o.Before(d) }

// DaysUntil returns the days from d to o.
func (d Date) DaysUntil(o Date) int {
	return int(o.Time(time.UTC).Sub(d.Time(time.UTC)).Hours() / 24)
}
