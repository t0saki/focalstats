package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/t0saki/focalstats/internal/scan"
)

func sampleStats() scan.Stats {
	return scan.Stats{
		Basis:     scan.Basis35mm,
		Round:     1,
		Total:     110,
		WithFocal: 100,
		NoFocal:   6,
		Failed:    4,
		Focal:     map[int]int{35: 40, 24: 10, 50: 50},
	}
}

func TestBuildSortsAscendingAndComputesPercent(t *testing.T) {
	s := Build(sampleStats(), 0)

	if got := len(s.Entries); got != 3 {
		t.Fatalf("entries = %d, want 3", got)
	}
	wantMM := []int{24, 35, 50}
	for i, e := range s.Entries {
		if e.FocalMM != wantMM[i] {
			t.Errorf("entry[%d].FocalMM = %d, want %d", i, e.FocalMM, wantMM[i])
		}
	}
	if got := s.Entries[2].Percent; got != 50.0 {
		t.Errorf("50mm percent = %v, want 50", got)
	}
	if s.WithFocal != 100 || s.NoFocal != 6 || s.Failed != 4 || s.Total != 110 {
		t.Errorf("summary counters mismatch: %+v", s)
	}
	if s.Basis != "35mm-equivalent" {
		t.Errorf("basis label = %q", s.Basis)
	}
}

func TestBuildTopN(t *testing.T) {
	s := Build(sampleStats(), 2)
	if got := len(s.Entries); got != 2 {
		t.Fatalf("entries = %d, want 2 (top-2)", got)
	}
	if s.Entries[0].FocalMM != 35 || s.Entries[1].FocalMM != 50 {
		t.Errorf("top-2 entries = %d,%d; want 35,50", s.Entries[0].FocalMM, s.Entries[1].FocalMM)
	}
}

func TestRenderCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderCSV(&buf, Build(sampleStats(), 0)); err != nil {
		t.Fatalf("RenderCSV: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "focal_mm,count,percent\n") {
		t.Errorf("missing CSV header, got:\n%s", out)
	}
	if !strings.Contains(out, "50,50,50.00") {
		t.Errorf("missing 50mm row, got:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderJSON(&buf, Build(sampleStats(), 0)); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`"basis": "35mm-equivalent"`, `"with_focal": 100`, `"focal_mm": 24`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON missing %q, got:\n%s", want, out)
		}
	}
}

func TestRenderTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderTable(&buf, Build(scan.Stats{Basis: scan.Basis35mm, Focal: map[int]int{}}, 0)); err != nil {
		t.Fatalf("RenderTable: %v", err)
	}
	if !strings.Contains(buf.String(), "no images") {
		t.Errorf("empty table missing notice, got:\n%s", buf.String())
	}
}
