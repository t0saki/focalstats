// Package exif extracts focal-length metadata from image files.
//
// It relies on github.com/evanoberholster/imagemeta, which reads only the
// EXIF/metadata headers (it never decodes the full image), keeping CPU and
// memory usage low and roughly constant regardless of image dimensions.
package exif

import (
	"os"

	"github.com/evanoberholster/imagemeta"
)

// Focal holds the focal-length values read from a single image's EXIF.
// A value of 0 means the corresponding tag was absent.
type Focal struct {
	Actual  float64 // FocalLength, in mm (the lens's true focal length)
	Equiv35 float64 // FocalLengthIn35mmFilm, in mm (full-frame equivalent)
}

// HasActual reports whether an actual focal length was present.
func (f Focal) HasActual() bool { return f.Actual > 0 }

// HasEquiv35 reports whether a 35mm-equivalent focal length was present.
func (f Focal) HasEquiv35() bool { return f.Equiv35 > 0 }

// Extract opens path, reads only its EXIF metadata, and returns the focal
// lengths it contains. The file is closed before returning.
func Extract(path string) (Focal, error) {
	f, err := os.Open(path)
	if err != nil {
		return Focal{}, err
	}
	defer f.Close()

	e, err := imagemeta.Decode(f)
	if err != nil {
		return Focal{}, err
	}

	return Focal{
		Actual:  float64(e.FocalLength),
		Equiv35: float64(e.FocalLengthIn35mmFormat),
	}, nil
}
