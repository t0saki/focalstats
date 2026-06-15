package report

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/t0saki/focalstats/internal/scan"
)

func richStats() scan.Stats {
	return scan.Stats{
		Basis:     scan.Basis35mm,
		Total:     6,
		WithFocal: 4,
		NoFocal:   1,
		Failed:    1,
		WithDate:  3,
		Records: []scan.Record{
			{Equiv: 52, Actual: 35, Unix: 1614589200, Camera: "Acme <Cam> & Co", Lens: "24-70mm", ApX10: 28, ISO: 100, ExpUS: 4000},
			{Equiv: 26, Actual: 9, Unix: 1614675600, Camera: "Phone X", Lens: "", ApX10: 18, ISO: 400, ExpUS: 16667},
			{Equiv: 52, Actual: 35, Unix: 1614762000, Camera: "Acme <Cam> & Co", Lens: "24-70mm", ApX10: 80, ISO: 200, ExpUS: 2000000},
			{Equiv: 70, Actual: 50, Camera: "Acme <Cam> & Co", Lens: "50mm", ApX10: 14, ISO: 100},
		},
	}
}

// extract the base64(gzip(JSON)) payload the template embeds and decode it.
func decodePayload(t *testing.T, html string) htmlPayload {
	t.Helper()
	m := regexp.MustCompile(`const RAW = "([A-Za-z0-9+/=]+)";`).FindStringSubmatch(html)
	if m == nil {
		t.Fatal("could not find embedded RAW payload")
	}
	gzBytes, err := base64.StdEncoding.DecodeString(m[1])
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(gzBytes))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	var p htmlPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("json: %v", err)
	}
	return p
}

func TestRenderHTMLPayload(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderHTML(&buf, richStats(), HTMLOptions{Title: "/data", Top: 0}); err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("not an HTML document")
	}
	// raw EXIF strings must never appear literally outside the compressed blob.
	if strings.Contains(html, "Acme <Cam> & Co") {
		t.Error("raw camera label leaked into HTML")
	}
	// filter UI controls present.
	for _, id := range []string{`id="filterBody"`, `id="reset"`, `id="matched"`, "DecompressionStream"} {
		if !strings.Contains(html, id) {
			t.Errorf("missing UI element/feature %q", id)
		}
	}

	p := decodePayload(t, html)
	if p.Counts.Records != 4 {
		t.Errorf("records = %d, want 4", p.Counts.Records)
	}
	if len(p.E) != 4 || len(p.T) != 4 || len(p.C) != 4 || len(p.S) != 4 {
		t.Fatalf("columnar arrays not all length 4: %+v", p)
	}
	if p.Basis != "35mm" {
		t.Errorf("basis = %q", p.Basis)
	}
	// dictionary-encoded camera, with the metachar name preserved in JSON (not HTML).
	if len(p.Cams) != 2 {
		t.Fatalf("cams = %v, want 2 entries", p.Cams)
	}
	if p.Cams[p.C[0]] != "Acme <Cam> & Co" {
		t.Errorf("camera dict mismatch: %q", p.Cams[p.C[0]])
	}
	// fourth record has no date -> Unix 0; lens index -1 only when empty (record 2 lens empty).
	if p.T[3] != 0 {
		t.Errorf("record 4 should have no date, got T=%d", p.T[3])
	}
	if p.L[1] != -1 {
		t.Errorf("record 2 lens should be -1 (empty), got %d", p.L[1])
	}
	if p.Ap[0] != 28 || p.ISO[1] != 400 || p.S[2] != 2000000 {
		t.Errorf("columnar values mismatch: ap0=%d iso1=%d s2=%d", p.Ap[0], p.ISO[1], p.S[2])
	}
}

func TestRenderHTMLEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderHTML(&buf, scan.Stats{Basis: scan.Basis35mm}, HTMLOptions{}); err != nil {
		t.Fatalf("RenderHTML empty: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("empty report should still be valid HTML")
	}
	p := decodePayload(t, html)
	if p.Counts.Records != 0 || len(p.E) != 0 {
		t.Errorf("empty payload expected, got records=%d len(e)=%d", p.Counts.Records, len(p.E))
	}
}
