// Command focalstats scans a directory for photos and reports how often each
// focal length was used, reading only EXIF metadata for speed and low memory.
//
// Usage:
//
//	focalstats [flags] <path>
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/t0saki/focalstats/internal/report"
	"github.com/t0saki/focalstats/internal/scan"
)

func main() {
	os.Exit(run())
}

func run() int {
	basisFlag := flag.String("basis", "35mm", "focal-length basis: 35mm | actual")
	workers := flag.Int("workers", 0, "parallel workers (0 = number of CPUs)")
	round := flag.Int("round", 1, "bucket rounding step in mm")
	top := flag.Int("top", 0, "limit focal table & gear lists to N entries (0 = sensible default)")
	asJSON := flag.Bool("json", false, "output JSON")
	asCSV := flag.Bool("csv", false, "output CSV")
	asHTML := flag.Bool("html", false, "output a self-contained HTML report")
	out := flag.String("o", "", "write output to this file instead of stdout")
	extsFlag := flag.String("ext", "", "comma-separated extensions to scan (overrides the built-in set)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "focalstats — count focal-length usage from photo EXIF\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  %s [flags] <path>\n\nFlags:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "error: exactly one <path> argument is required")
		flag.Usage()
		return 2
	}
	root := flag.Arg(0)

	if countTrue(*asJSON, *asCSV, *asHTML) > 1 {
		fmt.Fprintln(os.Stderr, "error: --json, --csv and --html are mutually exclusive")
		return 2
	}

	var basis scan.Basis
	switch *basisFlag {
	case "35mm":
		basis = scan.Basis35mm
	case "actual":
		basis = scan.BasisActual
	default:
		fmt.Fprintf(os.Stderr, "error: invalid --basis %q (want 35mm or actual)\n", *basisFlag)
		return 2
	}

	info, err := os.Stat(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: %s is not a directory\n", root)
		return 1
	}

	opts := scan.Options{Workers: *workers, Basis: basis, Round: *round, Collect: *asHTML}
	if *extsFlag != "" {
		opts.Exts = parseExts(*extsFlag)
	}

	stats, err := scan.Scan(root, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: traversal incomplete: %v\n", err)
	}

	w, closeOut, err := output(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	bw := bufio.NewWriter(w)

	switch {
	case *asHTML:
		err = report.RenderHTML(bw, stats, report.HTMLOptions{Title: root, Top: *top})
	case *asJSON:
		err = report.RenderJSON(bw, report.Build(stats, *top))
	case *asCSV:
		err = report.RenderCSV(bw, report.Build(stats, *top))
	default:
		err = report.RenderTable(bw, report.Build(stats, *top))
	}
	if ferr := bw.Flush(); err == nil {
		err = ferr
	}
	if cerr := closeOut(); err == nil {
		err = cerr
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// output returns the destination writer plus a close function. When path is
// empty it writes to stdout (whose close is a no-op).
func output(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func countTrue(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

// parseExts turns a comma-separated list into a lower-case extension set,
// tolerating leading dots and surrounding whitespace.
func parseExts(s string) map[string]bool {
	m := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		e := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), ".")))
		if e != "" {
			m[e] = true
		}
	}
	return m
}
