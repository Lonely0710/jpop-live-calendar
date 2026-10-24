package views

import (
	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/geo"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

// The place filters use Japan's usual regions and the capital city of each,
// because the data has a prefecture (and sometimes a venue) but not a
// consistent city field. The filters bind stable IDs, never the labels the
// current language shows.
const (
	allRegions = "all"
	allCities  = "all"
)

type regionInfo struct {
	id          string
	prefectures []string
	city        string // the capital's ID, as the filter binds it
	cityPref    string // the prefecture the capital belongs to
}

var japanRegions = []regionInfo{
	{"hokkaido", []string{"北海道"}, "sapporo", "北海道"},
	{"tohoku", []string{"青森", "岩手", "宮城", "秋田", "山形", "福島"}, "sendai", "宮城"},
	{"kanto", []string{"茨城", "栃木", "群馬", "埼玉", "千葉", "東京", "神奈川"}, "tokyo", "東京"},
	{"chubu", []string{"新潟", "富山", "石川", "福井", "山梨", "長野", "岐阜", "静岡", "愛知"}, "nagoya", "愛知"},
	{"kansai", []string{"三重", "滋賀", "京都", "大阪", "兵庫", "奈良", "和歌山"}, "osaka", "大阪"},
	{"chugoku", []string{"鳥取", "島根", "岡山", "広島", "山口"}, "hiroshima", "広島"},
	{"shikoku", []string{"徳島", "香川", "愛媛", "高知"}, "takamatsu", "香川"},
	{"kyushu", []string{"福岡", "佐賀", "長崎", "熊本", "大分", "宮崎", "鹿児島", "沖縄"}, "fukuoka", "福岡"},
}

// regionLabel names a region ID in the language the app shows.
func regionLabel(id string) string {
	switch id {
	case "hokkaido":
		return locales.RegionHokkaido()
	case "tohoku":
		return locales.RegionTohoku()
	case "kanto":
		return locales.RegionKanto()
	case "chubu":
		return locales.RegionChubu()
	case "kansai":
		return locales.RegionKansai()
	case "chugoku":
		return locales.RegionChugoku()
	case "shikoku":
		return locales.RegionShikoku()
	case "kyushu":
		return locales.RegionKyushu()
	}
	return locales.RegionAll()
}

// cityLabel names a region capital's ID in the language the app shows.
func cityLabel(id string) string {
	switch id {
	case "sapporo":
		return locales.CitySapporo()
	case "sendai":
		return locales.CitySendai()
	case "tokyo":
		return locales.CityTokyo()
	case "nagoya":
		return locales.CityNagoya()
	case "osaka":
		return locales.CityOsaka()
	case "hiroshima":
		return locales.CityHiroshima()
	case "takamatsu":
		return locales.CityTakamatsu()
	case "fukuoka":
		return locales.CityFukuoka()
	}
	return locales.CityAll()
}

// prefEN romanizes the prefectures whose data keeps their Japanese names.
var prefEN = map[string]string{
	"北海道": "Hokkaido", "青森": "Aomori", "岩手": "Iwate", "宮城": "Miyagi",
	"秋田": "Akita", "山形": "Yamagata", "福島": "Fukushima", "茨城": "Ibaraki",
	"栃木": "Tochigi", "群馬": "Gunma", "埼玉": "Saitama", "千葉": "Chiba",
	"東京": "Tokyo", "神奈川": "Kanagawa", "新潟": "Niigata", "富山": "Toyama",
	"石川": "Ishikawa", "福井": "Fukui", "山梨": "Yamanashi", "長野": "Nagano",
	"岐阜": "Gifu", "静岡": "Shizuoka", "愛知": "Aichi", "三重": "Mie",
	"滋賀": "Shiga", "京都": "Kyoto", "大阪": "Osaka", "兵庫": "Hyogo",
	"奈良": "Nara", "和歌山": "Wakayama", "鳥取": "Tottori", "島根": "Shimane",
	"岡山": "Okayama", "広島": "Hiroshima", "山口": "Yamaguchi", "徳島": "Tokushima",
	"香川": "Kagawa", "愛媛": "Ehime", "高知": "Kochi", "福岡": "Fukuoka",
	"佐賀": "Saga", "長崎": "Nagasaki", "熊本": "Kumamoto", "大分": "Oita",
	"宮崎": "Miyazaki", "鹿児島": "Kagoshima", "沖縄": "Okinawa",
}

// prefLabel names a prefecture as the data stores it: its Japanese name,
// or its romanization in English.
func prefLabel(name string) string {
	if lang() == "en" {
		if en, ok := prefEN[name]; ok {
			return en
		}
	}
	return geo.Suffixed(name)
}

func regionIDs() []string {
	out := []string{allRegions}
	for _, r := range japanRegions {
		out = append(out, r.id)
	}
	return out
}

func regionInfoOf(id string) (regionInfo, bool) {
	for _, r := range japanRegions {
		if r.id == id {
			return r, true
		}
	}
	return regionInfo{}, false
}

// cityIDs is the city select's choices: every region's capital, or only
// the selected region's capital.
func cityIDs(region string) []string {
	if r, ok := regionInfoOf(region); ok {
		return []string{allCities, r.city}
	}
	out := []string{allCities}
	for _, r := range japanRegions {
		out = append(out, r.city)
	}
	return out
}

func cityPrefecture(id string) (string, bool) {
	for _, r := range japanRegions {
		if r.city == id {
			return r.cityPref, true
		}
	}
	return "", false
}

// regionOf returns the region a prefecture belongs to.
func regionOf(prefecture string) (regionInfo, bool) {
	for _, r := range japanRegions {
		for _, p := range r.prefectures {
			if p == prefecture {
				return r, true
			}
		}
	}
	return regionInfo{}, false
}

func hasString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// placeSelect is a filter's drop-down: it binds a stable ID while showing
// the label of the language the app shows, which ui.Select cannot, as it
// binds the string it shows.
func placeSelect(c *ui.Context, selected *string, ids []string, label func(string) string, aria string, width float32) {
	sel := ui.SelectBase(c, selected)
	sel.Trigger.Width(width).Padding(6, 10).Gap(6).Radius(theme.Radius).Border(1, theme.MistGray).
		AlignItems(ui.Center).Label(aria)
	if sel.Trigger.Hovered() {
		sel.Trigger.Background(theme.Hover)
	}
	sel.Trigger.Children(func() {
		ui.Text(c, label(*selected)).Grow(1).Shrink(1).SingleLine()
		mygoicons.Image(c, icons.Next, 12).Rotate(90).Opacity(0.6)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Padding(4).Radius(theme.Radius).Background(theme.CloudWhite).Border(1, theme.MistGray)
		for _, id := range ids {
			item := sel.Item(id).Padding(6, 9).Radius(4)
			if item.Highlighted() {
				item.Background(theme.GlassSelected)
			}
			item.Children(func() {
				ui.Text(c, label(id))
			})
		}
	})
}

// entryPrefectures is every prefecture an entry happens in: a show has one,
// a ticket round may cover several shows.
func (a *App) entryPrefectures(e domain.Entry) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		ev, ok := a.snap.Event(id)
		if !ok || ev.Prefecture == "" || seen[ev.Prefecture] {
			return
		}
		seen[ev.Prefecture] = true
		out = append(out, ev.Prefecture)
	}
	if e.EventID != "" {
		add(e.EventID)
	}
	if e.RoundID != "" {
		if r, ok := a.snap.Round(e.RoundID); ok {
			for _, id := range r.EventIDs {
				add(id)
			}
		}
	}
	return out
}

// placeMatches applies the region and city filters. Entries without a known
// prefecture are hidden while one of the filters is active, never guessed.
func (a *App) placeMatches(e domain.Entry) bool {
	region, city := a.region, a.city
	if region == "" {
		region = allRegions
	}
	if city == "" {
		city = allCities
	}
	if region == allRegions && city == allCities {
		return true
	}
	prefs := a.entryPrefectures(e)
	if len(prefs) == 0 {
		return false
	}
	if region != allRegions {
		r, ok := regionInfoOf(region)
		if !ok {
			return false
		}
		found := false
		for _, p := range prefs {
			if hasString(r.prefectures, p) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if city != allCities {
		pref, ok := cityPrefecture(city)
		if !ok || !hasString(prefs, pref) {
			return false
		}
	}
	return true
}
