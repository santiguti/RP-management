package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrUnsupportedMime = errors.New("unsupported mime type")
	ErrTooLarge        = errors.New("file too large")
	ErrInvalidPath     = errors.New("invalid storage path")
)

const MaxUploadBytes int64 = 10 * 1024 * 1024

var AllowedMimes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type FileStore struct {
	Root string
}

func New(root string) (*FileStore, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &FileStore{Root: root}, nil
}

func (s *FileStore) Save(subdir string, r io.Reader) (relPath, mimeType string, size int64, err error) {
	cleanSubdir, err := cleanRelativePath(subdir)
	if err != nil {
		return "", "", 0, err
	}

	head := make([]byte, 512)
	n, readErr := io.ReadFull(r, head)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return "", "", 0, readErr
	}
	head = head[:n]
	mimeType = http.DetectContentType(head)
	ext, ok := AllowedMimes[mimeType]
	if !ok {
		return "", "", 0, ErrUnsupportedMime
	}

	nameBuf := make([]byte, 16)
	if _, err := rand.Read(nameBuf); err != nil {
		return "", "", 0, err
	}
	relPath = filepath.ToSlash(filepath.Join(cleanSubdir, hex.EncodeToString(nameBuf)+ext))
	absPath, err := s.resolve(relPath)
	if err != nil {
		return "", "", 0, err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		return "", "", 0, err
	}

	f, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", "", 0, err
	}
	defer f.Close()

	if _, err := f.Write(head); err != nil {
		_ = os.Remove(absPath)
		return "", "", 0, err
	}
	remaining := MaxUploadBytes - int64(len(head)) + 1
	n2, err := io.Copy(f, io.LimitReader(r, remaining))
	if err != nil {
		_ = os.Remove(absPath)
		return "", "", 0, err
	}
	size = int64(len(head)) + n2
	if size > MaxUploadBytes {
		_ = os.Remove(absPath)
		return "", "", 0, ErrTooLarge
	}
	return relPath, mimeType, size, nil
}

func (s *FileStore) Open(relPath string) (*os.File, error) {
	absPath, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(absPath)
}

func (s *FileStore) resolve(relPath string) (string, error) {
	clean, err := cleanRelativePath(relPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Root, clean), nil
}

func cleanRelativePath(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", ErrInvalidPath
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	return clean, nil
}
