// Package domain holds the public data model of Live Calendar.
//
// Everything here describes public information only: official
// announcements, lives and ticket rounds. There is no user data, no login
// and no database; the catalog is kept in memory.
package domain

import (
	"time"
	_ "time/tzdata" // Asia/Tokyo must work even without a system zoneinfo.
)

// Tokyo is the zone every Japanese live and ticket time is given in.
var Tokyo = mustLoad("Asia/Tokyo")

// TokyoName is the IANA name stored on events.
const TokyoName = "Asia/Tokyo"

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// EventType is what kind of show an event is.
type EventType string

const (
	TypeLive     EventType = "live"     // ワンマン / ツアー公演
	TypeFestival EventType = "festival" // フェス / 対バン / イベント出演
	TypeEvent    EventType = "event"    // トーク・公開収録など
)

// Status is a public fact about an event, never a personal state.
type Status string

const (
	StatusAnnounced Status = "announced" // 已公布
	StatusSoldOut   Status = "sold_out"  // 售罄
	StatusPostponed Status = "postponed" // 延期
	StatusCancelled Status = "cancelled" // 中止 / 取消
)

// Change records an update that must not be overwritten silently: a new
// time, a postponement, a moved deadline.
type Change struct {
	Field  string // 例：开演时间、状态、抽选截止
	Old    string
	New    string
	At     time.Time // when the announcement saying so was published
	Source string    // announcement URL
}

// Artist is a performer, with the spellings sources use for it.
type Artist struct {
	ID          string
	Name        string
	Aliases     []string
	OfficialURL string
	// AvatarURL, when set, is the picture to show in place of the one
	// looked up by name; Color ("#RRGGBB"), the theme color in place of
	// the one taken from the picture.
	AvatarURL string
	Color     string
	// Followed is set for the artists listed in sources.yaml: they stay on
	// the 艺人 page even before any show of theirs is found.
	Followed bool
}

// Venue is where a show takes place.
type Venue struct {
	ID          string
	Name        string
	Prefecture  string
	City        string
	Address     string
	OfficialURL string
}

// PublicEvent is one show on the calendar.
type PublicEvent struct {
	ID       string
	Title    string
	Tour     string
	Type     EventType
	Status   Status
	Timezone string

	// StartDate is the day of the show; OpenAt and StartAt are nil while
	// not announced (时间未公布) and are never guessed.
	StartDate Date
	OpenAt    *Clock
	StartAt   *Clock

	ArtistIDs  []string
	VenueID    string
	VenueName  string // as written in the announcement
	Prefecture string
	TicketURL  string

	AnnouncementIDs []string
	SourceIDs       []string
	Notes           []string // stated facts, as 振替公演
	NeedsReview     bool
	ReviewNotes     []string
	Changes         []Change
	FirstSeen       time.Time
	UpdatedAt       time.Time
}

// RoundType is who sells a ticket round.
type RoundType string

const (
	RoundFanClub   RoundType = "fanclub"   // FC / 会员先行
	RoundOfficial  RoundType = "official"  // オフィシャル先行
	RoundPlayguide RoundType = "playguide" // プレイガイド先行
	RoundGeneral   RoundType = "general"   // 一般发售
	RoundOther     RoundType = "other"
)

// PublicTicketRound is one public ticket stage. Its times are separate and
// each may be missing.
type PublicTicketRound struct {
	ID      string
	Name    string // 例：オフィシャル最速先行（抽選）
	Type    RoundType
	Lottery bool // 抽選; false is first come (先着)

	ArtistIDs []string
	Tour      string
	EventIDs  []string // shows the round sells

	OpensAt  *Moment
	ClosesAt *Moment
	ResultAt *Moment

	PurchaseURL string
	SourceURL   string

	AnnouncementIDs []string
	SourceIDs       []string
	NeedsReview     bool
	ReviewNotes     []string
	Changes         []Change
}

// PublicAnnouncement is the evidence an event comes from.
type PublicAnnouncement struct {
	ID          string
	SourceID    string
	URL         string
	Title       string
	PublishedAt time.Time
	FetchedAt   time.Time
	ContentHash string
	RawText     string
	// ArtistHints are performers the source itself names (e.g. a news
	// site's "関連アーティスト").
	ArtistHints []string
	// MetadataOnly marks a feed item whose body was not fetched again
	// because it was already known; the stored body is kept.
	MetadataOnly bool
	// Schedule marks a page of the artist's own live schedule: every
	// dated line of it is a show.
	Schedule bool
	Revision int
}

// EntryKind is a node on the calendar.
type EntryKind string

const (
	EntryLive         EntryKind = "live"          // 演出当天
	EntryLotteryOpen  EntryKind = "lottery_open"  // 抽选 / 先行受付开始
	EntryLotteryClose EntryKind = "lottery_close" // 抽选 / 先行受付截止
	EntryResult       EntryKind = "result"        // 结果公布
	EntrySaleOpen     EntryKind = "sale_open"     // 一般发售 / 先着发售开始
)

// Entry is one item the calendar shows: a show, or one time of a ticket
// round. Entries are derived from the catalog on every rebuild.
// Label is the kind's name in the UI.
func (k EntryKind) Label() string {
	switch k {
	case EntryLive:
		return "Live"
	case EntryLotteryOpen:
		return "受付开始"
	case EntryLotteryClose:
		return "受付截止"
	case EntryResult:
		return "结果公布"
	case EntrySaleOpen:
		return "一般/先着发售"
	}
	return string(k)
}

// Short is the kind in two characters, for narrow calendar cells.
func (k EntryKind) Short() string {
	switch k {
	case EntryLotteryOpen:
		return "开始"
	case EntryLotteryClose:
		return "截止"
	case EntryResult:
		return "结果"
	case EntrySaleOpen:
		return "发售"
	}
	return ""
}

// Kinds lists every entry kind in display order.
var Kinds = []EntryKind{EntryLive, EntryLotteryOpen, EntryLotteryClose, EntryResult, EntrySaleOpen}

// Label is the status's name in the UI.
func (s Status) Label() string {
	switch s {
	case StatusSoldOut:
		return "售罄"
	case StatusPostponed:
		return "延期"
	case StatusCancelled:
		return "中止"
	}
	return "已公布"
}

// Label is the round type's name in the UI.
func (t RoundType) Label() string {
	switch t {
	case RoundFanClub:
		return "FC 先行"
	case RoundOfficial:
		return "官方先行"
	case RoundPlayguide:
		return "售票平台先行"
	case RoundGeneral:
		return "一般发售"
	}
	return "其他"
}

type Entry struct {
	ID      string
	Kind    EntryKind
	Date    Date
	At      *Clock // nil: 时间未公布
	Title   string
	Detail  string // venue or round name
	EventID string
	RoundID string

	Status      Status
	Important   bool // deadlines, postponements, cancellations
	Changed     bool
	NeedsReview bool
	SourceIDs   []string
	ArtistIDs   []string
	Search      string // lower-case text searched
}

// SourceKind is how a source is read.
type SourceKind string

const (
	SourceDemo         SourceKind = "demo"
	SourceRSS          SourceKind = "rss"
	SourceOfficialSite SourceKind = "website"
	SourceAPI          SourceKind = "api"
)

// ArticleOptions tells an RSS source to read the linked article pages.
type ArticleOptions struct {
	Enabled     bool
	BodyStart   string // HTML marker the article body starts at
	BodyEnd     string
	ArtistStart string // text marker of the source's artist list
	ArtistEnd   string
	Keywords    []string      // only titles with one of these are opened
	Exclude     []string      // …and none of these ("グッズ", "配信")
	Include     string        // only titles matching this regular expression are kept at all
	TitleOnly   bool          // keep only the title until an article is opened; ignore the feed's description
	MaxAge      time.Duration // skip items older than this (0 keeps every item)
	MaxPerSync  int
	Delay       time.Duration
}

// SourceConfig configures one public data source.
type SourceConfig struct {
	ID       string
	Name     string
	Kind     SourceKind
	URL      string
	Enabled  bool
	Interval time.Duration
	Article  ArticleOptions
	// Website sources: the artist whose own site it is, a pattern of the
	// detail pages to open from a list page, and why a source is off.
	Artist string
	Follow string
	Note   string
	// Feature is a pattern of the tour pages ("…/feature/tour2026") an
	// announcement links to: they are found from the announcements and
	// opened too, so a tour's page is never configured by hand.
	Feature string
	// Layer is what the source watches: LayerLive (the LIVE/SCHEDULE list,
	// read every 12h) or LayerNews (NEWS/TICKET, every 4–6h).
	Layer string
}

// Source layers.
const (
	LayerLive = "live"
	LayerNews = "news"
)

// SyncState is the outcome of one source in a sync.
type SyncState string

const (
	SyncIdle      SyncState = "idle"
	SyncRunning   SyncState = "running"
	SyncSucceeded SyncState = "succeeded"
	SyncFailed    SyncState = "failed"
	SyncSkipped   SyncState = "skipped"
)

// SyncRun records one source's sync.
type SyncRun struct {
	SourceID    string
	State       SyncState
	StartedAt   time.Time
	FinishedAt  time.Time
	Fetched     int // announcements read
	New         int
	Updated     int
	NotModified bool
	Err         string
	// Retry marks a failure that is likely temporary (a robots.txt that
	// could not be read while a site is under maintenance): the source is
	// retried sooner than its interval.
	Retry bool
}
