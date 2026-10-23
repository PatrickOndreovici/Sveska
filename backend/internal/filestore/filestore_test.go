package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) (*Local, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "files")
	s, err := NewLocal(dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return s, dir
}

// filesIn lists every file under dir, relative to it.
func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPutThenOpenReturnsTheBytes(t *testing.T) {
	s, dir := newTestStore(t)
	ctx := context.Background()

	key, err := s.Put(ctx, strings.NewReader("%PDF-1.7 hello"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !validKey(key) {
		t.Errorf("key = %q, want 32 hex characters", key)
	}
	if got := filesIn(t, dir); len(got) != 1 || got[0] != filepath.Join(key[:2], key) {
		t.Errorf("files = %v, want only %s/%s", got, key[:2], key)
	}

	f, err := s.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	body, _ := io.ReadAll(f)
	if string(body) != "%PDF-1.7 hello" {
		t.Errorf("body = %q", body)
	}
}

func TestPutGivesEachFileItsOwnKey(t *testing.T) {
	s, _ := newTestStore(t)
	a, _ := s.Put(context.Background(), strings.NewReader("a"))
	b, _ := s.Put(context.Background(), strings.NewReader("b"))
	if a == b {
		t.Errorf("two files share key %q", a)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestPutLeavesNothingBehindWhenTheReaderFails(t *testing.T) {
	s, dir := newTestStore(t)
	tooLarge := errors.New("too large")

	_, err := s.Put(context.Background(), io.MultiReader(strings.NewReader("%PDF-"), failingReader{tooLarge}))
	if !errors.Is(err, tooLarge) {
		t.Fatalf("err = %v, want the reader's error", err)
	}
	if got := filesIn(t, dir); len(got) != 0 {
		t.Errorf("files left behind: %v", got)
	}
}

func TestOpenRejectsKeysThatAreNotKeys(t *testing.T) {
	s, dir := newTestStore(t)
	// A file a path-traversal key would reach if it were used as a path.
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "../secret", "ab/../../secret", strings.Repeat("A", 32), strings.Repeat("a", 31)} {
		if _, err := s.Open(context.Background(), key); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q) = %v, want ErrNotFound", key, err)
		}
	}
}

func TestOpenMissingFileIsNotFound(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.Open(context.Background(), strings.Repeat("a", 32)); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesTheFileAndCanBeRepeated(t *testing.T) {
	s, dir := newTestStore(t)
	ctx := context.Background()
	key, err := s.Put(ctx, strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := filesIn(t, dir); len(got) != 0 {
		t.Errorf("files after Delete: %v", got)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("second Delete: %v", err)
	}
}
