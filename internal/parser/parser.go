// Package parser extracts shows and ticket rounds from the text of a
// Japanese live announcement.
//
// It reads what the text states and nothing more: a show without a time
// keeps no time, a round without a deadline keeps no deadline, and
// anything ambiguous is reported in Notes for review instead of guessed.
// It understands the common layout of official and news announcements:
//
//	▼ツアー情報
//	"Example TOUR 2027"
//	2月4日(木) 広島 広島LIVE VANQUISH
//	OPEN 17:30 / START 18:30
//	▼チケット情報
//	【オフィシャル最速先行(抽選)】
//	受付期間:10月20日(火)12:00~11月1日(日)23:59
//	当落発表:11月5日(木)18:00
package parser

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/normalize"
)

// Input is one announcement to read.
type Input struct {
	Title       string
	Text        string
	ArtistHints []string
	Published   time.Time
	// Schedule reads the text as an artist's own live schedule: every
	// dated line is a show, and the lines under it give its time and
	// venue (see scheduleLines).
	Schedule bool
}

// Event is a show the text states.
type Event struct {
	Tour       string
	Date       domain.Date
	OpenAt     *domain.Clock
	StartAt    *domain.Clock
	Venue      string
	Prefecture string
	Type       domain.EventType
	Status     domain.Status // "" when the text says nothing
	Notes      []string      // needs review
	Info       []string      // stated facts worth showing, as 振替公演
	TicketURL  string        // a sales page given with the show
	Line       string
}

// Round is a ticket round the text states.
type Round struct {
	Name     string
	Type     domain.RoundType
	Lottery  bool
	OpensAt  *domain.Moment
	ClosesAt *domain.Moment
	ResultAt *domain.Moment
	URL      string
	Notes    []string
}

// Result is what one announcement says.
type Result struct {
	Events        []Event
	Rounds        []Round
	Tour          string
	Artists       []string
	ArtistGuessed bool
	Status        domain.Status // stated in the title
	Notes         []string
}

type section int

const (
	secNone section = iota
	secLive
	secTicket
	secOther
)

var (
	plainHeadRe   = regexp.MustCompile(`^(?:公演|開催)?(?:日程|日時)\s*(?:[・/／&＆]\s*(?:会場|場所))?\s*[:：]?$|^(?i:schedule|date\s*[&/]\s*venue)$`)
	headerRe      = regexp.MustCompile(`^(?:[▼■◆●★☆□◇]+\s*(.+)|<([^>]{1,30})>|\[([^\]]{1,30})\]|【([^】]{1,30})】)$`)
	roundTagRe    = regexp.MustCompile(`^[■◆●・*\-]*\s*【([^】]{1,40})】\s*(.*)$`)
	fieldRe       = regexp.MustCompile(`^[^:]{1,20}:\s*\S`)
	roundAngleRe  = regexp.MustCompile(`^<([^<>]{1,40})>\s*(.*)$`)
	roundKVRe     = regexp.MustCompile(`^[■◆●・*\-]*\s*([^:]{0,40}?(?:先行|一般発売|一般販売|先着販売|抽選販売|抽選受付|プレリザーブ|プレオーダー))\s*(?:受付|日)?\s*:\s*(.*)$`)
	eventRe       = regexp.MustCompile(`^(?:(?:日程|日時|公演日時|公演日|開催日時|開催日|振替公演|振替日|DATE|Date)\s*:\s*)?[・■◆●*\-]?\s*(?:(\d{4})\s*[年./-]\s*)?(\d{1,2})\s*[月/.]\s*(\d{1,2})\s*日?\s*(\([^)]{1,8}\))?\s*(.*)$`)
	extraDayRe    = regexp.MustCompile(`^(?:[・,、&]\s*(\d{1,2})\s*日?\s*(\([^)]{1,8}\))?|\s*(\d{1,2})\s*日?\s*(\((?i:[月火水木金土日]|mon|tue|wed|thu|fri|sat|sun)[^)]{0,6}\)))\s*`)
	openRe        = regexp.MustCompile(`(?i)(?:OPEN|開場)\s*:?\s*(\d{1,2}\s*(?::|時)\s*\d{0,2})\s*分?`)
	startRe       = regexp.MustCompile(`(?i)(?:START|開演|スタート)\s*:?\s*(\d{1,2}\s*(?::|時)\s*\d{0,2})\s*分?`)
	bareTimeRe    = regexp.MustCompile(`(\d{1,2}:\d{2})\s*(?:~|開演|スタート)?`)
	emptyAngleRe  = regexp.MustCompile(`<[\s/~-]*>`)
	bracketRe     = regexp.MustCompile(`【[^】]*】|\[[^\]]*\]|※.*$`)
	urlRe         = regexp.MustCompile(`https?://[^\s"'<>)）]+`)
	momentRe      = regexp.MustCompile(`(?:(\d{4})\s*[年./-]\s*)?(\d{1,2})\s*[月/.]\s*(\d{1,2})\s*日?\s*(\([^)]{1,8}\))?\s*(?:(\d{1,2})\s*(?::|時)\s*(\d{2})?\s*分?)?`)
	dayOnlyRe     = regexp.MustCompile(`^\s*(\d{1,2})\s*日\s*(\([^)]{1,8}\))?\s*(?:(\d{1,2})\s*(?::|時)\s*(\d{2})?)?`)
	timeOnlyRe    = regexp.MustCompile(`^\s*(\d{1,2})\s*(?::|時)\s*(\d{2})?`)
	quoteRe       = regexp.MustCompile(`^(?:「([^「」]+)」|『([^『』]+)』|“([^“”]+)”|"([^"]+)")$`)
	venueHeadRe   = regexp.MustCompile(`^[<＜〈]\s*([^<>＜＞〈〉]{2,5})\s*[>＞〉]\s*(\S.{1,60})$`)
	postStampRe   = regexp.MustCompile(`^\d{4}\.\d{1,2}\.\d{1,2}\s+\d{1,2}:\d{2}$`)
	weekdayLineRe = regexp.MustCompile(`^[(（]?(?:(?i:sun|mon|tue|wed|thu|fri|sat)[a-z]*\.?|[月火水木金土日](?:[・/][祝休])?)[)）]?$`)
	kagiRe        = regexp.MustCompile(`「([^「」]{3,80})」`)
	tourWordRe    = regexp.MustCompile(`(?i)tour|live|ツアー|ライブ|ライヴ|ワンマン|公演|fes`)
	titleQRe      = regexp.MustCompile(`["“『]([^"”』]{3,80})["”』]`)
	guestRe       = regexp.MustCompile(`ゲスト(?:として|で)?出演`)
)

func (p *state) curTour() string {
	if p.tour != "" {
		return p.tour
	}
	return p.postTour
}

// bracketTour is the name in 「」 of a post's title, if any.
func bracketTour(l string) string {
	if m := kagiRe.FindStringSubmatch(l); m != nil && !momentRe.MatchString(m[1]) {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// guestElsewhere reports whether a schedule page's title announces the
// artist (or one of its members) as a guest at someone else's show, whose
// dates and tour name are the host's, not the artist's.
func guestElsewhere(title string, hints []string) bool {
	if !guestRe.MatchString(title) {
		return false
	}
	for _, h := range hints {
		if h != "" && strings.Contains(title, h) {
			return false // the artist itself is the guest: still its appearance
		}
	}
	return true
}

// tourIn reads a tour's name from a title: the whole name in 「」, as
// "「Mrs. GREEN APPLE Ringo Jam Tour “SHADOWS”」香川公演"; the words of a
// tour before its name, as "なとり ONE-MAN LIVE TOUR「行進」 愛知県芸術劇場…"
// or "RADWIMPS Official Fan Club Tour 2026 “ユーリンチー”"; or a quoted
// name that is a tour's, as "“milet Asia Tour 2027”".
func tourIn(t string) string {
	if m := kagiRe.FindStringSubmatch(t); m != nil && tourWordRe.MatchString(m[1]) {
		return strings.TrimSpace(m[1])
	}
	if loc := kagiTourRe.FindStringSubmatchIndex(t); loc != nil {
		from := loc[2]
		if before := t[:from]; strings.HasSuffix(before, "』") {
			if i := strings.LastIndex(before, "『"); i >= 0 {
				from = i // "『ユイカ』TOUR「Anemone」"
			}
		}
		if s := trimLead(t[from:loc[3]]); len([]rune(s)) <= 80 && !momentRe.MatchString(s) {
			return s
		}
	}
	if m := quotedTourRe.FindStringSubmatch(t); m != nil {
		if q := strings.Trim(m[2], `"“”『』`); tourWordRe.MatchString(q) {
			return q
		}
		if s := trimLead(m[1]); len([]rune(s)) <= 60 && !momentRe.MatchString(s) {
			return s + " " + m[2]
		}
	}
	if m := labelTailRe.FindStringSubmatch(t); m != nil && len([]rune(m[1])) >= 4 {
		return strings.TrimSpace(m[1]) // "one room party vol.8【山形】（＊FC会員限定ライブ）"
	}
	return ""
}

// proseTitle reports whether a title is a sentence about a tour, as
// "…アジアツアーが開催決定!", not the tour's name.
func proseTitle(t string) bool {
	return proseWordRe.MatchString(t) || strings.ContainsAny(t, "。!！")
}

// trimLead drops a label and a date before a tour's name: "【SOLD OUT】",
// "6月27(土)、28日(日)".
func trimLead(s string) string {
	s = leadLabelRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

var (
	labelTailRe  = regexp.MustCompile(`^([^【】「」『』]+?)\s*【[^】]{1,8}】\s*(?:[(（][^)）]*[)）])?$`)
	kagiTourRe   = regexp.MustCompile(`(?i)((?:[^\s、,!！()（）「」『』]+\s+)*[^\s、,!！()（）「」『』]*(?:tour|live|ツアー|ライブ|ライヴ|ワンマン)[^\s「」『』]*(?:\s+[^\s「」『』]+)*?\s*「[^「」]{1,40}」)`)
	quotedTourRe = regexp.MustCompile(`(?i)^([^「」『』“”"]*?(?:tour|live|ツアー|ライブ|ライヴ|ワンマン)[^「」『』“”"]*?)\s*(["“『][^"”』]{2,60}["”』])`)
	leadLabelRe  = regexp.MustCompile(`^(?:\s*【[^】]*】)+|^\d{1,2}\s*[/月][\d/月日()（）火水木金土祝・、,\s]*`)
)

// tourLineIn finds a line naming the tour in a page whose title does not,
// as "Vaundy ASIA ARENA TOUR 2026 "HORO"" under a show titled by its venue:
// a short line with a tour word and a year or quotes, no date and no prose.
func tourLineIn(lines []string) string {
	for _, l := range lines {
		if i := strings.IndexAny(l, "|｜"); i > 0 {
			l = strings.TrimSpace(l[:i]) // a page's title: "… TOUR 2026 | Site"
		}
		if isNote(l) {
			continue
		}
		n := len([]rune(l))
		if n < 8 || n > 80 || !tourWordRe.MatchString(l) || urlRe.MatchString(l) || momentRe.MatchString(l) {
			continue
		}
		if !yearRe.MatchString(l) && !strings.ContainsAny(l, "「“\"『") {
			continue
		}
		if strings.ContainsAny(l, ":：。!！?？") || proseWordRe.MatchString(l) {
			continue
		}
		if t := tourIn(l); t != "" {
			return t
		}
		return strings.TrimSpace(l)
	}
	return ""
}

var (
	yearRe      = regexp.MustCompile(`20\d\d`)
	proseWordRe = regexp.MustCompile(`(?i)please|apply|詳細|チケット|受付|先行|販売|発売|配信|お知らせ|決定|開催|ご案内|について|グッズ|GOODS|STREAMING|ARCHIVE`)
)

// Parse reads one announcement.
func Parse(in Input) Result {
	title := normalize.Text(in.Title)
	text := normalize.Text(in.Text)
	ref := in.Published
	if ref.IsZero() {
		ref = time.Now()
	}
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	p := &state{ref: ref, res: Result{Status: statusIn(title)}, schedule: in.Schedule}
	if in.Schedule && guestElsewhere(title, in.ArtistHints) {
		// A member's guest appearance at another artist's show: the dates
		// on the page belong to the host's tour, not to this artist.
		return Result{Artists: clean(in.ArtistHints)}
	}
	if in.Schedule {
		lines = scheduleLines(lines)
		p.sec, p.secType = secLive, domain.TypeLive
	}
	hasHeaders := in.Schedule
	for _, l := range lines {
		if _, ok := header(l); ok {
			hasHeaders = true
			break
		}
	}
	if !hasHeaders && liveWords.MatchString(title) {
		p.sec = secLive
		p.prose = true
	}
	for _, l := range lines {
		p.line(l)
	}
	p.flushRound()
	if p.schedule {
		p.dropBareDates()
	}
	p.dedupEvents()

	res := &p.res
	p.roundsBeforeShows()
	if res.Tour == "" && len(res.Events) > 0 {
		res.Tour = tourIn(title)
		if res.Tour == "" && (!tourWordRe.MatchString(title) || proseTitle(title)) {
			res.Tour = tourLineIn(lines)
		}
		if res.Tour == "" {
			if m := titleQRe.FindStringSubmatch(title); m != nil {
				res.Tour = m[1]
			}
		}
	}
	for i := range res.Events {
		if res.Events[i].Tour == "" {
			res.Events[i].Tour = res.Tour
		}
	}
	switch {
	case len(in.ArtistHints) > 0:
		res.Artists = clean(in.ArtistHints)
	case len(p.performers) > 0:
		res.Artists = p.performers
	default:
		if a := guessArtist(title); a != "" {
			res.Artists = []string{a}
			res.ArtistGuessed = true
		}
	}
	if res.Status != "" {
		for i := range res.Events {
			e := &res.Events[i]
			if e.Status == "" && !strings.Contains(e.Line, "振替") {
				e.Status = res.Status
			}
		}
	}
	if p.prose && len(res.Events) > 0 {
		res.Notes = append(res.Notes, "公告没有结构化的日程段落，日期取自正文，需确认")
	}
	return *res
}

type state struct {
	ref        time.Time
	sec        section
	secType    domain.EventType
	prose      bool
	tour       string
	block      []int // events since the last time line
	round      *Round
	performers []string
	res        Result

	year      int // the year of a "2026年" line above, for the dates under it
	yearMonth int // the month of the first date under it

	schedule  bool
	postTour  string    // the tour of a schedule page's post, from its title
	venueHead [2]string // prefecture and venue of a "<大阪>NHK大阪ホール" over its dates
	stamped   bool      // the last line was a post's "2026.10.02 13:00"
	last      []int     // events of the last dated line (schedule)
	since     int       // lines read since it
}

var (
	liveWords      = regexp.MustCompile(`(?i)ライヴ|ライブ|LIVE|ツアー|TOUR|公演|ワンマン|フェス|FES|イベント|EVENT|出演|開催|日程|スケジュール|SCHEDULE`)
	ticketWords    = regexp.MustCompile(`(?i)チケット|TICKET|先行|抽選|受付|一般発売|一般販売|先着|プレイガイド|当落|発売開始`)
	otherWords     = regexp.MustCompile(`(?i)リリース|RELEASE|配信|CD|DVD|Blu-ray|グッズ|GOODS|特典|放送|配布|書籍|プロフィール`)
	ticketURLWords = regexp.MustCompile(`(?i)チケット|TICKET|LivePocket|ぴあ|イープラス|e\+|ローチケ|ローソン|ZAIKO|tiget|teket|購入|予約`)
	festWords      = regexp.MustCompile(`(?i)フェス|FES|対バン|イベント|EVENT`)
)

func header(l string) (string, bool) {
	if plainHeadRe.MatchString(l) {
		return "日程", true // "日程・会場" on a line of its own
	}
	m := headerRe.FindStringSubmatch(l)
	if m == nil {
		return "", false
	}
	for _, g := range m[1:] {
		if g != "" {
			return g, true
		}
	}
	return "", false
}

func (p *state) line(l string) {
	if p.schedule {
		// A page of posts: "2026.10.02 13:00" over the post's title, as
		// "美波「Timesick Traveler 2027」国内・アジアツアーの開催が決定".
		if p.stamped {
			p.postTour = bracketTour(l)
		}
		p.stamped = postStampRe.MatchString(l)
	}
	if h, ok := header(l); ok && p.round != nil && fieldRe.MatchString(h) && p.roundLine(h) {
		return // "■受付期間：9月21日(月・祝)21:30〜" under a round's name
	}
	if h, ok := header(l); ok && !((strings.HasPrefix(l, "【") || strings.HasPrefix(l, "<")) && isRoundName(h)) {
		p.flushRound()
		p.tour, p.block, p.prose = "", nil, false
		p.venueHead = [2]string{}
		switch {
		case ticketWords.MatchString(h) && !otherWords.MatchString(h):
			p.sec = secTicket
		case liveWords.MatchString(h):
			p.sec = secLive
			p.secType = domain.TypeLive
			if festWords.MatchString(h) {
				p.secType = domain.TypeFestival
			}
		default:
			p.sec = secOther
		}
		return
	}
	if p.sec == secOther && venueHeadRe.MatchString(l) {
		// "<埼玉>大宮ソニックシティ" after "<『ユイカ』コメント>": the shows again.
		if m := venueHeadRe.FindStringSubmatch(l); normalize.Prefecture(m[1]) != "" {
			p.sec, p.secType = secLive, domain.TypeLive
		}
	}
	if p.sec == secOther {
		return
	}
	if m := venueHeadRe.FindStringSubmatch(l); m != nil {
		if pref := normalize.Prefecture(m[1]); pref != "" && venueLike(m[2]) {
			p.flushRound() // a venue ends the round above it
			p.venueHead = [2]string{pref, strings.TrimSpace(m[2])}
			p.block = nil
			return
		}
	}
	if m := yearLineRe.FindStringSubmatch(l); m != nil {
		p.year, p.yearMonth = normalize.Atoi(m[1]), 0 // "2026年" over the dates
		return
	}
	if name, rest, ok := roundStart(l); ok {
		p.flushRound()
		p.round = newRound(name)
		if rest != "" {
			p.roundLine(rest)
		}
		return
	}
	if bareRoundName(l) {
		// A round's name on a line of its own, its dates below:
		// "ゆーりんち限定最速先行(抽選)" over "受付期間:7月25日(土)21:00~".
		p.flushRound()
		p.round = newRound(l)
		return
	}
	if p.round != nil && p.roundLine(l) {
		return
	}
	if p.sec == secTicket || (p.sec == secNone && ticketWords.MatchString(l)) {
		// A ticket line outside a round, as "一般発売日:12月5日(土)10:00~".
		if strings.Contains(l, "一般") && strings.Contains(l, "発売") {
			p.flushRound()
			p.round = newRound("一般発売")
			p.roundLine(l)
		}
		return
	}
	if p.sec != secLive {
		return
	}
	if m := quoteRe.FindStringSubmatch(l); m != nil {
		// A whole line in one pair of quotes: "「…」北海道公演 -… 「Ringo Jam」"
		// opens and closes with quotes, but of two pairs.
		p.tour = strings.Join(m[1:], "")
		if p.res.Tour == "" {
			p.res.Tour = p.tour
		}
		p.block = nil
		return
	}
	if names, ok := performers(l); ok {
		p.performers = appendUnique(p.performers, names...)
		return
	}
	if evs, ok := p.eventLine(l); ok {
		p.last, p.since = nil, 0
		for _, e := range evs {
			p.last = append(p.last, len(p.res.Events))
			p.res.Events = append(p.res.Events, e)
			if e.StartAt == nil {
				p.block = append(p.block, len(p.res.Events)-1)
			}
		}
		return
	}
	if p.schedule && p.continuation(l) {
		return
	}
	if p.prose {
		if evs := p.proseEvents(l); len(evs) > 0 {
			p.res.Events = append(p.res.Events, evs...)
			return
		}
	}
	if open, start, ok := times(l); ok && len(p.block) > 0 {
		for _, i := range p.block {
			p.res.Events[i].OpenAt, p.res.Events[i].StartAt = open, start
		}
		p.block = nil
		return
	}
	if u := urlRe.FindString(l); u != "" && len(p.res.Events) > 0 && ticketURLWords.MatchString(l) {
		for i := len(p.res.Events) - 1; i >= 0 && p.res.Events[i].TicketURL == ""; i-- {
			if p.res.Events[i].Tour != p.tour {
				break
			}
			p.res.Events[i].TicketURL = u
		}
		return
	}
	if v, ok := strings.CutPrefix(l, "会場:"); ok && len(p.res.Events) > 0 {
		last := p.last
		if len(last) == 0 {
			last = []int{len(p.res.Events) - 1}
		}
		pref, venue := normalize.SplitPrefecture(strings.TrimSpace(v))
		for _, i := range last {
			if i < len(p.res.Events) && p.res.Events[i].Venue == "" {
				p.res.Events[i].setVenue(pref, venue)
			}
		}
	}
}

func (e *Event) setVenue(pref, venue string) {
	if pref != "" && e.Prefecture == "" {
		e.Prefecture = pref
	}
	venue = venueURLRe.ReplaceAllString(venue, "") // "GLION ARENA KOBE(https://www.totteikobe.jp/)"
	venue = strings.Join(strings.Fields(strings.Trim(venue, "/|｜ ・…")), " ")
	if venue == "" {
		return
	}
	e.Venue = venue
	var notes []string
	for _, n := range e.Notes {
		if n != "场地未公布" {
			notes = append(notes, n)
		}
	}
	e.Notes = notes
}

func (p *state) eventLine(l string) ([]Event, bool) {
	m := eventRe.FindStringSubmatch(l)
	if m == nil {
		return nil, false
	}
	month, day := normalize.Atoi(m[2]), normalize.Atoi(m[3])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return nil, false
	}
	rest := m[5]
	if p.schedule && (strings.HasPrefix(rest, "現在") || strings.HasPrefix(rest, "時点")) {
		return nil, false // "2026年3月3日現在の…": a page's date, not a show
	}
	if p.schedule && rangeRe.MatchString(rest) {
		return nil, false // "2027.01.15(Fri)~2027.05.23(Sun)": a tour's period, not a show
	}
	if ticketWords.MatchString(rest) && !liveWords.MatchString(rest) {
		return nil, false // "2/4(木)より受付開始" is not a show
	}
	type md struct {
		day int
		wd  string
	}
	days := []md{{day, m[4]}}
	for {
		x := extraDayRe.FindStringSubmatch(rest)
		if x == nil {
			break
		}
		if x[3] != "" { // "2/12(水) 13(木)": the weekday tells it is a day
			x[1], x[2] = x[3], x[4]
		}
		days = append(days, md{normalize.Atoi(x[1]), x[2]})
		rest = rest[len(x[0]):]
	}
	base := Event{Line: l, Type: p.secType}
	if strings.Contains(l, "振替") {
		base.Info = append(base.Info, "振替公演")
	}
	if base.Type == "" {
		base.Type = domain.TypeLive
	}
	base.Status = statusIn(rest)
	base.OpenAt, base.StartAt, _ = times(rest)
	if base.StartAt == nil && base.OpenAt == nil {
		if t := bareTimeRe.FindStringSubmatch(rest); t != nil {
			if c, ok := normalize.ParseClock(t[1]); ok {
				base.StartAt = &c
			}
		}
	}
	venue := timeWordRe.ReplaceAllString(rest, "")
	venue = openRe.ReplaceAllString(venue, "")
	venue = startRe.ReplaceAllString(venue, "")
	venue = bareTimeRe.ReplaceAllString(venue, "")
	venue = bracketRe.ReplaceAllString(venue, "")
	venue = emptyAngleRe.ReplaceAllString(venue, "") // "<Open 16:30 / Start 18:30>" once the times are out
	venue = strings.Trim(strings.TrimSpace(venue), "/~ :")
	for _, suf := range []string{"公演", "にて開催", "にて"} {
		venue = strings.TrimSpace(strings.TrimSuffix(venue, suf))
	}
	if p.schedule && venue != "" && !venueLike(strings.Trim(venue, "/ ")) {
		venue = ""
	}
	pref, v := normalize.SplitPrefecture(venue)
	if v == "" && p.venueHead[1] != "" {
		pref, v = p.venueHead[0], p.venueHead[1] // the "<大阪>NHK大阪ホール" above
	}
	base.setVenue(pref, v)
	if base.Venue == "" {
		base.Notes = append(base.Notes, "场地未公布")
	}
	base.Tour = p.curTour()
	var out []Event
	for _, d := range days {
		e := base
		e.Notes = append([]string(nil), base.Notes...)
		e.Info = append([]string(nil), base.Info...)
		var y int
		if m[1] != "" {
			y = normalize.Atoi(m[1])
			p.year, p.yearMonth = y, month
		} else if p.year != 0 {
			y = p.underYear(time.Month(month), d.day, d.wd)
		} else {
			var wd *time.Weekday
			if w, ok := normalize.Weekday(d.wd); ok {
				wd = &w
			}
			var ok bool
			y, ok = normalize.InferYear(time.Month(month), d.day, wd, p.ref)
			if !ok {
				e.Notes = append(e.Notes, "星期与日期不一致，需确认")
			}
		}
		e.Date = domain.Date{Year: y, Month: time.Month(month), Day: d.day}
		out = append(out, e)
	}
	return out, true
}

// underYear is the year of a date written under a "2026年" line: that
// year, or the next once the months wrap around (a tour from November to
// February), as the weekday tells when given.
func (p *state) underYear(month time.Month, day int, wd string) int {
	if p.yearMonth == 0 {
		p.yearMonth = int(month)
	}
	y := p.year
	if int(month) < p.yearMonth {
		y++
	}
	if w, ok := normalize.Weekday(wd); ok {
		for _, c := range []int{y, p.year, p.year + 1} {
			if (domain.Date{Year: c, Month: month, Day: day}).Weekday() == w {
				return c
			}
		}
	}
	return y
}

var (
	proseDateRe  = regexp.MustCompile(`(?:(\d{4})年)?(\d{1,2})月(\d{1,2})日\s*(\([^)]{1,8}\))?`)
	proseVenueRe = regexp.MustCompile(`([^、。,]{2,30}?)(?:にて|で開催|で行われる)`)
)

// proseEvents reads shows from a sentence such as "12月12日(土)に渋谷WWWにて
// ワンマンを開催する". Everything it returns is marked for review.
func (p *state) proseEvents(l string) []Event {
	if !liveWords.MatchString(l) || otherWords.MatchString(l) {
		return nil
	}
	var out []Event
	for _, m := range proseDateRe.FindAllStringSubmatchIndex(l, -1) {
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return l[m[2*i]:m[2*i+1]]
		}
		month, day := normalize.Atoi(g(2)), normalize.Atoi(g(3))
		if month < 1 || month > 12 || day < 1 || day > 31 {
			continue
		}
		e := Event{Line: l, Type: domain.TypeLive, Tour: p.curTour()}
		y := normalize.Atoi(g(1))
		if y == 0 {
			var wd *time.Weekday
			if w, ok := normalize.Weekday(g(4)); ok {
				wd = &w
			}
			y, _ = normalize.InferYear(time.Month(month), day, wd, p.ref)
		}
		e.Date = domain.Date{Year: y, Month: time.Month(month), Day: day}
		rest := strings.TrimPrefix(l[m[1]:], "に")
		if v := proseVenueRe.FindStringSubmatch(rest); v != nil {
			e.Prefecture, e.Venue = normalize.SplitPrefecture(strings.TrimSpace(v[1]))
		} else {
			e.Notes = append(e.Notes, "场地未公布")
		}
		e.OpenAt, e.StartAt, _ = times(rest)
		e.Status = statusIn(rest)
		out = append(out, e)
	}
	return out
}

func times(s string) (open, start *domain.Clock, ok bool) {
	if afterStyle(s) {
		// "17:00開場／18:00開演": each time before its word.
		for _, m := range timeWordRe.FindAllStringSubmatch(s, -1) {
			if c, k := normalize.ParseClock(m[1]); k {
				if m[2] == "開場" {
					open, ok = &c, true
				} else {
					start, ok = &c, true
				}
			}
		}
		return
	}
	if m := openRe.FindStringSubmatch(s); m != nil {
		if c, k := normalize.ParseClock(m[1]); k {
			open, ok = &c, true
		}
	}
	if m := startRe.FindStringSubmatch(s); m != nil {
		if c, k := normalize.ParseClock(m[1]); k {
			start, ok = &c, true
		}
	}
	return
}

var (
	clockRe     = regexp.MustCompile(`\d{1,2}:\d{2}`)
	timeWordRe  = regexp.MustCompile(`(\d{1,2}:\d{2})\s*(開場|開演)`)
	wordTimeRe  = regexp.MustCompile(`(?i)(?:OPEN|START|開場|開演|スタート)\s*:?\s*$`)
	wordAfterRe = regexp.MustCompile(`^\s*(?:開場|開演)`)
)

// afterStyle reports whether a line's first time is followed by its word,
// as "17:00開場", and not preceded by one, as "開場17:00".
func afterStyle(s string) bool {
	loc := clockRe.FindStringIndex(s)
	return loc != nil && wordAfterRe.MatchString(s[loc[1]:]) && !wordTimeRe.MatchString(s[:loc[0]])
}

func performers(l string) ([]string, bool) {
	for _, k := range []string{"出演者:", "出演:", "LINE UP:", "LINEUP:", "ACT:"} {
		if v, ok := strings.CutPrefix(l, k); ok {
			return splitNames(v), true
		}
	}
	return nil, false
}

func splitNames(v string) []string {
	v = regexp.MustCompile(`\(※[^)]*\)|※.*$`).ReplaceAllString(v, "")
	parts := regexp.MustCompile(`\s*/\s*|、|,\s*`).Split(v, -1)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		low := strings.ToLower(p)
		if p == "" || low == "and more" || low == "and more..." || p == "ほか" || p == "他" || strings.HasPrefix(p, "...") {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Rounds.

func isRoundName(s string) bool {
	return regexp.MustCompile(`先行|抽選|一般発売|一般販売|先着|プレリザーブ|プレオーダー|受付`).MatchString(s) &&
		!regexp.MustCompile(`^(受付期間|受付|当落発表|当選発表|結果発表|抽選結果発表)$`).MatchString(s)
}

// roundsBeforeShows puts back a year the ticket dates read past the last
// show: written without a year, "7月25日" in a list published without a
// date reads as next July, but tickets are sold before the show.
func (p *state) roundsBeforeShows() {
	var last domain.Date
	for _, e := range p.res.Events {
		if e.Date.After(last) {
			last = e.Date
		}
	}
	if last.Year == 0 {
		return
	}
	limit := last.AddDays(31) // a deadline may follow a festival's first day
	back := func(m *domain.Moment) {
		for i := 0; m != nil && m.Date.After(limit) && i < 3; i++ {
			m.Date = m.Date.AddMonths(-12)
		}
	}
	for i := range p.res.Rounds {
		r := &p.res.Rounds[i]
		back(r.OpensAt)
		back(r.ClosesAt)
		back(r.ResultAt)
	}
}

// bareRoundName reports whether a line is only a round's name: short, with
// a round's word and no date.
func bareRoundName(l string) bool {
	return len([]rune(l)) <= 40 && isRoundName(l) && bareRoundRe.MatchString(l) && !momentRe.MatchString(l) &&
		!strings.ContainsAny(l, ":。、") && !strings.HasPrefix(l, "受付") && !particleEndRe.MatchString(l)
}

var (
	bareRoundRe   = regexp.MustCompile(`先行|抽選|一般発売|一般販売|先着|プレリザーブ|プレオーダー`)
	particleEndRe = regexp.MustCompile(`[をにはがでのてもへと]$`)
)

func roundStart(l string) (name, rest string, ok bool) {
	if m := roundTagRe.FindStringSubmatch(l); m != nil && isRoundName(m[1]) {
		return m[1], m[2], true
	}
	if m := roundKVRe.FindStringSubmatch(l); m != nil && isRoundName(m[1]) {
		return m[1], m[2], true
	}
	if m := roundAngleRe.FindStringSubmatch(l); m != nil && isRoundName(m[1]) {
		return m[1], m[2], true // "<オフィシャル最速先行>"
	}
	return "", "", false
}

func newRound(name string) *Round {
	name = strings.NewReplacer("<", "", ">", "", "《", "", "》", "", "〈", "", "〉", "").Replace(name)
	name = strings.TrimLeft(strings.TrimSpace(name), "○〇●◎・*■◆□◇ ")
	r := &Round{Name: strings.Join(strings.Fields(name), " ")}
	low := strings.ToLower(name)
	switch {
	case containsAny(low, "fc", "ファンクラブ", "会員", "モバイル", "fan club"):
		r.Type = domain.RoundFanClub
	case containsAny(low, "オフィシャル", "official", "hp", "公式", "最速"):
		r.Type = domain.RoundOfficial
	case containsAny(low, "プレイガイド", "ぴあ", "イープラス", "e+", "ローチケ", "ローソン", "livepocket", "プレリザーブ"):
		r.Type = domain.RoundPlayguide
	case strings.Contains(low, "一般"):
		r.Type = domain.RoundGeneral
	default:
		r.Type = domain.RoundOther
	}
	r.Lottery = strings.Contains(name, "抽選") || (r.Type != domain.RoundGeneral && !strings.Contains(name, "先着") && strings.Contains(name, "先行"))
	return r
}

func (p *state) flushRound() {
	if p.round == nil {
		return
	}
	r := p.round
	p.round = nil
	if r.OpensAt == nil && r.ClosesAt == nil && r.ResultAt == nil && r.URL == "" {
		return
	}
	p.res.Rounds = append(p.res.Rounds, *r)
}

// roundLine reads a line into the current round, reporting whether it was
// about it.
func (p *state) roundLine(l string) bool {
	r := p.round
	if u := urlRe.FindString(l); u != "" && !momentRe.MatchString(strings.Replace(l, u, "", 1)) {
		if r.URL == "" {
			r.URL = u
		}
		return true
	}
	if containsAny(l, "当落", "当選発表", "結果発表", "抽選結果", "当選結果") {
		if ms := p.moments(l); len(ms) > 0 {
			r.ResultAt = &ms[0]
			return true
		}
		return false
	}
	opens, closes, ms := p.span(l)
	if len(ms) == 0 && opens == nil {
		return false
	}
	switch {
	case opens != nil && closes != nil:
		r.OpensAt, r.ClosesAt = opens, closes
	case containsAny(l, "まで", "締切", "〆切", "締め切り") && len(ms) > 0:
		r.ClosesAt = &ms[len(ms)-1]
	case opens != nil && (r.OpensAt == nil || !isNote(l)):
		r.OpensAt = opens
	}
	if r.OpensAt != nil && r.ClosesAt != nil && r.ClosesAt.Date.Before(r.OpensAt.Date) && r.ClosesAt.Date.Year < r.OpensAt.Date.Year {
		// "2026/8/20(木)12:00~2025/9/21(日)23:59": a typo in the year
		c := *r.ClosesAt
		for c.Date.Before(r.OpensAt.Date) {
			c.Date = c.Date.AddMonths(12)
		}
		r.ClosesAt = &c
	}
	if u := urlRe.FindString(l); u != "" && r.URL == "" {
		r.URL = u
	}
	return true
}

// isNote reports whether a line is a remark, as "*11/13公演のみの受付となります。".
func isNote(l string) bool {
	return strings.HasPrefix(l, "*") || strings.HasPrefix(l, "※") || strings.HasPrefix(l, "注")
}

// dedupEvents drops a show listed twice in one announcement, as a feed's
// structured dates followed by the same dates in its free text.
func (p *state) dedupEvents() {
	key := func(e Event) string {
		v := strings.NewReplacer(" ", "", "|", "", "・", "").Replace(e.Venue)
		return e.Date.String() + "|" + v
	}
	seen := map[string]int{}
	var out []Event
	for _, e := range p.res.Events {
		k := key(e)
		if i, ok := seen[k]; ok {
			f := &out[i]
			if f.StartAt == nil || e.StartAt == nil || *f.StartAt == *e.StartAt {
				if f.StartAt == nil {
					f.OpenAt, f.StartAt = e.OpenAt, e.StartAt
				}
				if f.Prefecture == "" {
					f.Prefecture = e.Prefecture
				}
				if f.TicketURL == "" {
					f.TicketURL = e.TicketURL
				}
				continue
			}
		}
		seen[k] = len(out)
		out = append(out, e)
	}
	p.res.Events = out
}

// span reads "A~B" (B may be "8日(日)23:59" or "23:59"), or a single moment.
func (p *state) span(l string) (opens, closes *domain.Moment, all []domain.Moment) {
	left, right, isRange := strings.Cut(l, "~")
	lm := p.moments(left)
	if !isRange {
		if len(lm) > 0 {
			return &lm[0], nil, lm
		}
		return nil, nil, nil
	}
	if len(lm) == 0 {
		return nil, nil, nil
	}
	o := lm[len(lm)-1]
	rm := p.moments(right)
	all = append(lm, rm...)
	switch {
	case len(rm) > 0:
		return &o, &rm[0], all
	default:
		if m := dayOnlyRe.FindStringSubmatch(right); m != nil {
			d := domain.Date{Year: o.Date.Year, Month: o.Date.Month, Day: normalize.Atoi(m[1])}
			if d.Before(o.Date) {
				d = d.AddMonths(1)
				d.Day = normalize.Atoi(m[1])
			}
			c := domain.Moment{Date: d}
			if m[3] != "" {
				c.Clock = &domain.Clock{Hour: normalize.Atoi(m[3]), Minute: normalize.Atoi(m[4])}
			}
			return &o, &c, append(all, c)
		}
		if m := timeOnlyRe.FindStringSubmatch(right); m != nil {
			c := domain.Moment{Date: o.Date, Clock: &domain.Clock{Hour: normalize.Atoi(m[1]), Minute: normalize.Atoi(m[2])}}
			return &o, &c, append(all, c)
		}
	}
	return &o, nil, all // "10:00~": open-ended
}

func (p *state) moments(s string) []domain.Moment {
	var out []domain.Moment
	for _, m := range momentRe.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > 0 {
			prev := s[m[0]-1]
			if prev >= '0' && prev <= '9' {
				continue
			}
		}
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return s[m[2*i]:m[2*i+1]]
		}
		month, day := normalize.Atoi(g(2)), normalize.Atoi(g(3))
		if month < 1 || month > 12 || day < 1 || day > 31 {
			continue
		}
		var y int
		if g(1) != "" {
			y = normalize.Atoi(g(1))
		} else {
			var wd *time.Weekday
			if w, ok := normalize.Weekday(g(4)); ok {
				wd = &w
			}
			y, _ = normalize.InferYear(time.Month(month), day, wd, p.ref)
		}
		mo := domain.Moment{Date: domain.Date{Year: y, Month: time.Month(month), Day: day}}
		if g(5) != "" {
			h, mi := normalize.Atoi(g(5)), normalize.Atoi(g(6))
			if h <= 29 && mi <= 59 {
				mo.Clock = &domain.Clock{Hour: h, Minute: mi}
			}
		}
		out = append(out, mo)
	}
	return out
}

// Helpers.

func statusIn(s string) domain.Status {
	low := strings.ToLower(s)
	switch {
	case containsAny(s, "中止", "開催見合わせ", "公演取りやめ", "開催取りやめ") || strings.Contains(low, "cancel"):
		return domain.StatusCancelled
	case strings.Contains(s, "延期") || strings.Contains(low, "postpone"):
		return domain.StatusPostponed
	case containsAny(low, "sold out", "soldout", "完売", "ソールドアウト", "売り切れ"):
		return domain.StatusSoldOut
	}
	return ""
}

func guessArtist(title string) string {
	for _, sep := range []string{"、", " ", "が", "の"} {
		if a, _, ok := strings.Cut(title, sep); ok {
			if n := len([]rune(a)); n >= 2 && n <= 30 && !liveWords.MatchString(a) {
				return a
			}
			break
		}
	}
	return ""
}

func clean(xs []string) []string {
	var out []string
	for _, x := range xs {
		out = appendUnique(out, splitNames(normalize.Text(x))...)
	}
	return out
}

func appendUnique(xs []string, ys ...string) []string {
	for _, y := range ys {
		dup := false
		for _, x := range xs {
			if normalize.Key(x) == normalize.Key(y) {
				dup = true
				break
			}
		}
		if !dup && y != "" {
			xs = append(xs, y)
		}
	}
	return xs
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// Schedules.

var (
	yearLineRe  = regexp.MustCompile(`^(\d{4})\s*[年.]?$`)
	packedRe    = regexp.MustCompile(`^(\d{4})(\d{2})\.(\d{2})\s*([日月火水木金土][・祝休]*)?(.*)$`)
	enWeekdayRe = regexp.MustCompile(`^(\d{4}\s*[./]\s*\d{1,2}\s*[./]\s*\d{1,2}|\d{1,2}\s*[./]\s*\d{1,2})\s*\[?((?i:sun|mon|tue|wed|thu|fri|sat)[a-z]*\.?)\]?(\s|$)`)
	rangeRe     = regexp.MustCompile(`^\s*(?:\([^)]{1,8}\))?\s*(?:(?i:sun|mon|tue|wed|thu|fri|sat)[a-z]*)?\s*[~-]\s*(?:\d{4}\s*[./年]\s*)?\d{1,2}\s*[./月]\s*\d{1,2}`)
	navRe       = regexp.MustCompile(`(?i)^(?:back|list|top|news|topics|info|release|media|live|event|live/event|update|more|close|menu|schedule|goods|home|page top|go top|back to list|view more|日本語|english|한국어|中文)(?:\s|/|$)`)
	dayCountRe  = regexp.MustCompile(`\d{1,2}\s*日?\s*\([^)]{1,6}\)`)
	timePairRe  = regexp.MustCompile(`^(?:OPEN\s*/\s*START\s*[:：]?\s*)?(\d{1,2}:\d{2})\s*/\s*(\d{1,2}:\d{2})$`)
	venueURLRe  = regexp.MustCompile(`\s*[(（]\s*https?://[^)）]*[)）]`)
	notVenueRe  = regexp.MustCompile(`(?i)^会場|立ち見|着席|SOLD ?OUT|https?:|OPEN|START|開場|開演|詳細|MORE|SHARE|お問い合わせ|問合せ|INFO|決定|発表|受付|販売|発売|チケット|TICKET|TOUR|ツアー|公演|presents|配信|放送|[。!?「」『』“”"]|^[(\[].*[)\]]$|^[~-]$`)
)

// scheduleLines joins what a site's layout splits over lines, so that each
// show starts on a line of its own: "2022" over "12.17" (a year above the
// day), "2026." over "10.01", "202708.14土" (a packed year), and adds the
// parentheses of English weekdays, as "2021.12.31 Fri".
var packedVenueRe = regexp.MustCompile(`^(\d{1,2}\.\d{1,2})\(?(MON|TUE|WED|THU|FRI|SAT|SUN)\)?(\S.+?)(北海道|東京都|京都府|大阪府|(?:青森|岩手|宮城|秋田|山形|福島|茨城|栃木|群馬|埼玉|千葉|神奈川|新潟|富山|石川|福井|山梨|長野|岐阜|静岡|愛知|三重|滋賀|兵庫|奈良|和歌山|鳥取|島根|岡山|広島|山口|徳島|香川|愛媛|高知|福岡|佐賀|長崎|熊本|大分|宮崎|鹿児島|沖縄)県)\s*(OPEN\s*\d{1,2}:\d{2}\s*/\s*START\s*\d{1,2}:\d{2})`)

func scheduleLines(in []string) []string {
	var out []string
	for i := 0; i < len(in); i++ {
		l := in[i]
		// "■会場:" over the venue: a field's label, not a section.
		if h, ok := header(l); ok && strings.HasSuffix(h, ":") && i+1 < len(in) {
			label := strings.TrimSuffix(h, ":")
			switch {
			case strings.Contains(label, "期間") || dayCountRe.FindAllStringIndex(in[i+1], -1) != nil && len(dayCountRe.FindAllStringIndex(in[i+1], -1)) >= 3:
				i++ // a festival's days: the artist plays one of them
				continue
			case strings.Contains(label, "会場"):
				l = "会場:" + in[i+1]
				i++
			default:
				continue
			}
		}
		if m := packedRe.FindStringSubmatch(l); m != nil {
			l = m[1] + "." + m[2] + "." + m[3]
			if m[4] != "" {
				l += "(" + m[4] + ")"
			}
			l += " " + strings.TrimSpace(m[5])
		}
		// "01.26TUESGCホール有明東京都OPEN 18:00 / START 19:00ホットスタッフ…":
		// date, weekday, venue, prefecture, times and the promoter's phone.
		if m := packedVenueRe.FindStringSubmatch(l); m != nil {
			l = m[1] + "(" + m[2] + ") " + m[4] + " " + strings.TrimSpace(m[3]) + " " + m[5]
		}
		if m := enWeekdayRe.FindStringSubmatchIndex(l); m != nil {
			l = l[:m[3]] + "(" + l[m[4]:m[5]] + ")" + l[m[6]:]
		}
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

// continuation reads the lines under a dated line of a schedule: its
// times ("17:00 / 18:00"), its prefecture and its venue, each on a line
// of its own. It reports whether the line was one of them.
func (p *state) continuation(l string) bool {
	if len(p.last) == 0 {
		return false
	}
	p.since++
	if p.since > 5 {
		p.last = nil
		return false
	}
	evs := p.res.Events
	if weekdayLineRe.MatchString(l) {
		return true // "SAT" under "03.21"
	}
	if m := timePairRe.FindStringSubmatch(l); m != nil {
		o, ok1 := normalize.ParseClock(m[1])
		s, ok2 := normalize.ParseClock(m[2])
		if ok1 && ok2 {
			for _, i := range p.last {
				if evs[i].StartAt == nil {
					evs[i].OpenAt, evs[i].StartAt = &o, &s
				}
			}
			p.block = nil
			return true
		}
	}
	if evs[p.last[0]].Venue != "" {
		return false
	}
	if pref := normalize.Prefecture(l); pref != "" && len([]rune(l)) <= 4 {
		for _, i := range p.last {
			evs[i].setVenue(pref, "")
		}
		return true
	}
	if !venueLike(l) {
		return false
	}
	pref, venue := normalize.SplitPrefecture(l)
	for _, i := range p.last {
		evs[i].setVenue(pref, venue)
	}
	return true
}

func venueLike(l string) bool {
	if isNote(l) {
		return false
	}
	n := len([]rune(l))
	if n < 2 || n > 64 || notVenueRe.MatchString(l) || navRe.MatchString(l) || eventRe.MatchString(l) {
		return false
	}
	if sentenceRe.MatchString(l) {
		return false
	}
	digits, letters := 0, 0
	for _, r := range l {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case unicode.IsLetter(r):
			letters++
		}
	}
	return digits*3 < n && letters >= 2
}

// sentenceRe matches lines of prose, not names: ending in a period or a
// comma, an English sentence, or a fan's guide (参戦心得).
var sentenceRe = regexp.MustCompile(`[.,、，]$|^[A-Z][a-z]+(?: [a-z]+){3,}|心得|注意事項|ご案内|^vol\.`)

// dropBareDates removes the dated lines that are not shows, as the date a
// page was posted: in a schedule where some shows give a venue, a date
// without one is not a show.
func (p *state) dropBareDates() {
	var keep []Event
	for _, e := range p.res.Events {
		if e.Venue != "" {
			keep = append(keep, e)
		}
	}
	if len(keep) > 0 {
		p.res.Events = keep
	}
}
