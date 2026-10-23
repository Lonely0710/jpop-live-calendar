// Command genmap writes japan.bin, the simplified map of Japan's 47
// prefectures that package geo embeds, from the GeoJSON and the capitals
// of the "Data of Japan" project (github.com/dataofjapan/land), drawn
// from 地球地図日本 (Global Map Japan, GSI):
//
//	go run ./internal/geo/internal/genmap -geojson japan.geojson -capitals prefecturalCapital.csv
//
// It keeps each prefecture's outer rings larger than -min square degrees,
// drops the Ogasawara Islands, moves the Nansei Islands (Okinawa and
// Amami) into an inset in the Sea of Japan, as Japanese maps do, and
// simplifies the rings with Douglas–Peucker.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
)

// The projection: equirectangular, its x scaled by the cosine of the
// map's middle latitude, in units of 1/scale degree of latitude.
const (
	scale   = 200
	midLat  = 36.5
	lonMin  = 127.8
	latMax  = 45.75
	insetLt = 29.5 // rings south of this latitude, west of insetLn: the inset
	insetLn = 131.5
	// The inset's shift, in degrees: from around (123–130°E, 24–28.6°N)
	// to the empty sea northwest of Honshu.
	shiftLon = 6.0
	shiftLat = 16.4
)

type pt struct{ x, y float64 } // lon, lat

func main() {
	geo := flag.String("geojson", "japan.geojson", "the prefectures' GeoJSON")
	caps := flag.String("capitals", "prefecturalCapital.csv", "the capitals' CSV (id,nam_ja,lat,lon)")
	out := flag.String("o", "internal/geo/japan.bin", "output file")
	minArea := flag.Float64("min", 0.006, "smallest ring kept, in square degrees")
	tol := flag.Float64("tol", 0.012, "simplification tolerance, in degrees")
	flag.Parse()

	var fc struct {
		Features []struct {
			Properties struct {
				ID int `json:"id"`
			} `json:"properties"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	data, err := os.ReadFile(*geo)
	check(err)
	check(json.Unmarshal(data, &fc))
	rings := make([][][]pt, 48)
	// The inset's bounds, around the shifted islands.
	ilo, ihi := pt{math.Inf(1), math.Inf(1)}, pt{math.Inf(-1), math.Inf(-1)}
	for _, f := range fc.Features {
		var polys [][][][2]float64
		switch f.Geometry.Type {
		case "MultiPolygon":
			check(json.Unmarshal(f.Geometry.Coordinates, &polys))
		case "Polygon":
			var poly [][][2]float64
			check(json.Unmarshal(f.Geometry.Coordinates, &poly))
			polys = [][][][2]float64{poly}
		}
		id := f.Properties.ID
		for _, poly := range polys {
			if len(poly) == 0 {
				continue
			}
			var r []pt
			for _, c := range poly[0] {
				r = append(r, pt{c[0], c[1]})
			}
			if math.Abs(area(r)) < *minArea {
				continue
			}
			lo, hi := bounds(r)
			if hi.y < 30 && lo.x > 135 { // Ogasawara
				continue
			}
			if hi.y < insetLt && hi.x < insetLn {
				for i := range r {
					r[i] = pt{r[i].x + shiftLon, r[i].y + shiftLat}
				}
				lo, hi := bounds(r)
				ilo = pt{math.Min(ilo.x, lo.x), math.Min(ilo.y, lo.y)}
				ihi = pt{math.Max(ihi.x, hi.x), math.Max(ihi.y, hi.y)}
			}
			r = simplify(r, *tol)
			if len(r) >= 4 {
				rings[id] = append(rings[id], r)
			}
		}
	}

	capitals := readCapitals(*caps)
	var w, h float64
	for id := 1; id <= 47; id++ {
		if len(rings[id]) == 0 {
			log.Fatalf("prefecture %d has no ring", id)
		}
		for _, r := range rings[id] {
			for _, p := range r {
				x, y := project(p)
				w, h = math.Max(w, x), math.Max(h, y)
			}
		}
	}
	const pad = 0.35
	x0, y0 := project(pt{ilo.x - pad, ihi.y + pad})
	x1, y1 := project(pt{ihi.x + pad, ilo.y - pad})

	var buf bytes.Buffer
	buf.WriteString("JPMAP1")
	put := func(v any) { check(binary.Write(&buf, binary.LittleEndian, v)) }
	put([]int16{int16(math.Ceil(w)) + 1, int16(math.Ceil(h)) + 1, round(x0), round(y0), round(x1), round(y1)})
	points := 0
	for id := 1; id <= 47; id++ {
		cx, cy := project(capitals[id])
		put([]int16{round(cx), round(cy), int16(len(rings[id]))})
		for _, r := range rings[id] {
			put(uint16(len(r)))
			for _, p := range r {
				x, y := project(p)
				put([]int16{round(x), round(y)})
			}
			points += len(r)
		}
	}
	check(os.WriteFile(*out, buf.Bytes(), 0o644))
	fmt.Printf("%s: %d points, %d bytes, %.0f×%.0f\n", *out, points, buf.Len(), w, h)
}

func project(p pt) (x, y float64) {
	return (p.x - lonMin) * math.Cos(midLat*math.Pi/180) * scale, (latMax - p.y) * scale
}

func round(v float64) int16 { return int16(math.Round(v)) }

func readCapitals(path string) []pt {
	f, err := os.Open(path)
	check(err)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	check(err)
	out := make([]pt, 48)
	for _, row := range rows[1:] {
		id, err := strconv.Atoi(row[0])
		check(err)
		lat, err := strconv.ParseFloat(row[2], 64)
		check(err)
		lon, err := strconv.ParseFloat(row[3], 64)
		check(err)
		p := pt{lon, lat}
		if lat < insetLt && lon < insetLn {
			p = pt{lon + shiftLon, lat + shiftLat}
		}
		out[id] = p
	}
	for id := 1; id <= 47; id++ {
		if out[id] == (pt{}) {
			log.Fatalf("capital %d missing", id)
		}
	}
	return out
}

func area(r []pt) float64 {
	s := 0.0
	for i := range r {
		j := (i + 1) % len(r)
		s += r[i].x*r[j].y - r[j].x*r[i].y
	}
	return s / 2
}

func bounds(r []pt) (lo, hi pt) {
	lo, hi = r[0], r[0]
	for _, p := range r {
		lo = pt{math.Min(lo.x, p.x), math.Min(lo.y, p.y)}
		hi = pt{math.Max(hi.x, p.x), math.Max(hi.y, p.y)}
	}
	return lo, hi
}

// simplify is Douglas–Peucker on a closed ring, split at its first point
// and the point farthest from it.
func simplify(r []pt, tol float64) []pt {
	if len(r) > 1 && r[0] == r[len(r)-1] {
		r = r[:len(r)-1]
	}
	if len(r) < 4 {
		return r
	}
	far, best := 0, -1.0
	for i, p := range r {
		if d := math.Hypot(p.x-r[0].x, p.y-r[0].y); d > best {
			far, best = i, d
		}
	}
	a := dp(r[:far+1], tol)
	b := dp(append(append([]pt{}, r[far:]...), r[0]), tol)
	return append(a[:len(a)-1], b[:len(b)-1]...)
}

func dp(line []pt, tol float64) []pt {
	if len(line) < 3 {
		return line
	}
	a, b := line[0], line[len(line)-1]
	idx, best := 0, 0.0
	for i := 1; i < len(line)-1; i++ {
		if d := segDist(line[i], a, b); d > best {
			idx, best = i, d
		}
	}
	if best <= tol {
		return []pt{a, b}
	}
	l := dp(line[:idx+1], tol)
	rr := dp(line[idx:], tol)
	return append(l[:len(l)-1], rr...)
}

func segDist(p, a, b pt) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.x-a.x, p.y-a.y)
	}
	t := math.Max(0, math.Min(1, ((p.x-a.x)*dx+(p.y-a.y)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p.x-a.x-t*dx, p.y-a.y-t*dy)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
