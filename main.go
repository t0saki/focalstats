// Command focalstats scans a directory for photos and reports how often each
// focal length was used, reading only EXIF metadata for speed and low memory.
//
// Usage:
//
//	focalstats [flags] <path>
package main

import (
	"flag"
	"fmt"
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
	top := flag.Int("top", 0, "show only the N most-used focal lengths (0 = all)")
	asJSON := flag.Bool("json", false, "output JSON instead of a table")
	asCSV := flag.Bool("csv", false, "output CSV instead of a table")
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

	if *asJSON && *asCSV {
		fmt.Fprintln(os.Stderr, "error: --json and --csv are mutually exclusive")
		return 2
	}

	var basis scan.Basis
	var basisLabel string
	switch *basisFlag {
	case "35mm":
		basis, basisLabel = scan.Basis35mm, "等效35mm"
	case "actual":
		basis, basisLabel = scan.BasisActual, "实际焦距"
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

	opts := scan.Options{Workers: *workers, Basis: basis, Round: *round}
	if *extsFlag != "" {
		opts.Exts = parseExts(*extsFlag)
	}

	res, err := scan.Scan(root, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: traversal incomplete: %v\n", err)
	}

	summary := report.Build(res, basisLabel, *top)

	switch {
	case *asJSON:
		err = report.RenderJSON(os.Stdout, summary)
	case *asCSV:
		err = report.RenderCSV(os.Stdout, summary)
	default:
		err = report.RenderTable(os.Stdout, summary)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
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
