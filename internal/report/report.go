// Package report turns a scan.Stats into a human- or machine-readable summary
// (aligned terminal table, JSON, CSV, or a self-contained HTML report).
package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/t0saki/focalstats/internal/scan"
)

// Entry is one focal-length bucket.
type Entry struct {
	FocalMM int     `json:"focal_mm"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"` // share of WithFocal, 0..100
}

// Summary is the render-ready focal-length statistics object (table/JSON/CSV).
type Summary struct {
	Basis     string  `json:"basis"`
	Total     int     `json:"total_images"`
	WithFocal int     `json:"with_focal"`
	NoFocal   int     `json:"no_focal"`
	Failed    int     `json:"failed"`
	Entries   []Entry `json:"distribution"` // sorted by focal length ascending
}

// BasisLabel returns a short English label for a basis.
func BasisLabel(b scan.Basis) string {
	if b == scan.BasisActual {
		return "actual focal length"
	}
	return "35mm-equivalent"
}

// focalEntries turns a focal histogram into Entries (sorted ascending), with
// percentages relative to withFocal. If top > 0, only the top-N buckets by
// count are kept (still presented ascending by focal length).
func focalEntries(focal map[int]int, withFocal, top int) []Entry {
	entries := make([]Entry, 0, len(focal))
	for mm, n := range focal {
		pct := 0.0
		if withFocal > 0 {
			pct = float64(n) / float64(withFocal) * 100
		}
		entries = append(entries, Entry{FocalMM: mm, Count: n, Percent: pct})
	}
	if top > 0 && top < len(entries) {
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Count != entries[j].Count {
				return entries[i].Count > entries[j].Count
			}
			return entries[i].FocalMM < entries[j].FocalMM
		})
		entries = entries[:top]
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].FocalMM < entries[j].FocalMM })
	return entries
}

// Build converts a scan.Stats into a focal-length Summary. If top > 0, only the
// top-N focal buckets by count are kept.
func Build(s scan.Stats, top int) Summary {
	return Summary{
		Basis:     BasisLabel(s.Basis),
		Total:     s.Total,
		WithFocal: s.WithFocal,
		NoFocal:   s.NoFocal,
		Failed:    s.Failed,
		Entries:   focalEntries(s.Focal, s.WithFocal, top),
	}
}

// RenderJSON writes s as indented JSON.
func RenderJSON(w io.Writer, s Summary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// RenderCSV writes s as CSV (focal_mm,count,percent).
func RenderCSV(w io.Writer, s Summary) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"focal_mm", "count", "percent"}); err != nil {
		return err
	}
	for _, e := range s.Entries {
		row := []string{
			strconv.Itoa(e.FocalMM),
			strconv.Itoa(e.Count),
			strconv.FormatFloat(e.Percent, 'f', 2, 64),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

const barWidth = 40

// RenderTable writes s as an aligned terminal table with a histogram.
func RenderTable(w io.Writer, s Summary) error {
	fmt.Fprintf(w, "focalstats — focal-length usage (basis: %s)\n", s.Basis)
	fmt.Fprintf(w, "scanned: %d  with focal: %d  no focal: %d  failed: %d\n\n",
		s.Total, s.WithFocal, s.NoFocal, s.Failed)

	if len(s.Entries) == 0 {
		fmt.Fprintln(w, "(no images with focal-length metadata found)")
		return nil
	}

	maxCount := 0
	for _, e := range s.Entries {
		if e.Count > maxCount {
			maxCount = e.Count
		}
	}

	fmt.Fprintf(w, "  %-8s  %8s  %7s  %s\n", "focal", "count", "share", "distribution")
	fmt.Fprintf(w, "  %-8s  %8s  %7s  %s\n",
		strings.Repeat("-", 8), strings.Repeat("-", 8), strings.Repeat("-", 7), strings.Repeat("-", barWidth))
	for _, e := range s.Entries {
		bar := 0
		if maxCount > 0 {
			bar = e.Count * barWidth / maxCount
		}
		fmt.Fprintf(w, "  %5dmm   %8d  %6.1f%%  %s\n",
			e.FocalMM, e.Count, e.Percent, strings.Repeat("█", bar))
	}
	return nil
}
