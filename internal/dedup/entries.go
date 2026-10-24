package dedup

import (
	"sort"
	"strings"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/locales"
)

// Entries turns shows and ticket rounds into calendar entries: one for each
// show day and one for each dated step of a round (受付开始, 受付截止,
// 结果公布, 一般发售), sorted by date and time.
func Entries(events []domain.PublicEvent, rounds []domain.PublicTicketRound, artists []domain.Artist, sourceNames map[string]string, locale string) []domain.Entry {
	byID := map[string]domain.Artist{}
	for _, a := range artists {
		byID[a.ID] = a
	}
	artistText := func(ids []string) (names, search string) {
		var ns, all []string
		for _, id := range ids {
			if a, ok := byID[id]; ok {
				ns = append(ns, a.Name)
				all = append(all, a.Name)
				all = append(all, a.Aliases...)
			}
		}
		return strings.Join(ns, " / "), strings.Join(all, " ")
	}
	sourceText := func(ids []string) string {
		var out []string
		for _, id := range ids {
			out = append(out, id, sourceNames[id])
		}
		return strings.Join(out, " ")
	}
	eventByID := map[string]domain.PublicEvent{}
	var out []domain.Entry
	for _, e := range events {
		eventByID[e.ID] = e
		_, as := artistText(e.ArtistIDs)
		detail := e.VenueName
		if e.Prefecture != "" && detail != "" {
			detail = e.Prefecture + " · " + detail
		}
		out = append(out, domain.Entry{
			ID:          "live:" + e.ID,
			Kind:        domain.EntryLive,
			Date:        e.StartDate,
			At:          e.StartAt,
			Title:       e.Title,
			Detail:      detail,
			EventID:     e.ID,
			Status:      e.Status,
			Important:   e.Status == domain.StatusPostponed || e.Status == domain.StatusCancelled,
			Changed:     len(e.Changes) > 0,
			NeedsReview: e.NeedsReview,
			SourceIDs:   e.SourceIDs,
			ArtistIDs:   e.ArtistIDs,
			Search: strings.ToLower(strings.Join([]string{e.Title, e.Tour, as, e.VenueName, e.Prefecture,
				sourceText(e.SourceIDs), domain.EntryLive.Label(locale), e.Status.Label(locale), strings.Join(e.Notes, " ")}, " ")),
		})
	}
	for _, r := range rounds {
		names, as := artistText(r.ArtistIDs)
		title := r.Tour
		if title == "" {
			title = names
		}
		if title == "" {
			title = r.Name
		}
		var venues []string
		for _, id := range r.EventIDs {
			if e, ok := eventByID[id]; ok {
				venues = append(venues, e.VenueName, e.Prefecture)
			}
		}
		changed := func(field string) bool {
			for _, c := range r.Changes {
				if c.Field == field {
					return true
				}
			}
			return false
		}
		add := func(kind domain.EntryKind, m *domain.Moment, field string) {
			if m == nil {
				return
			}
			out = append(out, domain.Entry{
				ID:          string(kind) + ":" + r.ID,
				Kind:        kind,
				Date:        m.Date,
				At:          m.Clock,
				Title:       title,
				Detail:      r.Name,
				RoundID:     r.ID,
				Important:   kind == domain.EntryLotteryClose,
				Changed:     changed(string(field)),
				NeedsReview: r.NeedsReview,
				SourceIDs:   r.SourceIDs,
				ArtistIDs:   r.ArtistIDs,
				Search: strings.ToLower(strings.Join([]string{title, r.Tour, r.Name, as, strings.Join(venues, " "),
					sourceText(r.SourceIDs), kind.Label(locale), r.Type.Label(locale)}, " ")),
			})
		}
		open := domain.EntryLotteryOpen
		if !r.Lottery {
			open = domain.EntrySaleOpen
		}
		// The builder names a round's change fields in the language it
		// builds in, so the entry has to look for the same names.
		msgs := locales.In(locale)
		add(open, r.OpensAt, msgs.ChangeFieldOpens())
		add(domain.EntryLotteryClose, r.ClosesAt, msgs.ChangeFieldCloses())
		add(domain.EntryResult, r.ResultAt, msgs.ChangeFieldResults())
	}
	sort.SliceStable(out, func(i, j int) bool { return Less(out[i], out[j]) })
	return out
}

var kindOrder = map[domain.EntryKind]int{}

func init() {
	for i, k := range domain.Kinds {
		kindOrder[k] = i
	}
}

// Less orders entries by date, then all-day before timed, then time, kind
// and title.
func Less(a, b domain.Entry) bool {
	if a.Date != b.Date {
		return a.Date.Before(b.Date)
	}
	if (a.At == nil) != (b.At == nil) {
		return a.At == nil
	}
	if a.At != nil && a.At.Minutes() != b.At.Minutes() {
		return a.At.Minutes() < b.At.Minutes()
	}
	if a.Kind != b.Kind {
		return kindOrder[a.Kind] < kindOrder[b.Kind]
	}
	return a.Title < b.Title
}
