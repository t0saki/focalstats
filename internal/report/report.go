// Package report turns a scan.Result into a human- or machine-readable summary
// (aligned terminal table, JSON, or CSV).
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

// Summary is the full, render-ready statistics object.
type Summary struct {
	Basis     string  `json:"basis"`
	Total     int     `json:"total_images"`
	WithFocal int     `json:"with_focal"`
	NoFocal   int     `json:"no_focal"`
	Failed    int     `json:"failed"`
	Entries   []Entry `json:"distribution"` // sorted by focal length ascending
}

// Build converts a scan.Result into a Summary. basis is a human label for the
// chosen focal basis. If top > 0, only the top-N buckets by count are kept
// (the kept buckets are still presented sorted by focal length).
func Build(res scan.Result, basis string, top int) Summary {
	entries := make([]Entry, 0, len(res.Counts))
	for mm, n := range res.Counts {
		pct := 0.0
		if res.WithFocal > 0 {
			pct = float64(n) / float64(res.WithFocal) * 100
		}
		entries = append(entries, Entry{FocalMM: mm, Count: n, Percent: pct})
	}

	if top > 0 && top < len(entries) {
		// Keep the N most-used buckets.
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Count != entries[j].Count {
				return entries[i].Count > entries[j].Count
			}
			return entries[i].FocalMM < entries[j].FocalMM
		})
		entries = entries[:top]
	}

	// Final presentation order: ascending by focal length.
	sort.Slice(entries, func(i, j int) bool { return entries[i].FocalMM < entries[j].FocalMM })

	return Summary{
		Basis:     basis,
		Total:     res.Total,
		WithFocal: res.WithFocal,
		NoFocal:   res.NoFocal,
		Failed:    res.Failed,
		Entries:   entries,
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
	fmt.Fprintf(w, "focalstats — 焦段使用统计 (基准: %s)\n", s.Basis)
	fmt.Fprintf(w, "扫描图片: %d  含焦段: %d  无焦段: %d  读取失败: %d\n\n",
		s.Total, s.WithFocal, s.NoFocal, s.Failed)

	if len(s.Entries) == 0 {
		fmt.Fprintln(w, "(未找到含焦段信息的图片)")
		return nil
	}

	maxCount := 0
	for _, e := range s.Entries {
		if e.Count > maxCount {
			maxCount = e.Count
		}
	}

	fmt.Fprintf(w, "  %8s  %7s  %7s  %s\n", "焦段(mm)", "数量", "占比", "分布")
	fmt.Fprintf(w, "  %8s  %7s  %7s  %s\n",
		strings.Repeat("-", 8), strings.Repeat("-", 7), strings.Repeat("-", 7), strings.Repeat("-", barWidth))
	for _, e := range s.Entries {
		bar := 0
		if maxCount > 0 {
			bar = e.Count * barWidth / maxCount
		}
		fmt.Fprintf(w, "  %8d  %7d  %6.1f%%  %s\n",
			e.FocalMM, e.Count, e.Percent, strings.Repeat("█", bar))
	}
	return nil
}
