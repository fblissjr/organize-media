package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		ext  string
		want MediaType
	}{
		{".jpg", MediaImage},
		{".JPG", MediaImage},
		{".jpeg", MediaImage},
		{".png", MediaImage},
		{".heic", MediaImage},
		{".heif", MediaImage},
		{".cr2", MediaImage},
		{".mp4", MediaVideo},
		{".MOV", MediaVideo},
		{".avi", MediaVideo},
		{".mkv", MediaVideo},
		{".txt", MediaUnknown},
		{".pdf", MediaUnknown},
		{"", MediaUnknown},
	}
	for _, tt := range tests {
		got := classifyFile(tt.ext)
		if got != tt.want {
			t.Errorf("classifyFile(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestIsMediaFile(t *testing.T) {
	if !isMediaFile(".jpg") {
		t.Error("expected .jpg to be media")
	}
	if isMediaFile(".txt") {
		t.Error("expected .txt to not be media")
	}
}

func TestDiscoverFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	touch(t, filepath.Join(dir, "photo.jpg"))
	touch(t, filepath.Join(dir, "video.mp4"))
	touch(t, filepath.Join(dir, "readme.txt"))
	touch(t, filepath.Join(dir, ".hidden.jpg"))

	// Create subdirectory with media
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	touch(t, filepath.Join(sub, "deep.png"))

	// Create hidden directory
	hidden := filepath.Join(dir, ".hidden")
	os.MkdirAll(hidden, 0o755)
	touch(t, filepath.Join(hidden, "secret.jpg"))

	files, err := discoverFiles(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Should find: photo.jpg, video.mp4, sub/deep.png
	// Should skip: readme.txt, .hidden.jpg, .hidden/secret.jpg
	if len(files) != 3 {
		t.Errorf("got %d files, want 3: %v", len(files), files)
	}

	names := make(map[string]bool)
	for _, f := range files {
		names[filepath.Base(f)] = true
	}
	for _, want := range []string{"photo.jpg", "video.mp4", "deep.png"} {
		if !names[want] {
			t.Errorf("missing expected file %s", want)
		}
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}
