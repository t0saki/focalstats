package report

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/t0saki/focalstats/internal/scan"
)

//go:embed template.html
var htmlTemplate string

// dataPlaceholder is replaced with the base64(gzip(JSON)) payload at render time.
const dataPlaceholder = "__DATA_B64__"

// HTMLOptions configures the HTML report.
type HTMLOptions struct {
	Title string // shown in the header, e.g. the scanned path
	Top   int    // default size of "top N" lists in the UI (0 = sensible default)
}

const defTopLists = 15

type htmlCounts struct {
	Total     int `json:"total"`
	WithFocal int `json:"withFocal"`
	NoFocal   int `json:"noFocal"`
	Failed    int `json:"failed"`
	WithDate  int `json:"withDate"`
	Records   int `json:"records"`
}

// htmlPayload is the columnar, dictionary-encoded dataset embedded in the
// report. Parallel arrays E/A/T/C/L/Ap/ISO/S all have length Counts.Records.
type htmlPayload struct {
	Title     string     `json:"title"`
	Generated string     `json:"generated"`
	Basis     string     `json:"basis"` // "35mm" or "actual"
	TopN      int        `json:"topN"`
	Counts    htmlCounts `json:"counts"`
	Cams      []string   `json:"cams"`
	Lenses    []string   `json:"lenses"`
	E         []int      `json:"e"`   // 35mm-equivalent focal length, mm (0 = none)
	A         []int      `json:"a"`   // actual focal length, mm (0 = none)
	T         []int64    `json:"t"`   // capture time, unix seconds wall-clock-as-UTC (0 = none)
	C         []int      `json:"c"`   // camera dictionary index (-1 = none)
	L         []int      `json:"l"`   // lens dictionary index (-1 = none)
	Ap        []int      `json:"ap"`  // f-number * 10 (0 = none)
	ISO       []int      `json:"iso"` // ISO (0 = none)
	S         []int      `json:"s"`   // exposure microseconds (0 = none)
}

func basisKey(b scan.Basis) string {
	if b == scan.BasisActual {
		return "actual"
	}
	return "35mm"
}

// RenderHTML writes a self-contained, offline, interactive HTML report. The
// per-photo records (scan.Stats.Records) are embedded as base64(gzip(JSON)) and
// the bundled JavaScript filters and re-aggregates them entirely in-browser.
func RenderHTML(w io.Writer, s scan.Stats, opts HTMLOptions) error {
	topN := opts.Top
	if topN <= 0 {
		topN = defTopLists
	}

	n := len(s.Records)
	camIdx := map[string]int{}
	lensIdx := map[string]int{}
	p := htmlPayload{
		Title:     opts.Title,
		Generated: time.Now().Format("2006-01-02 15:04"),
		Basis:     basisKey(s.Basis),
		TopN:      topN,
		Counts: htmlCounts{
			Total: s.Total, WithFocal: s.WithFocal, NoFocal: s.NoFocal,
			Failed: s.Failed, WithDate: s.WithDate, Records: n,
		},
		Cams:   []string{},
		Lenses: []string{},
		E:      make([]int, n), A: make([]int, n), T: make([]int64, n),
		C: make([]int, n), L: make([]int, n), Ap: make([]int, n),
		ISO: make([]int, n), S: make([]int, n),
	}
	intern := func(dict *[]string, idx map[string]int, v string) int {
		if v == "" {
			return -1
		}
		if i, ok := idx[v]; ok {
			return i
		}
		i := len(*dict)
		*dict = append(*dict, v)
		idx[v] = i
		return i
	}
	for i, r := range s.Records {
		p.E[i] = r.Equiv
		p.A[i] = r.Actual
		p.T[i] = r.Unix
		p.C[i] = intern(&p.Cams, camIdx, r.Camera)
		p.L[i] = intern(&p.Lenses, lensIdx, r.Lens)
		p.Ap[i] = r.ApX10
		p.ISO[i] = r.ISO
		p.S[i] = r.ExpUS
	}

	var gzBuf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&gzBuf, gzip.BestCompression)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(gz).Encode(p); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	b64 := base64.StdEncoding.EncodeToString(gzBuf.Bytes())

	html := strings.Replace(htmlTemplate, dataPlaceholder, b64, 1)
	_, err = io.WriteString(w, html)
	return err
}
