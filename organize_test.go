package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTargetSubdir(t *testing.T) {
	dt := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		granularity string
		want        string
	}{
		{"year", "2026"},
		{"month", "2026/03"},
		{"day", "2026/03/15"},
	}
	for _, tt := range tests {
		got := targetSubdir(dt, tt.granularity)
		if got != tt.want {
			t.Errorf("targetSubdir(%q) = %q, want %q", tt.granularity, got, tt.want)
		}
	}
}

func TestBuildFilename(t *testing.T) {
	dt := time.Date(2026, 3, 15, 14, 30, 45, 0, time.UTC)
	got := buildFilename("photo", dt, ".jpg")
	want := "photo__20260315_143045.jpg"
	if got != want {
		t.Errorf("buildFilename = %q, want %q", got, want)
	}
}

func TestChoosePrefix(t *testing.T) {
	cfg := Config{Prefix: "media", ImagePrefix: "pic", VideoPrefix: "clip"}

	if got := choosePrefix(MediaImage, cfg); got != "pic" {
		t.Errorf("image prefix = %q, want pic", got)
	}
	if got := choosePrefix(MediaVideo, cfg); got != "clip" {
		t.Errorf("video prefix = %q, want clip", got)
	}
	if got := choosePrefix(MediaUnknown, cfg); got != "media" {
		t.Errorf("unknown prefix = %q, want media", got)
	}

	// Without type-specific prefixes
	cfg2 := Config{Prefix: "all"}
	if got := choosePrefix(MediaImage, cfg2); got != "all" {
		t.Errorf("image prefix = %q, want all", got)
	}
}

func TestResolveCollision(t *testing.T) {
	dir := t.TempDir()

	// Non-existent target: use as-is
	target := filepath.Join(dir, "photo.jpg")
	got, err := resolveCollision(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}

	// Create the file so it exists
	f, _ := createTestFile(t, target, 100)
	f.Close()

	// Collision: should get _1 suffix
	got, err = resolveCollision(target)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "photo_1.jpg")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStrictPattern(t *testing.T) {
	tests := []struct {
		name  string
		match bool
	}{
		{"photo__20260315_143045_0.jpg", true},
		{"my-media__20260315_143045_12.mp4", true},
		{"photo__20260315_143045.jpg", true},    // no counter -- still valid
		{"photo_20260315_143045_0.jpg", false},  // single underscore
		{"20260315_143045_0.jpg", false},         // no prefix
	}
	for _, tt := range tests {
		if got := strictPattern.MatchString(tt.name); got != tt.match {
			t.Errorf("strictPattern.MatchString(%q) = %v, want %v", tt.name, got, tt.match)
		}
	}
}

func TestOrganizeDryRun(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a file with a date in its name
	f, _ := createTestFile(t, filepath.Join(src, "IMG_20260315_120000.jpg"), 100)
	f.Close()

	cfg := Config{
		SourceDir:   src,
		TargetDir:   dst,
		Granularity: "month",
		DryRun:      true,
		NoDedup:     true,
	}

	err := organize(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// File should still be in source (dry run)
	if _, err := os.Stat(filepath.Join(src, "IMG_20260315_120000.jpg")); err != nil {
		t.Error("source file should still exist after dry run")
	}
}

func TestOrganizeMove(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create files with dates in names
	f1, _ := createTestFile(t, filepath.Join(src, "IMG_20260315_120000.jpg"), 100)
	f1.Close()
	f2, _ := createTestFile(t, filepath.Join(src, "VID_20251225_090000.mp4"), 200)
	f2.Close()

	cfg := Config{
		SourceDir:   src,
		TargetDir:   dst,
		Granularity: "month",
		NoDedup:     true,
	}

	err := organize(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Check files moved to correct locations
	expectedPaths := []string{
		filepath.Join(dst, "2026", "03", "IMG_20260315_120000.jpg"),
		filepath.Join(dst, "2025", "12", "VID_20251225_090000.mp4"),
	}
	for _, p := range expectedPaths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file at %s", p)
		}
	}

	// Source files should be gone
	entries, _ := os.ReadDir(src)
	if len(entries) != 0 {
		t.Errorf("source dir should be empty, has %d entries", len(entries))
	}
}

func TestOrganizeWithPrefix(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	f, _ := createTestFile(t, filepath.Join(src, "IMG_20260315_120000.jpg"), 100)
	f.Close()

	cfg := Config{
		SourceDir:   src,
		TargetDir:   dst,
		Granularity: "month",
		Prefix:      "photo",
		NoDedup:     true,
	}

	err := organize(cfg)
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(dst, "2026", "03", "photo__20260315_120000.jpg")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected file at %s", expected)
	}
}

func TestOrganizeWithDedup(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source file
	f1, _ := createTestFile(t, filepath.Join(src, "IMG_20260315_120000.jpg"), 500)
	f1.Close()

	// Create identical file already in target
	targetDir := filepath.Join(dst, "2026", "03")
	os.MkdirAll(targetDir, 0o755)
	f2, _ := createTestFile(t, filepath.Join(targetDir, "existing.jpg"), 500)
	f2.Close()

	cfg := Config{
		SourceDir:   src,
		TargetDir:   dst,
		Granularity: "month",
	}

	err := organize(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Source file should be skipped (duplicate detected)
	if _, err := os.Stat(filepath.Join(src, "IMG_20260315_120000.jpg")); err != nil {
		t.Error("source file should still exist (duplicate skipped)")
	}
}

func TestOrganizeGranularity(t *testing.T) {
	for _, gran := range []string{"year", "month", "day"} {
		t.Run(gran, func(t *testing.T) {
			src := t.TempDir()
			dst := t.TempDir()

			f, _ := createTestFile(t, filepath.Join(src, "IMG_20260315_120000.jpg"), 100)
			f.Close()

			cfg := Config{
				SourceDir:   src,
				TargetDir:   dst,
				Granularity: gran,
				NoDedup:     true,
			}

			err := organize(cfg)
			if err != nil {
				t.Fatal(err)
			}

			var expected string
			switch gran {
			case "year":
				expected = filepath.Join(dst, "2026", "IMG_20260315_120000.jpg")
			case "month":
				expected = filepath.Join(dst, "2026", "03", "IMG_20260315_120000.jpg")
			case "day":
				expected = filepath.Join(dst, "2026", "03", "15", "IMG_20260315_120000.jpg")
			}

			if _, err := os.Stat(expected); err != nil {
				t.Errorf("expected file at %s", expected)
			}
		})
	}
}
