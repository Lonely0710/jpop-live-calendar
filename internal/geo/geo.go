// Package geo is the map of Japan the map page draws: the 47 prefectures,
// simplified, with the Nansei Islands (Okinawa and Amami) in an inset, and
// each prefecture's capital, where its shows are marked.
//
// The outlines come from 地球地図日本 (Global Map Japan, by the Geospatial
// Information Authority of Japan), through the GeoJSON of the "Data of
// Japan" project (github.com/dataofjapan/land); japan.bin is written by
// internal/genmap. Map units: x grows east, y south, 200 units to a degree
// of latitude; nothing here touches the network.
package geo

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"sync"

	"jpop-live-calendar/internal/normalize"
)

//go:embed japan.bin
var japanBin []byte

// Point is a position in map units.
type Point struct{ X, Y float32 }

// Rect is a rectangle in map units.
type Rect struct{ X, Y, W, H float32 }

// Contains reports whether p is in r.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X <= r.X+r.W && p.Y >= r.Y && p.Y <= r.Y+r.H
}

// Prefecture is one prefecture's outline.
type Prefecture struct {
	// Name is the prefecture without its suffix (都・道・府・県), as the
	// catalog stores it: "東京", "北海道".
	Name    string
	Rings   [][]Point // outer rings: the main land and its larger islands
	Bounds  Rect
	Capital Point // the prefectural office, where its marker goes
}

// Map is the map of Japan.
type Map struct {
	Size Point // the map's extent; it starts at the origin
	// Inset is the frame of the Nansei Islands, moved to the sea northwest
	// of Honshu.
	Inset       Rect
	Prefectures []Prefecture // in the order of their JIS codes, 北海道 first
}

var (
	japanOnce sync.Once
	japan     *Map
	japanErr  error
)

// Japan returns the map, decoded once; it panics on a corrupt embed, which
// the package's tests catch.
func Japan() *Map {
	japanOnce.Do(func() { japan, japanErr = decode(japanBin) })
	if japanErr != nil {
		panic(japanErr)
	}
	return japan
}

func decode(b []byte) (*Map, error) {
	if len(b) < 6 || string(b[:6]) != "JPMAP1" {
		return nil, fmt.Errorf("geo: not a map")
	}
	b = b[6:]
	bad := false
	i16 := func() float32 {
		if len(b) < 2 {
			bad = true
			return 0
		}
		v := int16(binary.LittleEndian.Uint16(b))
		b = b[2:]
		return float32(v)
	}
	u16 := func() int {
		if len(b) < 2 {
			bad = true
			return 0
		}
		v := binary.LittleEndian.Uint16(b)
		b = b[2:]
		return int(v)
	}
	m := &Map{}
	m.Size = Point{i16(), i16()}
	x0, y0, x1, y1 := i16(), i16(), i16(), i16()
	m.Inset = Rect{x0, y0, x1 - x0, y1 - y0}
	names := normalize.Prefectures()
	for _, name := range names {
		p := Prefecture{Name: name, Capital: Point{i16(), i16()}}
		n := u16()
		lo, hi := Point{1e9, 1e9}, Point{-1e9, -1e9}
		for range n {
			k := u16()
			if bad || len(b) < 4*k {
				return nil, fmt.Errorf("geo: %s: truncated", name)
			}
			ring := make([]Point, k)
			for j := range ring {
				pt := Point{i16(), i16()}
				ring[j] = pt
				lo = Point{min(lo.X, pt.X), min(lo.Y, pt.Y)}
				hi = Point{max(hi.X, pt.X), max(hi.Y, pt.Y)}
			}
			p.Rings = append(p.Rings, ring)
		}
		if bad || len(p.Rings) == 0 {
			return nil, fmt.Errorf("geo: %s: no outline", name)
		}
		p.Bounds = Rect{lo.X, lo.Y, hi.X - lo.X, hi.Y - lo.Y}
		m.Prefectures = append(m.Prefectures, p)
	}
	if len(b) != 0 {
		return nil, fmt.Errorf("geo: %d bytes left over", len(b))
	}
	return m, nil
}

// Find returns the index of the prefecture named name, or -1.
func (m *Map) Find(name string) int {
	for i, p := range m.Prefectures {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// Hit returns the index of the prefecture under pt, or -1 over the sea.
func (m *Map) Hit(pt Point) int {
	for i, p := range m.Prefectures {
		if !p.Bounds.Contains(pt) {
			continue
		}
		for _, r := range p.Rings {
			if inside(r, pt) {
				return i
			}
		}
	}
	return -1
}

// inside is the even–odd test of pt against the closed ring r.
func inside(r []Point, pt Point) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		a, b := r[i], r[j]
		if (a.Y > pt.Y) != (b.Y > pt.Y) && pt.X < (b.X-a.X)*(pt.Y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}

// Suffixed is a prefecture's full name: "東京" → "東京都", "大阪" → "大阪府",
// "北海道" as it is, the others with 県.
func Suffixed(name string) string {
	switch name {
	case "北海道", "":
		return name
	case "東京":
		return "東京都"
	case "京都", "大阪":
		return name + "府"
	}
	return name + "県"
}
