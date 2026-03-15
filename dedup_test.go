package main

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestFile(t *testing.T, path string, size int) (*os.File, string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	f.Write(data)
	f.Sync()
	return f, path
}

func TestPartialHash(t *testing.T) {
	dir := t.TempDir()

	// Create two identical files
	f1, p1 := createTestFile(t, filepath.Join(dir, "a.jpg"), 1000)
	f1.Close()
	f2, p2 := createTestFile(t, filepath.Join(dir, "b.jpg"), 1000)
	f2.Close()

	h1, err := partialHash(p1)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := partialHash(p2)
	if err != nil {
		t.Fatal(err)
	}

	if h1 != h2 {
		t.Error("identical files should have same hash")
	}

	// Create a different file
	f3, p3 := createTestFile(t, filepath.Join(dir, "c.jpg"), 1001)
	f3.Close()
	h3, err := partialHash(p3)
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h3 {
		t.Error("different files should have different hashes")
	}
}

func TestPartialHashLargeFile(t *testing.T) {
	dir := t.TempDir()

	// Create file larger than 128KB
	path := filepath.Join(dir, "large.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 200*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	f.Write(data)
	f.Close()

	h, err := partialHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Error("expected non-empty hash")
	}
}

func TestDedupCache(t *testing.T) {
	dir := t.TempDir()

	cache, err := NewDedupCache(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Insert a record
	cache.Insert("/some/file.jpg", "abc123", 1000, 1234567890)

	// Should find duplicate
	isDup, path, err := cache.IsDuplicate("abc123", 1000, false, "")
	if err != nil {
		t.Fatal(err)
	}
	// Won't be found because /some/file.jpg doesn't exist (stale entry removal)
	// That's expected behavior
	if isDup {
		t.Errorf("should not find dup for non-existent path, got %s", path)
	}

	// Create a real file and test
	f1, p1 := createTestFile(t, filepath.Join(dir, "real.jpg"), 500)
	f1.Close()
	h1, _ := partialHash(p1)

	cache.Insert(p1, h1, 500, 1234567890)

	isDup, path, err = cache.IsDuplicate(h1, 500, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !isDup {
		t.Error("expected duplicate")
	}
	if path != p1 {
		t.Errorf("got %s, want %s", path, p1)
	}

	// Save and reload
	cache.Close()
	cache2, err := NewDedupCache(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	isDup, _, err = cache2.IsDuplicate(h1, 500, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !isDup {
		t.Error("expected duplicate after reload")
	}
}

func TestDedupCacheEnsureIndexed(t *testing.T) {
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	os.MkdirAll(mediaDir, 0o755)

	// Create some media files
	f1, _ := createTestFile(t, filepath.Join(mediaDir, "a.jpg"), 100)
	f1.Close()
	f2, _ := createTestFile(t, filepath.Join(mediaDir, "b.png"), 200)
	f2.Close()

	cache, err := NewDedupCache(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	err = cache.EnsureIndexed(mediaDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should have 2 files indexed
	total := 0
	for _, records := range cache.Files {
		total += len(records)
	}
	if total != 2 {
		t.Errorf("expected 2 indexed files, got %d", total)
	}

	// Indexing again should be a no-op
	err = cache.EnsureIndexed(mediaDir)
	if err != nil {
		t.Fatal(err)
	}
}
