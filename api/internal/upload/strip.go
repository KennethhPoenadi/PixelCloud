package upload

import (
	"encoding/binary"
	"errors"
)

var errMalformed = errors.New("malformed image data")

// --- JPEG -------------------------------------------------------------------

const (
	jpegSOI  = 0xD8
	jpegEOI  = 0xD9
	jpegSOS  = 0xDA
	jpegAPP0 = 0xE0
	jpegAPP1 = 0xE1 // EXIF / XMP
	jpegAPP2 = 0xE2 // ICC profile (kept for correct colours)
	jpegAPPE = 0xEE // Adobe (kept, affects colour transform)
	jpegCOM  = 0xFE
)

func stripJPEG(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != jpegSOI {
		return nil, errMalformed
	}
	out := make([]byte, 0, len(data))
	out = append(out, 0xFF, jpegSOI)
	insertAt := len(out) // EXIF goes after SOI, or after APP0/JFIF when present
	orientation := 0

	i := 2
	for i < len(data) {
		if data[i] != 0xFF {
			return nil, errMalformed
		}
		for i < len(data) && data[i] == 0xFF { // fill bytes
			i++
		}
		if i >= len(data) {
			return nil, errMalformed
		}
		marker := data[i]
		i++
		if marker == jpegEOI || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
			out = append(out, 0xFF, marker)
			if marker == jpegEOI {
				return withOrientation(out, insertAt, orientation), nil
			}
			continue
		}
		if i+2 > len(data) {
			return nil, errMalformed
		}
		segLen := int(binary.BigEndian.Uint16(data[i:]))
		if segLen < 2 || i+segLen > len(data) {
			return nil, errMalformed
		}
		payload := data[i+2 : i+segLen]
		segment := data[i : i+segLen]

		if marker == jpegSOS {
			// Everything from the first scan on is image data; copy verbatim.
			out = append(out, 0xFF, marker)
			out = append(out, data[i:]...)
			return withOrientation(out, insertAt, orientation), nil
		}

		i += segLen
		isAPP := marker >= jpegAPP0 && marker <= 0xEF
		switch {
		case marker == jpegAPP1:
			if o := exifOrientation(payload); o > 0 && orientation == 0 {
				orientation = o
			}
			continue
		case marker == jpegCOM, isAPP && marker != jpegAPP0 && marker != jpegAPP2 && marker != jpegAPPE:
			continue
		}
		out = append(out, 0xFF, marker)
		out = append(out, segment...)
		if marker == jpegAPP0 && insertAt == 2 {
			insertAt = len(out)
		}
	}
	return nil, errMalformed
}

// withOrientation inserts a minimal EXIF segment holding only the orientation tag.
func withOrientation(out []byte, at, orientation int) []byte {
	if orientation <= 1 {
		return out
	}
	seg := minimalExif(orientation)
	res := make([]byte, 0, len(out)+len(seg))
	res = append(res, out[:at]...)
	res = append(res, seg...)
	return append(res, out[at:]...)
}

func minimalExif(orientation int) []byte {
	tiff := []byte{
		'M', 'M', 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08, // big-endian header, IFD0 at offset 8
		0x00, 0x01, // one entry
		0x01, 0x12, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01, // Orientation, SHORT, count 1
		0x00, byte(orientation), 0x00, 0x00, // value
		0x00, 0x00, 0x00, 0x00, // no next IFD
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, jpegAPP1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}

// exifOrientation extracts tag 0x0112 from IFD0 of an APP1 EXIF payload.
func exifOrientation(p []byte) int {
	if len(p) < 14 || string(p[:6]) != "Exif\x00\x00" {
		return 0
	}
	t := p[6:]
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	if bo.Uint16(t[2:4]) != 42 {
		return 0
	}
	off := uint64(bo.Uint32(t[4:8]))
	if off+2 > uint64(len(t)) {
		return 0
	}
	n := uint64(bo.Uint16(t[off:]))
	for k := range n {
		e := off + 2 + 12*k
		if e+12 > uint64(len(t)) {
			return 0
		}
		if bo.Uint16(t[e:]) != 0x0112 {
			continue
		}
		if bo.Uint16(t[e+2:]) != 3 { // SHORT
			return 0
		}
		v := int(bo.Uint16(t[e+8:]))
		if v >= 1 && v <= 8 {
			return v
		}
		return 0
	}
	return 0
}

// --- PNG --------------------------------------------------------------------

const pngSignature = "\x89PNG\r\n\x1a\n"

var pngDropChunks = map[string]bool{"eXIf": true, "tEXt": true, "zTXt": true, "iTXt": true, "tIME": true}

func stripPNG(data []byte) ([]byte, error) {
	if len(data) < 8 || string(data[:8]) != pngSignature {
		return nil, errMalformed
	}
	out := make([]byte, 0, len(data))
	out = append(out, data[:8]...)
	i := 8
	for i+12 <= len(data) {
		n := uint64(binary.BigEndian.Uint32(data[i:]))
		typ := string(data[i+4 : i+8])
		end := uint64(i) + 12 + n
		if end > uint64(len(data)) {
			return nil, errMalformed
		}
		if !pngDropChunks[typ] {
			out = append(out, data[i:end]...)
		}
		i = int(end)
		if typ == "IEND" {
			return out, nil
		}
	}
	return nil, errMalformed
}

// --- WebP -------------------------------------------------------------------

const (
	vp8xFlagEXIF = 0x08
	vp8xFlagXMP  = 0x04
)

func stripWebP(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, errMalformed
	}
	limit := min(uint64(binary.LittleEndian.Uint32(data[4:]))+8, uint64(len(data)))
	out := make([]byte, 0, len(data))
	out = append(out, data[:12]...)
	i := uint64(12)
	for i+8 <= limit {
		fourcc := string(data[i : i+4])
		size := uint64(binary.LittleEndian.Uint32(data[i+4:]))
		end := min(i+8+size+size&1, limit) // chunks are padded to an even size
		if i+8+size > limit {
			return nil, errMalformed
		}
		switch fourcc {
		case "EXIF", "XMP ":
		case "VP8X":
			start := len(out)
			out = append(out, data[i:end]...)
			if size > 0 {
				out[start+8] &^= vp8xFlagEXIF | vp8xFlagXMP
			}
		default:
			out = append(out, data[i:end]...)
		}
		i = end
	}
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return out, nil
}
