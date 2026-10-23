package identity

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"learning-serbian-platform/backend/internal/apperr"
	"learning-serbian-platform/backend/internal/store"
)

// fakeProfileStore keeps one user and their picture in memory.
type fakeProfileStore struct {
	user   store.UpdateDisplayNameRow
	avatar *store.GetAvatarRow
	now    time.Time
}

func newFakeProfileStore() *fakeProfileStore {
	return &fakeProfileStore{
		user: store.UpdateDisplayNameRow{ID: "u1", Email: "ana@example.com", DisplayName: "Ana", Role: store.UserRoleStudent},
		now:  time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}
}

func (f *fakeProfileStore) UpdateDisplayName(_ context.Context, arg store.UpdateDisplayNameParams) (store.UpdateDisplayNameRow, error) {
	if arg.ID != f.user.ID {
		return store.UpdateDisplayNameRow{}, pgx.ErrNoRows
	}
	f.user.DisplayName = arg.DisplayName
	if f.avatar != nil {
		f.user.AvatarUpdatedAt = &f.avatar.UpdatedAt
	}
	return f.user, nil
}

func (f *fakeProfileStore) SaveAvatar(_ context.Context, arg store.SaveAvatarParams) (time.Time, error) {
	f.avatar = &store.GetAvatarRow{Image: arg.Image, UpdatedAt: f.now}
	return f.now, nil
}

func (f *fakeProfileStore) GetAvatar(_ context.Context, userID string) (store.GetAvatarRow, error) {
	if userID != f.user.ID || f.avatar == nil {
		return store.GetAvatarRow{}, pgx.ErrNoRows
	}
	return *f.avatar, nil
}

func (f *fakeProfileStore) DeleteAvatar(_ context.Context, _ string) error {
	f.avatar = nil
	return nil
}

func fieldProblem(t *testing.T, err error, field string) string {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.Invalid {
		t.Fatalf("err = %v, want a validation error", err)
	}
	return appErr.Fields[field]
}

func TestUpdateDisplayName(t *testing.T) {
	svc := NewProfileService(newFakeProfileStore())

	user, err := svc.UpdateDisplayName(context.Background(), "u1", "  Ana Petrović ")
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "Ana Petrović" || user.Email != "ana@example.com" || user.Role != RoleStudent {
		t.Errorf("user = %+v", user)
	}
}

func TestUpdateDisplayNameRejectsInvalidNames(t *testing.T) {
	svc := NewProfileService(newFakeProfileStore())
	for name, want := range map[string]string{
		"   ":                   "Enter your name.",
		strings.Repeat("a", 81): "Use at most 80 characters.",
		"Ana\nPetrović":         "Remove line breaks and control characters.",
	} {
		_, err := svc.UpdateDisplayName(context.Background(), "u1", name)
		if got := fieldProblem(t, err, "displayName"); got != want {
			t.Errorf("%q: problem = %q, want %q", name, got, want)
		}
	}
}

func TestUpdateDisplayNameForAGoneUser(t *testing.T) {
	svc := NewProfileService(newFakeProfileStore())
	_, err := svc.UpdateDisplayName(context.Background(), "someone-else", "Ana")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.Unauthenticated {
		t.Errorf("err = %v, want unauthenticated", err)
	}
}

// halves is a w×h image whose left half is red and right half blue, so
// which way it was turned shows in the result.
func halves(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 255, A: 255}
			if x >= w/2 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withOrientation adds an EXIF segment recording orientation o to a JPEG,
// in the byte order a camera chose.
func withOrientation(jpg []byte, o uint16, order binary.AppendByteOrder) []byte {
	tiff := make([]byte, 0, 26)
	if order == binary.LittleEndian {
		tiff = append(tiff, 'I', 'I')
	} else {
		tiff = append(tiff, 'M', 'M')
	}
	tiff = order.AppendUint16(tiff, 42)
	tiff = order.AppendUint32(tiff, 8) // the first directory follows the header
	tiff = order.AppendUint16(tiff, 2) // two entries: a width, then the orientation
	tiff = order.AppendUint16(tiff, 0x0100)
	tiff = order.AppendUint16(tiff, 4)
	tiff = order.AppendUint32(tiff, 1)
	tiff = order.AppendUint32(tiff, 64)
	tiff = order.AppendUint16(tiff, 0x0112)
	tiff = order.AppendUint16(tiff, 3)
	tiff = order.AppendUint32(tiff, 1)
	tiff = order.AppendUint16(tiff, o)
	tiff = order.AppendUint16(tiff, 0)
	tiff = order.AppendUint32(tiff, 0) // no next directory

	segment := append([]byte("Exif\x00\x00"), tiff...)
	out := []byte{0xFF, 0xD8, 0xFF, 0xE1}
	out = binary.BigEndian.AppendUint16(out, uint16(len(segment)+2))
	out = append(out, segment...)
	return append(out, jpg[2:]...)
}

func decodeAvatar(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" {
		t.Errorf("format = %s, want jpeg", format)
	}
	return img
}

// colourAt names the colour at a point: "red", "blue" or "white".
func colourAt(img image.Image, x, y int) string {
	r, g, b, _ := img.At(x, y).RGBA()
	switch {
	case r > 0xc000 && g > 0xc000 && b > 0xc000:
		return "white"
	case r > 0xc000 && g < 0x4000 && b < 0x4000:
		return "red"
	case b > 0xc000 && r < 0x4000 && g < 0x4000:
		return "blue"
	}
	return "other"
}

func TestSaveAvatarCropsAndScalesToASquare(t *testing.T) {
	fake := newFakeProfileStore()
	svc := NewProfileService(fake)
	user := User{ID: "u1", DisplayName: "Ana"}

	// 1200×600: the centred square is x 300–900, half red and half blue.
	saved, err := svc.SaveAvatar(context.Background(), user, bytes.NewReader(encodeJPEG(t, halves(1200, 600))))
	if err != nil {
		t.Fatal(err)
	}
	if saved.AvatarUpdatedAt == nil || !saved.AvatarUpdatedAt.Equal(fake.now) || saved.DisplayName != "Ana" {
		t.Errorf("user = %+v", saved)
	}
	img := decodeAvatar(t, fake.avatar.Image)
	if b := img.Bounds(); b.Dx() != avatarSide || b.Dy() != avatarSide {
		t.Fatalf("size = %v, want %d×%d", b.Size(), avatarSide, avatarSide)
	}
	if left, right := colourAt(img, 40, 128), colourAt(img, 216, 128); left != "red" || right != "blue" {
		t.Errorf("left %s, right %s; want red and blue halves", left, right)
	}
}

func TestSaveAvatarKeepsASmallPictureSmall(t *testing.T) {
	fake := newFakeProfileStore()
	svc := NewProfileService(fake)
	if _, err := svc.SaveAvatar(context.Background(), User{ID: "u1"}, bytes.NewReader(encodeJPEG(t, halves(100, 80)))); err != nil {
		t.Fatal(err)
	}
	if b := decodeAvatar(t, fake.avatar.Image).Bounds(); b.Dx() != 80 || b.Dy() != 80 {
		t.Errorf("size = %v, want 80×80", b.Size())
	}
}

func TestSaveAvatarTurnsPhotosUpright(t *testing.T) {
	jpg := encodeJPEG(t, halves(64, 64))
	// Where the red half (stored on the left) shows after turning.
	tests := []struct {
		orientation uint16
		redAt       image.Point
		blueAt      image.Point
	}{
		{1, image.Pt(10, 32), image.Pt(54, 32)},
		{2, image.Pt(54, 32), image.Pt(10, 32)}, // mirrored: red on the right
		{3, image.Pt(54, 32), image.Pt(10, 32)}, // upside down
		{6, image.Pt(32, 10), image.Pt(32, 54)}, // turned right: the left side goes to the top
		{8, image.Pt(32, 54), image.Pt(32, 10)}, // turned left: the left side goes to the bottom
		{5, image.Pt(32, 10), image.Pt(32, 54)}, // transposed: the left column becomes the top row
		{7, image.Pt(32, 54), image.Pt(32, 10)},
	}
	for _, tt := range tests {
		for _, order := range []binary.AppendByteOrder{binary.BigEndian, binary.LittleEndian} {
			fake := newFakeProfileStore()
			svc := NewProfileService(fake)
			if _, err := svc.SaveAvatar(context.Background(), User{ID: "u1"}, bytes.NewReader(withOrientation(jpg, tt.orientation, order))); err != nil {
				t.Fatal(err)
			}
			img := decodeAvatar(t, fake.avatar.Image)
			if got := colourAt(img, tt.redAt.X, tt.redAt.Y); got != "red" {
				t.Errorf("orientation %d (%v): at %v %s, want red", tt.orientation, order, tt.redAt, got)
			}
			if got := colourAt(img, tt.blueAt.X, tt.blueAt.Y); got != "blue" {
				t.Errorf("orientation %d (%v): at %v %s, want blue", tt.orientation, order, tt.blueAt, got)
			}
		}
	}
}

func TestExifOrientationIgnoresBrokenData(t *testing.T) {
	jpg := encodeJPEG(t, halves(8, 8))
	broken := withOrientation(jpg, 6, binary.BigEndian)
	broken[4], broken[5] = 0xFF, 0xFF // a segment length past the end of the file
	for name, data := range map[string][]byte{
		"no EXIF":          jpg,
		"not a JPEG":       []byte("GIF89a"),
		"empty":            nil,
		"orientation 9":    withOrientation(jpg, 9, binary.BigEndian),
		"segment too long": broken,
	} {
		if got := exifOrientation(data); got != 1 {
			t.Errorf("%s: orientation = %d, want 1", name, got)
		}
	}
}

func TestSaveAvatarMakesTransparencyWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20)) // fully transparent
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	fake := newFakeProfileStore()
	if _, err := NewProfileService(fake).SaveAvatar(context.Background(), User{ID: "u1"}, &buf); err != nil {
		t.Fatal(err)
	}
	if got := colourAt(decodeAvatar(t, fake.avatar.Image), 10, 10); got != "white" {
		t.Errorf("colour = %s, want white", got)
	}
}

func TestSaveAvatarAcceptsGIF(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 10, 10), color.Palette{color.RGBA{R: 255, A: 255}})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	fake := newFakeProfileStore()
	if _, err := NewProfileService(fake).SaveAvatar(context.Background(), User{ID: "u1"}, &buf); err != nil {
		t.Fatal(err)
	}
	if got := colourAt(decodeAvatar(t, fake.avatar.Image), 5, 5); got != "red" {
		t.Errorf("colour = %s, want red", got)
	}
}

func TestSaveAvatarRejectsWhatIsNotAPicture(t *testing.T) {
	svc := NewProfileService(newFakeProfileStore())
	for name, data := range map[string][]byte{
		"empty": nil,
		"text":  []byte("hello"),
		"pdf":   []byte("%PDF-1.7\n"),
	} {
		_, err := svc.SaveAvatar(context.Background(), User{ID: "u1"}, bytes.NewReader(data))
		if got := fieldProblem(t, err, "file"); got != "Choose a JPEG, PNG, GIF or WebP picture." {
			t.Errorf("%s: problem = %q", name, got)
		}
	}
}

func TestSaveAvatarRejectsHugeImages(t *testing.T) {
	// A PNG header claiming 10000×10000 pixels: refused before decoding.
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	binary.BigEndian.PutUint32(data[16:], 10000) // IHDR width
	binary.BigEndian.PutUint32(data[20:], 10000) // IHDR height
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))

	_, err := NewProfileService(newFakeProfileStore()).SaveAvatar(context.Background(), User{ID: "u1"}, bytes.NewReader(data))
	if got := fieldProblem(t, err, "file"); !strings.Contains(got, "50 megapixels") {
		t.Errorf("problem = %q", got)
	}
}

func TestSaveAvatarRejectsLargeFiles(t *testing.T) {
	fake := newFakeProfileStore()
	_, err := NewProfileService(fake).SaveAvatar(context.Background(), User{ID: "u1"}, bytes.NewReader(make([]byte, MaxAvatarBytes+1)))
	if !errors.Is(err, ErrAvatarTooLarge) {
		t.Errorf("err = %v, want ErrAvatarTooLarge", err)
	}
	if fake.avatar != nil {
		t.Error("a picture was saved")
	}
}

func TestAvatarAndDeleteAvatar(t *testing.T) {
	fake := newFakeProfileStore()
	svc := NewProfileService(fake)
	ctx := context.Background()

	if _, err := svc.Avatar(ctx, "u1"); !isNotFound(err) {
		t.Errorf("before saving: err = %v, want not found", err)
	}
	if _, err := svc.SaveAvatar(ctx, User{ID: "u1"}, bytes.NewReader(encodeJPEG(t, halves(10, 10)))); err != nil {
		t.Fatal(err)
	}
	avatar, err := svc.Avatar(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(avatar.Image) == 0 || !avatar.UpdatedAt.Equal(fake.now) {
		t.Errorf("avatar = %d bytes at %v", len(avatar.Image), avatar.UpdatedAt)
	}

	if err := svc.DeleteAvatar(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Avatar(ctx, "u1"); !isNotFound(err) {
		t.Errorf("after deleting: err = %v, want not found", err)
	}
	if err := svc.DeleteAvatar(ctx, "u1"); err != nil {
		t.Errorf("deleting again: %v", err)
	}
}

func isNotFound(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Kind == apperr.NotFound
}
