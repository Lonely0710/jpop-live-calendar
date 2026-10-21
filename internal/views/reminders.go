package views

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
)

// Reminder is one system notification: a ticket deadline about to pass.
type Reminder struct {
	ID       string // stable per deadline, so a repeated check replaces it
	Title    string
	Subtitle string
	Body     string
	EntryID  string // the calendar entry a click opens
}

// How far ahead deadlines are announced, and how many at a time.
const (
	reminderDays = 3
	reminderMax  = 3
)

// notifyDeadlines shows a system notification for the lottery deadlines
// of the next few days that were not announced yet. It runs on the UI
// thread, and shows them on a background goroutine: asking for the
// notification permission blocks.
func (a *App) notifyDeadlines() {
	if a.opt.Notify == nil || a.notifyBusy || a.notifyOff {
		return
	}
	notes := a.deadlineReminders(a.opt.Now())
	if len(notes) == 0 {
		return
	}
	a.notifyBusy = true
	a.opt.Go(func() {
		shown := make([]Reminder, 0, len(notes))
		var failure error
		for _, n := range notes {
			if err := a.opt.Notify(n); err != nil {
				failure = err
				break
			}
			shown = append(shown, n)
		}
		a.opt.Post(func() {
			a.notifyBusy = false
			if failure != nil {
				// Denied, or no packaged app: do not ask again this run.
				log.Printf("notification: %v", failure)
				a.notifyOff = true
			}
			if len(shown) == 0 {
				return
			}
			at := a.opt.Now().Format(time.RFC3339)
			for _, n := range shown {
				a.notified[n.ID] = at
			}
			// Keep the file small: old deadlines cannot come back.
			cutoff := a.opt.Now().AddDate(0, 0, -90)
			for id, when := range a.notified {
				if t, err := time.Parse(time.RFC3339, when); err != nil || t.Before(cutoff) {
					delete(a.notified, id)
				}
			}
			if a.opt.SaveNotified != nil {
				if err := a.opt.SaveNotified(a.notified); err != nil {
					log.Printf("notified.json: %v", err)
				}
			}
		})
	})
}

// deadlineReminders lists the deadlines of the next days that have not
// been announced, nearest first.
func (a *App) deadlineReminders(now time.Time) []Reminder {
	today := domain.DateOf(now, theme.DisplayZone)
	end := today.AddDays(reminderDays)
	snap := a.opt.Catalog.Snapshot()
	type due struct {
		at time.Time
		r  Reminder
	}
	var dues []due
	for _, e := range snap.Between(today, end) {
		if e.Kind != domain.EntryLotteryClose {
			continue
		}
		at := e.Date.Time(theme.DisplayZone)
		if e.At != nil {
			at = at.Add(time.Duration(e.At.Minutes()) * time.Minute)
		} else {
			// A date-only deadline passes at the end of its day.
			at = at.Add(24*time.Hour - time.Second)
		}
		if at.Before(now) {
			continue
		}
		id := fmt.Sprintf("deadline:%s:%d", e.ID, at.Unix())
		if _, ok := a.notified[id]; ok {
			continue
		}
		label := shortDay(e.Date) + " 截止"
		if e.At != nil {
			label = shortDay(e.Date) + " " + e.At.String() + " 截止"
		}
		dues = append(dues, due{at: at, r: Reminder{
			ID:       id,
			Title:    "抽选截止提醒",
			Subtitle: e.Title,
			Body:     label + " · " + countdown(&domain.Moment{Date: e.Date, Clock: e.At}, now),
			EntryID:  e.ID,
		}})
	}
	sort.Slice(dues, func(i, j int) bool { return dues[i].at.Before(dues[j].at) })
	out := make([]Reminder, 0, reminderMax)
	for _, d := range dues {
		if len(out) == reminderMax {
			break
		}
		out = append(out, d.r)
	}
	return out
}

// EntryIDFromNotification returns the calendar entry a notification opens,
// from the ID its Reminder gave: deadline:<entry>:<unix>.
func EntryIDFromNotification(id string) string {
	rest, ok := strings.CutPrefix(id, "deadline:")
	if !ok {
		return ""
	}
	if i := strings.LastIndexByte(rest, ':'); i > 0 {
		return rest[:i]
	}
	return ""
}

// OpenEntry selects a calendar entry, as clicking its deadline
// notification does.
func (a *App) OpenEntry(id string) {
	if id == "" {
		return
	}
	a.showEntry(id)
}
