// Package upload validates uploaded images (magic bytes, dimensions) and strips
// privacy-sensitive metadata (EXIF/GPS, XMP, text chunks) without re-encoding.
package upload

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register decoders for DecodeConfig
	_ "image/png"
	"net/http"

	_ "golang.org/x/image/webp"
)

var ErrUnsupported = errors.New("unsupported image format")

type Info struct {
	MIME   string
	Ext    string
	Width  int
	Height int
}

var allowed = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// Inspect sniffs the real content type from the bytes (the client-provided
// Content-Type is ignored) and reads the dimensions from the header only.
func Inspect(data []byte) (Info, error) {
	mime := http.DetectContentType(data)
	ext, ok := allowed[mime]
	if !ok {
		return Info{}, ErrUnsupported
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Info{}, fmt.Errorf("%w: empty image", ErrUnsupported)
	}
	return Info{MIME: mime, Ext: ext, Width: cfg.Width, Height: cfg.Height}, nil
}

// StripMetadata removes metadata for the given MIME type. JPEG keeps only the
// EXIF orientation (so photos are not shown sideways); ICC profiles are kept.
func StripMetadata(mime string, data []byte) ([]byte, error) {
	switch mime {
	case "image/jpeg":
		return stripJPEG(data)
	case "image/png":
		return stripPNG(data)
	case "image/webp":
		return stripWebP(data)
	default:
		return nil, ErrUnsupported
	}
}
