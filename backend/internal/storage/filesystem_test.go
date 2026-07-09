package storage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStore_SavePNG_OK(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pngBytes := testPNG(t)
	rel, mimeType, size, err := store.Save("wo-test", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/png" || size != int64(len(pngBytes)) {
		t.Fatalf("mime/size = %q/%d, want image/png/%d", mimeType, size, len(pngBytes))
	}
	if _, err := os.Stat(filepath.Join(store.Root, rel)); err != nil {
		t.Fatal(err)
	}
}

func TestFileStore_SaveRejectsPDF(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.Save("wo-test", bytes.NewReader([]byte("%PDF-1.4")))
	if !errors.Is(err, ErrUnsupportedMime) {
		t.Fatalf("err = %v, want ErrUnsupportedMime", err)
	}
}

func TestFileStore_SaveTooLarge(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0}, int(MaxUploadBytes))...)
	_, _, _, err = store.Save("wo-test", bytes.NewReader(content))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestFileStore_OpenRoundTrip(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pngBytes := testPNG(t)
	rel, _, _, err := store.Save("wo-test", bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	f, err := store.Open(rel)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Fatal("opened bytes differ from saved bytes")
	}
}

func TestFileStore_Remove(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rel, _, _, err := store.Save("wo-test", bytes.NewReader(testPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Remove(rel); err != nil {
		t.Fatal(err)
	}
	if f, err := store.Open(rel); err == nil {
		_ = f.Close()
		t.Fatal("open removed file succeeded")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestFileStore_OpenRejectsTraversal(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Open("../secret")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("err = %v, want ErrInvalidPath", err)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
