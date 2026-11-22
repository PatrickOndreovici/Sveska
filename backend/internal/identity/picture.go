package identity

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif" // registers the formats a picture may be uploaded in
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// MaxAvatarBytes is the largest picture accepted. A phone photo is a
	// few megabytes.
	MaxAvatarBytes = 10 << 20
	// maxAvatarPixels bounds the memory spent decoding, whatever the file
	// size: a small file can describe a huge image.
	maxAvatarPixels = 50_000_000
	// avatarSide is the stored picture's width and height, sharp enough for
	// the largest place it is shown (128px) on a 2x screen.
	avatarSide    = 256
	avatarQuality = 85
)

// makeAvatar turns an uploaded image into the stored picture: its largest
// centred square, turned upright, at most avatarSide wide, as a JPEG.
// Re-encoding also drops whatever else the file carried, such as the
// place a photo was taken.
func makeAvatar(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return nil, errNotAPicture
	}
	if cfg.Width*cfg.Height > maxAvatarPixels {
		return nil, errPictureTooBig
	}
	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errNotAPicture
	}

	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(b.Min).Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))
	out := min(side, avatarSide)
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	// A JPEG has no transparency, so transparent parts become white, not black.
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Over, nil)
	if format == "jpeg" {
		dst = orient(dst, exifOrientation(data))
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: avatarQuality}); err != nil {
		return nil, fmt.Errorf("encode avatar: %w", err)
	}
	return buf.Bytes(), nil
}

// orient turns a square image upright, given the EXIF orientation the
// camera recorded: 1 is already upright, 6 means it was held turned right,
// and so on. Image decoders keep the pixels as stored, so a phone photo
// would otherwise show on its side.
func orient(img *image.RGBA, orientation int) *image.RGBA {
	if orientation < 2 || orientation > 8 {
		return img
	}
	n := img.Bounds().Dx()
	out := image.NewRGBA(img.Bounds())
	for y := range n {
		for x := range n {
			// (sx, sy) is the stored pixel that shows at (x, y).
			sx, sy := x, y
			switch orientation {
			case 2: // mirrored
				sx = n - 1 - x
			case 3: // upside down
				sx, sy = n-1-x, n-1-y
			case 4: // upside down and mirrored
				sy = n - 1 - y
			case 5: // mirrored along the diagonal
				sx, sy = y, x
			case 6: // turned a quarter left: turn it right
				sx, sy = y, n-1-x
			case 7: // mirrored along the other diagonal
				sx, sy = n-1-y, n-1-x
			case 8: // turned a quarter right: turn it left
				sx, sy = n-1-y, x
			}
			out.SetRGBA(x, y, img.RGBAAt(sx, sy))
		}
	}
	return out
}

// exifOrientation returns the orientation recorded in a JPEG's EXIF data,
// 1 to 8, or 1 when there is none.
//
// A JPEG is a series of segments: 0xFF, a marker byte, and a big-endian
// length that counts itself. EXIF is the APP1 segment that starts with
// "Exif\0\0"; the metadata segments all come before the image data (SOS).
func exifOrientation(jpg []byte) int {
	if len(jpg) < 2 || jpg[0] != 0xFF || jpg[1] != 0xD8 {
		return 1
	}
	r := jpg[2:]
	for len(r) >= 4 && r[0] == 0xFF {
		marker := r[1]
		if marker == 0xDA {
			return 1
		}
		n := int(binary.BigEndian.Uint16(r[2:4]))
		if n < 2 || len(r) < 2+n {
			return 1
		}
		segment := r[4 : 2+n]
		if marker == 0xE1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			return tiffOrientation(segment[6:])
		}
		r = r[2+n:]
	}
	return 1
}

// tiffOrientation reads tag 0x0112 from the first directory of EXIF's TIFF
// structure: a byte-order mark ("II" little-endian, "MM" big-endian), the
// directory's offset, then its 12-byte entries of tag, type, count and value.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	dir := int(order.Uint32(t[4:8]))
	if dir < 8 || dir+2 > len(t) {
		return 1
	}
	count := int(order.Uint16(t[dir:]))
	for i := range count {
		e := dir + 2 + 12*i
		if e+12 > len(t) {
			return 1
		}
		const orientationTag, shortType = 0x0112, 3
		if order.Uint16(t[e:]) != orientationTag {
			continue
		}
		if o := int(order.Uint16(t[e+8:])); order.Uint16(t[e+2:]) == shortType && o >= 1 && o <= 8 {
			return o
		}
		return 1
	}
	return 1
}
