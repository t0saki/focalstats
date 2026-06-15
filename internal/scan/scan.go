// Package scan walks a directory tree and aggregates photographic statistics
// from the EXIF metadata of the image files it finds.
//
// Traversal uses filepath.WalkDir (cheap, no per-entry stat beyond what the OS
// already returns) and feeds matching paths to a bounded worker pool. Each
// worker keeps a private Stats tally and the tallies are merged once at the end,
// so there is no lock contention on the hot path. At any instant a worker holds
// a single open file and reads only its metadata header, so memory stays bounded
// no matter how large the directory or the individual images are. The aggregates
// are histograms keyed by low-cardinality values (focal length, hour, camera,
// …), so memory is bounded by the number of distinct values, not photo count.
package scan

import (
	"io/fs"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

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

// Stats is the aggregated outcome of a Scan. Every histogram counts photos.
type Stats struct {
	Basis Basis
	Round int

	Total     int // image files matched by extension
	WithFocal int // files that yielded a focal value for the chosen basis
	NoFocal   int // files read OK but lacking the chosen focal tag
	Failed    int // files that could not be read/parsed
	WithDate  int // files with a usable capture time

	MinDate time.Time
	MaxDate time.Time

	Focal       map[int]int            // rounded focal length (mm) -> count
	ByHour      [24]int                // capture hour 0..23 -> count
	ByWeekday   [7]int                 // Sunday=0 .. Saturday=6 -> count
	ByYear      map[int]int            // calendar year -> count
	ByYearMonth map[string]int         // "YYYY-MM" -> count
	Cameras     map[string]int         // camera body -> count
	Lenses      map[string]int         // lens model -> count
	Apertures   map[string]int         // "f/1.8" -> count
	ISOs        map[int]int            // ISO -> count
	Shutters    map[string]int         // "1/250" / "2s" -> count
	FocalByYear map[int]map[int]int    // focal bucket -> year -> count (heatmap)
	FocalByCam  map[string]map[int]int // camera -> focal bucket -> count (per-body comparison)
}

func newStats(basis Basis, round int) *Stats {
	if round <= 0 {
		round = 1
	}
	return &Stats{
		Basis:       basis,
		Round:       round,
		Focal:       map[int]int{},
		ByYear:      map[int]int{},
		ByYearMonth: map[string]int{},
		Cameras:     map[string]int{},
		Lenses:      map[string]int{},
		Apertures:   map[string]int{},
		ISOs:        map[int]int{},
		Shutters:    map[string]int{},
		FocalByYear: map[int]map[int]int{},
		FocalByCam:  map[string]map[int]int{},
	}
}

// add folds a single image's metadata into the tally.
func (s *Stats) add(m exif.Meta) {
	v := m.Equiv35
	if s.Basis == BasisActual {
		v = m.Actual
	}
	haveFocal := v > 0
	bucket := 0
	if haveFocal {
		bucket = roundTo(v, s.Round)
		s.Focal[bucket]++
		s.WithFocal++
	} else {
		s.NoFocal++
	}

	if !m.Taken.IsZero() {
		t := m.Taken
		s.WithDate++
		s.ByHour[t.Hour()]++
		s.ByWeekday[int(t.Weekday())]++
		y := t.Year()
		s.ByYear[y]++
		s.ByYearMonth[t.Format("2006-01")]++
		if s.MinDate.IsZero() || t.Before(s.MinDate) {
			s.MinDate = t
		}
		if s.MaxDate.IsZero() || t.After(s.MaxDate) {
			s.MaxDate = t
		}
		if haveFocal {
			if s.FocalByYear[bucket] == nil {
				s.FocalByYear[bucket] = map[int]int{}
			}
			s.FocalByYear[bucket][y]++
		}
	}

	if m.Camera != "" {
		s.Cameras[m.Camera]++
		if haveFocal {
			if s.FocalByCam[m.Camera] == nil {
				s.FocalByCam[m.Camera] = map[int]int{}
			}
			s.FocalByCam[m.Camera][bucket]++
		}
	}
	if m.Lens != "" {
		s.Lenses[m.Lens]++
	}
	if m.FNumber > 0 {
		s.Apertures[apertureLabel(m.FNumber)]++
	}
	if m.ISO > 0 {
		s.ISOs[m.ISO]++
	}
	if m.Exposure > 0 {
		s.Shutters[shutterLabel(m.Exposure)]++
	}
}

// merge folds another (worker-local) tally into s.
func (s *Stats) merge(o *Stats) {
	s.WithFocal += o.WithFocal
	s.NoFocal += o.NoFocal
	s.Failed += o.Failed
	s.WithDate += o.WithDate
	for i := range s.ByHour {
		s.ByHour[i] += o.ByHour[i]
	}
	for i := range s.ByWeekday {
		s.ByWeekday[i] += o.ByWeekday[i]
	}
	mergeMap(s.Focal, o.Focal)
	mergeMap(s.ByYear, o.ByYear)
	mergeMap(s.ByYearMonth, o.ByYearMonth)
	mergeMap(s.Cameras, o.Cameras)
	mergeMap(s.Lenses, o.Lenses)
	mergeMap(s.Apertures, o.Apertures)
	mergeMap(s.ISOs, o.ISOs)
	mergeMap(s.Shutters, o.Shutters)
	for fb, ym := range o.FocalByYear {
		if s.FocalByYear[fb] == nil {
			s.FocalByYear[fb] = map[int]int{}
		}
		mergeMap(s.FocalByYear[fb], ym)
	}
	for cam, fm := range o.FocalByCam {
		if s.FocalByCam[cam] == nil {
			s.FocalByCam[cam] = map[int]int{}
		}
		mergeMap(s.FocalByCam[cam], fm)
	}
	if !o.MinDate.IsZero() && (s.MinDate.IsZero() || o.MinDate.Before(s.MinDate)) {
		s.MinDate = o.MinDate
	}
	if !o.MaxDate.IsZero() && (s.MaxDate.IsZero() || o.MaxDate.After(s.MaxDate)) {
		s.MaxDate = o.MaxDate
	}
}

func mergeMap[K comparable](dst, src map[K]int) {
	for k, v := range src {
		dst[k] += v
	}
}

// Scan walks root and returns photographic statistics under opts.
//
// I/O and parse errors on individual files are counted (Stats.Failed) rather
// than aborting the walk. A non-nil error is only returned for a failure of the
// directory traversal itself.
func Scan(root string, opts Options) (Stats, error) {
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	round := opts.Round
	if round <= 0 {
		round = 1
	}
	exts := opts.Exts
	if exts == nil {
		exts = DefaultExtSet()
	}

	locals := make([]*Stats, workers)
	paths := make(chan string, 256)
	var wg sync.WaitGroup
	for i := range locals {
		locals[i] = newStats(opts.Basis, round)
		wg.Add(1)
		go func(l *Stats) {
			defer wg.Done()
			for p := range paths {
				m, err := exif.Extract(p)
				if err != nil {
					l.Failed++
					continue
				}
				l.add(m)
			}
		}(locals[i])
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

	res := newStats(opts.Basis, round)
	res.Total = total
	for _, l := range locals {
		res.merge(l)
	}
	return *res, walkErr
}

// roundTo rounds v (mm) to the nearest multiple of step.
func roundTo(v float64, step int) int {
	if step <= 1 {
		return int(math.Round(v))
	}
	return int(math.Round(v/float64(step))) * step
}

// apertureLabel formats an f-number, e.g. 1.8 -> "f/1.8", 8.0 -> "f/8".
func apertureLabel(f float64) string {
	return "f/" + trimFloat(f)
}

// shutterLabel formats an exposure time in seconds, e.g. 0.004 -> "1/250",
// 2.0 -> "2s".
func shutterLabel(sec float64) string {
	if sec >= 1 {
		return trimFloat(sec) + "s"
	}
	inv := int(math.Round(1 / sec))
	if inv < 1 {
		inv = 1
	}
	return "1/" + strconv.Itoa(inv)
}

func trimFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}
