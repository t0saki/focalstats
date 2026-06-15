// Package exif extracts photographic metadata from image files.
//
// It relies on github.com/evanoberholster/imagemeta, which reads only the
// EXIF/metadata headers (it never decodes the full image), keeping CPU and
// memory usage low and roughly constant regardless of image dimensions.
package exif

import (
	"os"
	"strings"
	"time"

	"github.com/evanoberholster/imagemeta"
)

// Meta holds the photographic metadata read from a single image's EXIF.
// Zero values mean the corresponding tag was absent.
type Meta struct {
	Actual   float64   // FocalLength, in mm (the lens's true focal length)
	Equiv35  float64   // FocalLengthIn35mmFilm, in mm (full-frame equivalent)
	Taken    time.Time // capture time (DateTimeOriginal, falling back to CreateDate)
	Camera   string    // camera body, e.g. "SONY ILCE-7M4"
	Lens     string    // lens model
	FNumber  float64   // aperture f-number
	ISO      int       // ISO sensitivity
	Exposure float64   // exposure (shutter) time, in seconds
}

// HasActual reports whether an actual focal length was present.
func (m Meta) HasActual() bool { return m.Actual > 0 }

// HasEquiv35 reports whether a 35mm-equivalent focal length was present.
func (m Meta) HasEquiv35() bool { return m.Equiv35 > 0 }

// Extract opens path, reads only its EXIF metadata, and returns it. The file is
// closed before returning.
func Extract(path string) (Meta, error) {
	f, err := os.Open(path)
	if err != nil {
		return Meta{}, err
	}
	defer f.Close()

	e, err := imagemeta.Decode(f)
	if err != nil {
		return Meta{}, err
	}

	taken := e.DateTimeOriginal()
	if taken.IsZero() {
		taken = e.CreateDate()
	}

	iso := int(e.ISO)
	if iso == 0 {
		iso = int(e.ISOSpeed)
	}

	lens := strings.TrimSpace(e.LensModel)
	if lens == "" {
		lens = strings.TrimSpace(e.LensMake)
	}

	return Meta{
		Actual:   float64(e.FocalLength),
		Equiv35:  float64(e.FocalLengthIn35mmFormat),
		Taken:    taken,
		Camera:   camera(e.Make, e.Model),
		Lens:     lens,
		FNumber:  float64(e.FNumber),
		ISO:      iso,
		Exposure: float64(e.ExposureTime),
	}, nil
}

// camera combines the EXIF make and model into a single readable label,
// avoiding duplication when the model already begins with the make
// (e.g. make "Canon", model "Canon EOS R6" -> "Canon EOS R6").
func camera(make, model string) string {
	mk := strings.TrimSpace(make)
	md := strings.TrimSpace(model)
	switch {
	case md == "":
		return mk
	case mk == "":
		return md
	case strings.HasPrefix(strings.ToLower(md), strings.ToLower(mk)):
		return md
	default:
		return mk + " " + md
	}
}
