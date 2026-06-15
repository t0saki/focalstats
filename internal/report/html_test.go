package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/t0saki/focalstats/internal/scan"
)

func richStats() scan.Stats {
	s := scan.Stats{
		Basis:       scan.Basis35mm,
		Round:       1,
		Total:       10,
		WithFocal:   8,
		NoFocal:     1,
		Failed:      1,
		WithDate:    8,
		MinDate:     time.Date(2021, 3, 1, 9, 0, 0, 0, time.UTC),
		MaxDate:     time.Date(2024, 7, 1, 18, 0, 0, 0, time.UTC),
		Focal:       map[int]int{24: 5, 50: 3},
		ByYear:      map[int]int{2021: 4, 2024: 4},
		ByYearMonth: map[string]int{"2021-03": 4, "2024-07": 4},
		Cameras:     map[string]int{"Acme <Cam> & Co": 6, "Phone X": 2},
		Lenses:      map[string]int{"24-70mm F2.8": 8},
		Apertures:   map[string]int{"f/2.8": 5, "f/8": 3},
		ISOs:        map[int]int{100: 6, 800: 2},
		Shutters:    map[string]int{"1/250": 5, "2s": 3},
		FocalByYear: map[int]map[int]int{24: {2021: 3, 2024: 2}, 50: {2024: 3}},
		FocalByCam:  map[string]map[int]int{"Acme <Cam> & Co": {24: 4, 50: 2}, "Phone X": {24: 2}},
	}
	s.ByHour[9] = 4
	s.ByHour[18] = 4
	s.ByWeekday[1] = 5
	s.ByWeekday[6] = 3
	return s
}

func TestRenderHTML(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderHTML(&buf, richStats(), HTMLOptions{Title: "/data", Top: 0}); err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		"Focal-length distribution",
		"Focal length × year",
		"Focal length per camera body",
		"By hour of day",
		"By weekday",
		"Camera bodies",
		"Lenses",
		"Exposure settings",
		"<svg",
		"2021-03-01 — 2024-07-01",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing %q", want)
		}
	}

	// EXIF text with HTML metacharacters must be escaped, never raw.
	if strings.Contains(out, "Acme <Cam> & Co") {
		t.Errorf("camera label was not HTML-escaped")
	}
	if !strings.Contains(out, "Acme &lt;Cam&gt; &amp; Co") {
		t.Errorf("expected escaped camera label in output")
	}
}

func TestRenderHTMLEmpty(t *testing.T) {
	var buf bytes.Buffer
	s := scan.Stats{Basis: scan.Basis35mm, Focal: map[int]int{}}
	if err := RenderHTML(&buf, s, HTMLOptions{}); err != nil {
		t.Fatalf("RenderHTML empty: %v", err)
	}
	if !strings.Contains(buf.String(), "<!DOCTYPE html>") {
		t.Errorf("empty report should still be valid HTML")
	}
}
