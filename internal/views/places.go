package views

import "jpop-live-calendar/internal/domain"

// The place filters use Japan's usual regions and the capital city of each,
// because the data has a prefecture (and sometimes a venue) but not a
// consistent city field.
const (
	allRegions = "全部地区"
	allCities  = "全部城市"
)

type regionInfo struct {
	label       string
	prefectures []string
	city        string // the region's capital, in the filter's Chinese label
	cityPref    string // the prefecture the capital belongs to
}

var japanRegions = []regionInfo{
	{"北海道", []string{"北海道"}, "札幌", "北海道"},
	{"东北", []string{"青森", "岩手", "宮城", "秋田", "山形", "福島"}, "仙台", "宮城"},
	{"关东", []string{"茨城", "栃木", "群馬", "埼玉", "千葉", "東京", "神奈川"}, "东京", "東京"},
	{"中部", []string{"新潟", "富山", "石川", "福井", "山梨", "長野", "岐阜", "静岡", "愛知"}, "名古屋", "愛知"},
	{"关西（近畿）", []string{"三重", "滋賀", "京都", "大阪", "兵庫", "奈良", "和歌山"}, "大阪", "大阪"},
	{"中国", []string{"鳥取", "島根", "岡山", "広島", "山口"}, "广岛", "広島"},
	{"四国", []string{"徳島", "香川", "愛媛", "高知"}, "高松", "香川"},
	{"九州・冲绳", []string{"福岡", "佐賀", "長崎", "熊本", "大分", "宮崎", "鹿児島", "沖縄"}, "福冈", "福岡"},
}

var regionOptions = func() []string {
	out := []string{allRegions}
	for _, r := range japanRegions {
		out = append(out, r.label)
	}
	return out
}()

func regionInfoOf(label string) (regionInfo, bool) {
	for _, r := range japanRegions {
		if r.label == label {
			return r, true
		}
	}
	return regionInfo{}, false
}

// cityOptions is the city select's choices: every region's capital, or only
// the selected region's capital.
func cityOptions(region string) []string {
	if r, ok := regionInfoOf(region); ok {
		return []string{allCities, r.city}
	}
	out := []string{allCities}
	for _, r := range japanRegions {
		out = append(out, r.city)
	}
	return out
}

func cityPrefecture(label string) (string, bool) {
	for _, r := range japanRegions {
		if r.city == label {
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
