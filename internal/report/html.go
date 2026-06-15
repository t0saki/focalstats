package report

import (
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/t0saki/focalstats/internal/scan"
)

//go:embed template.html
var htmlTemplate string

var tmpl = template.Must(template.New("report").Parse(htmlTemplate))

// HTMLOptions configures the HTML report.
type HTMLOptions struct {
	Title string // shown in the header, e.g. the scanned path
	Top   int    // top-N for focal table & gear lists (0 = sensible defaults)
}

const (
	defFocalTable = 30
	defGearTop    = 12
	heatmapRows   = 12
	camCompareTop = 6
)

type camView struct {
	Name  string
	Count int
	Chart template.HTML
}

type htmlData struct {
	Title       string
	Generated   string
	Basis       string
	Total       int
	WithFocal   int
	NoFocal     int
	Failed      int
	WithDate    int
	DateRange   string
	FocalChart  template.HTML
	FocalTable  []Entry
	Heatmap     template.HTML
	HourChart   template.HTML
	WeekChart   template.HTML
	YearChart   template.HTML
	MonthChart  template.HTML
	CamCompare  []camView
	CameraChart template.HTML
	LensChart   template.HTML
	ApChart     template.HTML
	ISOChart    template.HTML
	ShutChart   template.HTML
}

// RenderHTML writes a self-contained, offline HTML report (no external assets).
func RenderHTML(w io.Writer, s scan.Stats, opts HTMLOptions) error {
	focalTop := opts.Top
	if focalTop <= 0 {
		focalTop = defFocalTable
	}
	gearTop := opts.Top
	if gearTop <= 0 {
		gearTop = defGearTop
	}

	d := htmlData{
		Title:       opts.Title,
		Generated:   time.Now().Format("2006-01-02 15:04"),
		Basis:       BasisLabel(s.Basis),
		Total:       s.Total,
		WithFocal:   s.WithFocal,
		NoFocal:     s.NoFocal,
		Failed:      s.Failed,
		WithDate:    s.WithDate,
		FocalChart:  vbars(focalBars(s)),
		FocalTable:  focalEntries(s.Focal, s.WithFocal, focalTop),
		Heatmap:     heatmap(s, heatmapRows),
		HourChart:   vbars(hourBars(s.ByHour)),
		WeekChart:   vbars(weekdayBars(s.ByWeekday)),
		YearChart:   vbars(intKeyBars(s.ByYear, func(k int) string { return strconv.Itoa(k) })),
		MonthChart:  vbars(monthBars(s.ByYearMonth)),
		CameraChart: hbars(topStrBars(s.Cameras, gearTop)),
		LensChart:   hbars(topStrBars(s.Lenses, gearTop)),
		ApChart:     vbars(apertureBars(s.Apertures)),
		ISOChart:    vbars(intKeyBars(s.ISOs, func(k int) string { return strconv.Itoa(k) })),
		ShutChart:   hbars(topStrBars(s.Shutters, gearTop)),
		CamCompare:  cameraCompare(s),
	}
	if s.WithDate > 0 && !s.MinDate.IsZero() {
		d.DateRange = s.MinDate.Format("2006-01-02") + " — " + s.MaxDate.Format("2006-01-02")
	}

	return tmpl.Execute(w, d)
}

// --- bar/series builders ---

type bar struct {
	Label string
	Value int
	Title string
}

func focalBars(s scan.Stats) []bar {
	keys := make([]int, 0, len(s.Focal))
	for k := range s.Focal {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	bars := make([]bar, 0, len(keys))
	for _, k := range keys {
		n := s.Focal[k]
		pct := 0.0
		if s.WithFocal > 0 {
			pct = float64(n) / float64(s.WithFocal) * 100
		}
		bars = append(bars, bar{
			Label: strconv.Itoa(k),
			Value: n,
			Title: fmt.Sprintf("%dmm: %d (%.1f%%)", k, n, pct),
		})
	}
	return bars
}

func hourBars(h [24]int) []bar {
	bars := make([]bar, 24)
	for i := 0; i < 24; i++ {
		bars[i] = bar{
			Label: strconv.Itoa(i),
			Value: h[i],
			Title: fmt.Sprintf("%02d:00–%02d:59: %d", i, i, h[i]),
		}
	}
	return bars
}

func weekdayBars(wd [7]int) []bar {
	// Present Monday-first; scan stores Sunday=0.
	order := []int{1, 2, 3, 4, 5, 6, 0}
	names := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	bars := make([]bar, 7)
	for i, idx := range order {
		bars[i] = bar{Label: names[i], Value: wd[idx], Title: fmt.Sprintf("%s: %d", names[i], wd[idx])}
	}
	return bars
}

func intKeyBars(m map[int]int, label func(int) string) []bar {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	bars := make([]bar, 0, len(keys))
	for _, k := range keys {
		bars = append(bars, bar{Label: label(k), Value: m[k], Title: fmt.Sprintf("%s: %d", label(k), m[k])})
	}
	return bars
}

func monthBars(m map[string]int) []bar {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	bars := make([]bar, 0, len(keys))
	for _, k := range keys {
		bars = append(bars, bar{Label: k, Value: m[k], Title: fmt.Sprintf("%s: %d", k, m[k])})
	}
	return bars
}

func apertureBars(m map[string]int) []bar {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// sort by numeric f-number ascending
	sort.Slice(keys, func(i, j int) bool { return apertureNum(keys[i]) < apertureNum(keys[j]) })
	bars := make([]bar, 0, len(keys))
	for _, k := range keys {
		bars = append(bars, bar{Label: strings.TrimPrefix(k, "f/"), Value: m[k], Title: fmt.Sprintf("%s: %d", k, m[k])})
	}
	return bars
}

func apertureNum(label string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimPrefix(label, "f/"), 64)
	return f
}

func topStrBars(m map[string]int, n int) []bar {
	type kv struct {
		k string
		v int
	}
	items := make([]kv, 0, len(m))
	for k, v := range m {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].v != items[j].v {
			return items[i].v > items[j].v
		}
		return items[i].k < items[j].k
	})
	if n > 0 && n < len(items) {
		items = items[:n]
	}
	bars := make([]bar, 0, len(items))
	for _, it := range items {
		bars = append(bars, bar{Label: it.k, Value: it.v, Title: fmt.Sprintf("%s: %d", it.k, it.v)})
	}
	return bars
}

func cameraCompare(s scan.Stats) []camView {
	type kv struct {
		k string
		v int
	}
	cams := make([]kv, 0, len(s.Cameras))
	for k, v := range s.Cameras {
		if s.FocalByCam[k] != nil {
			cams = append(cams, kv{k, v})
		}
	}
	sort.Slice(cams, func(i, j int) bool {
		if cams[i].v != cams[j].v {
			return cams[i].v > cams[j].v
		}
		return cams[i].k < cams[j].k
	})
	if len(cams) > camCompareTop {
		cams = cams[:camCompareTop]
	}
	views := make([]camView, 0, len(cams))
	for _, c := range cams {
		fm := s.FocalByCam[c.k]
		keys := make([]int, 0, len(fm))
		for k := range fm {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		total := 0
		for _, v := range fm {
			total += v
		}
		bars := make([]bar, 0, len(keys))
		for _, k := range keys {
			pct := 0.0
			if total > 0 {
				pct = float64(fm[k]) / float64(total) * 100
			}
			bars = append(bars, bar{Label: strconv.Itoa(k), Value: fm[k], Title: fmt.Sprintf("%dmm: %d (%.1f%%)", k, fm[k], pct)})
		}
		views = append(views, camView{Name: c.k, Count: c.v, Chart: vbars(bars)})
	}
	return views
}

// --- SVG renderers ---

func esc(s string) string { return html.EscapeString(s) }

// vbars renders a vertical bar chart as inline SVG.
func vbars(bars []bar) template.HTML {
	n := len(bars)
	if n == 0 {
		return ""
	}
	step := 36
	switch {
	case n > 60:
		step = 9
	case n > 24:
		step = 14
	case n > 10:
		step = 26
	}
	barW := step * 7 / 10
	if barW < 3 {
		barW = 3
	}
	const padTop, chartH, padBottom, padL, padR = 16, 180, 34, 8, 8
	maxV := 1
	for _, b := range bars {
		if b.Value > maxV {
			maxV = b.Value
		}
	}
	w := padL + n*step + padR
	h := padTop + chartH + padBottom
	stride := 1
	for n/stride > 16 {
		stride++
	}
	showVal := step >= 24

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg class="chart" width="%d" height="%d" viewBox="0 0 %d %d" role="img">`, w, h, w, h)
	fmt.Fprintf(&sb, `<line class="axis" x1="%d" y1="%d" x2="%d" y2="%d"/>`, padL, padTop+chartH, w-padR, padTop+chartH)
	for i, b := range bars {
		bh := b.Value * chartH / maxV
		x := padL + i*step + (step-barW)/2
		y := padTop + chartH - bh
		fmt.Fprintf(&sb, `<rect class="bar" x="%d" y="%d" width="%d" height="%d"><title>%s</title></rect>`,
			x, y, barW, bh, esc(b.Title))
		if showVal && b.Value > 0 {
			fmt.Fprintf(&sb, `<text class="val" x="%d" y="%d" text-anchor="middle">%d</text>`, x+barW/2, y-3, b.Value)
		}
		if i%stride == 0 {
			fmt.Fprintf(&sb, `<text class="tick" x="%d" y="%d" text-anchor="middle">%s</text>`,
				padL+i*step+step/2, padTop+chartH+14, esc(b.Label))
		}
	}
	sb.WriteString(`</svg>`)
	return template.HTML(sb.String())
}

// hbars renders a horizontal bar chart (good for long text labels).
func hbars(bars []bar) template.HTML {
	n := len(bars)
	if n == 0 {
		return ""
	}
	const W, labelW, rowH, valW = 720, 220, 26, 60
	barMax := W - labelW - valW
	maxV := 1
	for _, b := range bars {
		if b.Value > maxV {
			maxV = b.Value
		}
	}
	h := n*rowH + 6
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg class="chart hbar" width="100%%" viewBox="0 0 %d %d" preserveAspectRatio="xMinYMin meet" role="img">`, W, h)
	for i, b := range bars {
		y := i*rowH + 3
		bw := b.Value * barMax / maxV
		fmt.Fprintf(&sb, `<text class="rlabel" x="%d" y="%d" text-anchor="end">%s</text>`,
			labelW-8, y+rowH/2+3, esc(truncate(b.Label, 30)))
		fmt.Fprintf(&sb, `<rect class="bar" x="%d" y="%d" width="%d" height="%d"><title>%s</title></rect>`,
			labelW, y, bw, rowH-8, esc(b.Title))
		fmt.Fprintf(&sb, `<text class="val" x="%d" y="%d">%d</text>`, labelW+bw+6, y+rowH/2+3, b.Value)
	}
	sb.WriteString(`</svg>`)
	return template.HTML(sb.String())
}

// heatmap renders a focal-length × year grid, opacity scaled by count.
func heatmap(s scan.Stats, maxRows int) template.HTML {
	type fb struct{ focal, total int }
	var rows []fb
	for f, ym := range s.FocalByYear {
		t := 0
		for _, c := range ym {
			t += c
		}
		rows = append(rows, fb{f, t})
	}
	if len(rows) == 0 {
		return ""
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].total > rows[j].total })
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].focal < rows[j].focal })

	yearset := map[int]bool{}
	for _, r := range rows {
		for y := range s.FocalByYear[r.focal] {
			yearset[y] = true
		}
	}
	years := make([]int, 0, len(yearset))
	for y := range yearset {
		years = append(years, y)
	}
	sort.Ints(years)
	if len(years) == 0 {
		return ""
	}
	maxC := 1
	for _, r := range rows {
		for _, c := range s.FocalByYear[r.focal] {
			if c > maxC {
				maxC = c
			}
		}
	}
	const cw, ch, leftW, topH, botH = 30, 24, 60, 6, 22
	w := leftW + len(years)*cw
	h := topH + len(rows)*ch + botH
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg class="chart" width="%d" height="%d" viewBox="0 0 %d %d" role="img">`, w, h, w, h)
	for ri, r := range rows {
		y := topH + ri*ch
		fmt.Fprintf(&sb, `<text class="rlabel" x="%d" y="%d" text-anchor="end">%dmm</text>`, leftW-6, y+ch/2+3, r.focal)
		for ci, yr := range years {
			c := s.FocalByYear[r.focal][yr]
			x := leftW + ci*cw
			op := 0.0
			if c > 0 {
				op = 0.15 + 0.85*float64(c)/float64(maxC)
			}
			fmt.Fprintf(&sb, `<rect class="cell" x="%d" y="%d" width="%d" height="%d" fill="var(--accent)" fill-opacity="%.3f"><title>%dmm · %d: %d</title></rect>`,
				x, y, cw-2, ch-2, op, r.focal, yr, c)
		}
	}
	for ci, yr := range years {
		fmt.Fprintf(&sb, `<text class="tick" x="%d" y="%d" text-anchor="middle">%d</text>`,
			leftW+ci*cw+cw/2, topH+len(rows)*ch+14, yr)
	}
	sb.WriteString(`</svg>`)
	return template.HTML(sb.String())
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
