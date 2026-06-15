// Package scan walks a directory tree and aggregates focal-length usage from
// the EXIF metadata of the image files it finds.
//
// Traversal uses filepath.WalkDir (cheap, no per-entry stat beyond what the OS
// already returns) and feeds matching paths to a bounded worker pool. Each
// worker keeps a private tally and the tallies are merged once at the end, so
// there is no lock contention on the hot path. At any instant a worker holds a
// single open file and reads only its metadata header, so memory stays bounded
// no matter how large the directory or the individual images are.
package scan

import (
	"io/fs"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/t0saki/focalstats/internal/exif"
)

// Basis selects which focal-length value drives the statistics.
type Basis int

const (
	// Basis35mm buckets by the 35mm-equivalent focal length.
	Basis35mm Basis = iota
	// BasisActual buckets by the lens's true focal length.
	BasisActual
)

// DefaultExts is the built-in set of image extensions (without the dot,
// lower-case) that are scanned: common JPEG/HEIC/TIFF plus widespread RAW
// formats. imagemeta reads these without decoding the pixels.
var DefaultExts = []string{
	"jpg", "jpeg", "heic", "heif", "tif", "tiff",
	"dng", "cr2", "cr3", "nef", "nrw", "arw", "sr2", "srf",
	"rw2", "raf", "orf", "srw", "pef", "rwl", "3fr", "iiq", "dcr",
}

// DefaultExtSet returns DefaultExts as a lookup set.
func DefaultExtSet() map[string]bool {
	m := make(map[string]bool, len(DefaultExts))
	for _, e := range DefaultExts {
		m[e] = true
	}
	return m
}

// Options configures a Scan.
type Options struct {
	Workers int             // parallel workers; <=0 means runtime.NumCPU()
	Basis   Basis           // which focal value to bucket by
	Round   int             // bucket step in mm; <=0 means 1
	Exts    map[string]bool // extension set; nil means DefaultExtSet()
}

// Result is the aggregated outcome of a Scan.
type Result struct {
	Counts    map[int]int // rounded focal length (mm) -> number of photos
	Total     int         // image files matched by extension
	WithFocal int         // files that yielded a focal value for the chosen basis
	NoFocal   int         // files read OK but lacking the chosen focal tag
	Failed    int         // files that could not be read/parsed
}

// Scan walks root and returns focal-length statistics under opts.
//
// I/O and parse errors on individual files are counted (Result.Failed) rather
// than aborting the walk. A non-nil error is only returned for a failure of the
// directory traversal itself.
func Scan(root string, opts Options) (Result, error) {
	if opts.Workers <= 0 {
		opts.Workers = runtime.NumCPU()
	}
	if opts.Round <= 0 {
		opts.Round = 1
	}
	exts := opts.Exts
	if exts == nil {
		exts = DefaultExtSet()
	}

	type local struct {
		counts    map[int]int
		withFocal int
		noFocal   int
		failed    int
	}
	locals := make([]local, opts.Workers)

	paths := make(chan string, 256)
	var wg sync.WaitGroup
	for i := range locals {
		locals[i].counts = make(map[int]int)
		wg.Add(1)
		go func(l *local) {
			defer wg.Done()
			for p := range paths {
				f, err := exif.Extract(p)
				if err != nil {
					l.failed++
					continue
				}
				v := f.Equiv35
				if opts.Basis == BasisActual {
					v = f.Actual
				}
				if v <= 0 {
					l.noFocal++
					continue
				}
				l.counts[roundTo(v, opts.Round)]++
				l.withFocal++
			}
		}(&locals[i])
	}

	total := 0
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries, keep walking
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if !exts[ext] {
			return nil
		}
		total++
		paths <- path
		return nil
	})
	close(paths)
	wg.Wait()

	res := Result{Counts: make(map[int]int), Total: total}
	for i := range locals {
		for k, v := range locals[i].counts {
			res.Counts[k] += v
		}
		res.WithFocal += locals[i].withFocal
		res.NoFocal += locals[i].noFocal
		res.Failed += locals[i].failed
	}
	return res, walkErr
}

// roundTo rounds v (mm) to the nearest multiple of step.
func roundTo(v float64, step int) int {
	if step <= 1 {
		return int(math.Round(v))
	}
	return int(math.Round(v/float64(step))) * step
}
