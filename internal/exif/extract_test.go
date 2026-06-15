package exif

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// malformedTIFF builds a big-endian TIFF whose IFD (at offset 18) declares one
// entry but is truncated, which makes imagemeta read past a short buffer and
// panic — exactly the failure seen in the wild on a NAS photo library.
func malformedTIFF() []byte {
	buf := new(bytes.Buffer)
	buf.Write([]byte{'M', 'M', 0x00, 0x2A})         // big-endian TIFF magic
	binary.Write(buf, binary.BigEndian, uint32(18)) // first-IFD offset
	for buf.Len() < 18 {                            // pad to the IFD offset
		buf.WriteByte(0)
	}
	binary.Write(buf, binary.BigEndian, uint16(1)) // IFD entry count = 1
	buf.Write(bytes.Repeat([]byte{0xAB}, 12))      // truncated entry data
	return buf.Bytes()
}

func TestExtractMalformedDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.tif")
	if err := os.WriteFile(p, malformedTIFF(), 0o644); err != nil {
		t.Fatal(err)
	}

	// Must return an error rather than panicking and crashing the scan.
	_, err := Extract(p)
	if err == nil {
		t.Fatal("expected an error for the malformed file, got nil")
	}
}

func TestExtractMissingFile(t *testing.T) {
	if _, err := Extract(filepath.Join(t.TempDir(), "nope.jpg")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
