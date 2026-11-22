// Package filestore keeps uploaded files outside the database.
//
// Put stores bytes and hands back an opaque key; callers record the key and
// use it later to open or delete the file. Callers never choose paths, so a
// key from the database cannot point outside the storage directory.
package filestore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// ErrNotFound means no file exists for the key.
var ErrNotFound = errors.New("filestore: file not found")

// Local stores files in a directory on disk, fanned out by the first two
// characters of the key (ab/abcdef...) so no single directory grows huge.
type Local struct {
	dir string
}

// NewLocal returns a store rooted at dir, creating it if needed.
func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create file storage directory: %w", err)
	}
	return &Local{dir: dir}, nil
}

// Put copies r into a new file and returns its key. The file appears under
// its key only once it is complete: if r fails part way, for example because
// the upload was too large, nothing is left behind and r's error is returned
// wrapped.
func (s *Local) Put(ctx context.Context, r io.Reader) (key string, err error) {
	key, err = newKey()
	if err != nil {
		return "", err
	}
	// The temporary file sits in the same directory so the final rename
	// stays on one filesystem and is atomic.
	tmp, err := os.CreateTemp(s.dir, ".upload-*")
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = io.Copy(tmp, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return "", fmt.Errorf("sync file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return "", fmt.Errorf("close file: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	path := s.path(key)
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create file directory: %w", err)
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return "", fmt.Errorf("move file into place: %w", err)
	}
	return key, nil
}

// Open returns the file for reading. The caller must close it.
func (s *Local) Open(_ context.Context, key string) (io.ReadSeekCloser, error) {
	if !validKey(key) {
		return nil, ErrNotFound
	}
	f, err := os.Open(s.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

// Delete removes the file. Deleting a file that does not exist is not an
// error, so a retried delete succeeds.
func (s *Local) Delete(_ context.Context, key string) error {
	if !validKey(key) {
		return nil
	}
	err := os.Remove(s.path(key))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (s *Local) path(key string) string {
	return filepath.Join(s.dir, key[:2], key)
}

// A key is 128 random bits in hex: unguessable and safe as a file name.
var keyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func validKey(key string) bool {
	return keyPattern.MatchString(key)
}

func newKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate file key: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
