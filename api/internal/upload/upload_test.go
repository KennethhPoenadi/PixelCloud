package upload

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInspect(t *testing.T) {
	cases := []struct {
		file, mime string
	}{
		{"with_exif.jpg", "image/jpeg"},
		{"with_text.png", "image/png"},
		{"with_exif.webp", "image/webp"},
	}
	for _, c := range cases {
		info, err := Inspect(fixture(t, c.file))
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if info.MIME != c.mime || info.Width != 40 || info.Height != 20 {
			t.Fatalf("%s: got %+v", c.file, info)
		}
	}
	if _, err := Inspect(fixture(t, "not_image.txt")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("text file: err = %v, want ErrUnsupported", err)
	}
	// Right magic bytes, broken body.
	broken := append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0)
	if _, err := Inspect(broken); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("broken png: err = %v", err)
	}
}

var secrets = []string{"SecretCam", "SecretXMP", "SecretComment", "SecretText"}

func assertClean(t *testing.T, name string, out []byte) {
	t.Helper()
	for _, s := range secrets {
		if bytes.Contains(out, []byte(s)) {
			t.Fatalf("%s: stripped output still contains %q", name, s)
		}
	}
	if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("%s: stripped output does not decode: %v", name, err)
	}
}

func TestStripJPEGKeepsOrientationOnly(t *testing.T) {
	in := fixture(t, "with_exif.jpg")
	if !bytes.Contains(in, []byte("SecretCam")) {
		t.Fatal("fixture should contain EXIF make")
	}
	out, err := StripMetadata("image/jpeg", in)
	if err != nil {
		t.Fatal(err)
	}
	assertClean(t, "jpeg", out)

	// The rebuilt EXIF must carry orientation 6 (rotate 90° CW).
	idx := bytes.Index(out, []byte("Exif\x00\x00"))
	if idx < 0 {
		t.Fatal("expected minimal EXIF segment with orientation")
	}
	if got := exifOrientation(out[idx:]); got != 6 {
		t.Fatalf("orientation = %d, want 6", got)
	}
	if len(out) >= len(in) {
		t.Fatalf("expected smaller output, got %d >= %d", len(out), len(in))
	}
}

func TestStripJPEGWithoutMetadataIsUnchangedImage(t *testing.T) {
	in := fixture(t, "plain.jpg")
	out, err := StripMetadata("image/jpeg", in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) {
		t.Fatal("no EXIF should be added when orientation is absent")
	}
	assertClean(t, "plain", out)
}

func TestStripPNG(t *testing.T) {
	out, err := StripMetadata("image/png", fixture(t, "with_text.png"))
	if err != nil {
		t.Fatal(err)
	}
	assertClean(t, "png", out)
	if bytes.Contains(out, []byte("eXIf")) {
		t.Fatal("eXIf chunk must be removed")
	}
}

func TestStripWebP(t *testing.T) {
	out, err := StripMetadata("image/webp", fixture(t, "with_exif.webp"))
	if err != nil {
		t.Fatal(err)
	}
	assertClean(t, "webp", out)
	if bytes.Contains(out, []byte("EXIF")) || bytes.Contains(out, []byte("XMP ")) {
		t.Fatal("EXIF/XMP chunks must be removed")
	}
}

func TestStripRejectsGarbage(t *testing.T) {
	for _, mime := range []string{"image/jpeg", "image/png", "image/webp"} {
		if _, err := StripMetadata(mime, []byte("garbage")); err == nil {
			t.Fatalf("%s: expected error for garbage input", mime)
		}
	}
	// Truncated JPEG: segment length points past the end.
	if _, err := StripMetadata("image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}); err == nil {
		t.Fatal("expected error for truncated jpeg")
	}
}
