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

	// Prefix format
	got := buildPrefixFilename("photo", dt, ".jpg")
	want := "photo__20260315_143045.jpg"
	if got != want {
		t.Errorf("buildPrefixFilename = %q, want %q", got, want)
	}

	// Default format: YYYYMMDD_HHMMSS_<8hex>.ext
	got = buildDefaultFilename(dt, ".jpg")
	if len(got) != len("20260315_143045_a1b2c3d4.jpg") {
		t.Errorf("buildDefaultFilename length = %d, want %d: %q", len(got), len("20260315_143045_a1b2c3d4.jpg"), got)
	}
	if got[:16] != "20260315_143045_" {
		t.Errorf("buildDefaultFilename prefix = %q, want 20260315_143045_", got[:16])
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

func TestNamingPatterns(t *testing.T) {
	// prefixPattern: prefix__YYYYMMDD_HHMMSS[_N].ext
	prefixTests := []struct {
		name  string
		match bool
	}{
		{"photo__20260315_143045_0.jpg", true},
		{"my-media__20260315_143045_12.mp4", true},
		{"photo__20260315_143045.jpg", true},
		{"photo_20260315_143045_0.jpg", false},  // single underscore
		{"20260315_143045_0.jpg", false},         // no prefix
	}
	for _, tt := range prefixTests {
		if got := prefixPattern.MatchString(tt.name); got != tt.match {
			t.Errorf("prefixPattern.MatchString(%q) = %v, want %v", tt.name, got, tt.match)
		}
	}

	// datetimePattern: YYYYMMDD_HHMMSS[_suffix].ext
	datetimeTests := []struct {
		name  string
		match bool
	}{
		{"20260315_143045.jpg", true},             // bare datetime
		{"20260315_143045_a3f8b1c4.jpg", true},    // datetime + hex
		{"20260315_143045_1.jpg", true},            // datetime + counter
		{"00000000_000000.mp4", true},              // all zeros, still matches pattern
		{"IMG_20260315_143045.jpg", false},         // has non-digit prefix
		{"photo__20260315_143045.jpg", false},      // prefix format, not datetime
		{"Baroque Slumber.png", false},             // no date
		{"12128182.jpeg", false},                   // 8 digits but no underscore+6
	}
	for _, tt := range datetimeTests {
		if got := datetimePattern.MatchString(tt.name); got != tt.match {
			t.Errorf("datetimePattern.MatchString(%q) = %v, want %v", tt.name, got, tt.match)
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
		NoRename:    true,
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
		NoRename:    true,
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

func TestOrganizeAlreadyInPlace(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the directory structure with a file matching datetime pattern
	subdir := filepath.Join(dir, "2026", "03")
	os.MkdirAll(subdir, 0o755)

	f, _ := createTestFile(t, filepath.Join(subdir, "20260315_120000_a1b2c3d4.jpg"), 100)
	f.Close()

	cfg := Config{
		SourceDir:   dir,
		Granularity: "month",
		NoDedup:     true,
	}

	err := organize(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// File should still be at original location (skipped as "already named")
	if _, err := os.Stat(filepath.Join(subdir, "20260315_120000_a1b2c3d4.jpg")); err != nil {
		t.Error("file should still be at original location")
	}

	// Should be exactly 1 file -- no collision copies
	entries, _ := os.ReadDir(subdir)
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected 1 file in subdir, got %d: %v", len(entries), names)
	}
}

func TestOrganizeAlreadyInPlaceNoRename(t *testing.T) {
	dir := t.TempDir()

	// Pre-create with original filename, using NoRename mode
	subdir := filepath.Join(dir, "2026", "03")
	os.MkdirAll(subdir, 0o755)

	f, _ := createTestFile(t, filepath.Join(subdir, "IMG_20260315_120000.jpg"), 100)
	f.Close()

	cfg := Config{
		SourceDir:   dir,
		Granularity: "month",
		NoRename:    true,
		NoDedup:     true,
	}

	err := organize(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// File should still be at original location via SameFile check
	if _, err := os.Stat(filepath.Join(subdir, "IMG_20260315_120000.jpg")); err != nil {
		t.Error("file should still be at original location")
	}

	entries, _ := os.ReadDir(subdir)
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected 1 file in subdir, got %d: %v", len(entries), names)
	}
}

func TestOrganizeDefaultRename(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	f, _ := createTestFile(t, filepath.Join(src, "random_name.jpg"), 100)
	f.Close()

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

	// Source should be gone
	if _, err := os.Stat(filepath.Join(src, "random_name.jpg")); err == nil {
		t.Error("source file should have been moved")
	}

	// Find the renamed file in the target month dir
	entries, _ := os.ReadDir(dst)
	if len(entries) == 0 {
		t.Fatal("expected files in target dir")
	}

	// Walk to find the file
	var found string
	filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		found = info.Name()
		return nil
	})

	if found == "" {
		t.Fatal("no file found in target")
	}

	// Should match YYYYMMDD_HHMMSS_<8hex>.jpg
	if !datetimePattern.MatchString(found) {
		t.Errorf("renamed file %q does not match datetime pattern", found)
	}
	if filepath.Ext(found) != ".jpg" {
		t.Errorf("extension = %q, want .jpg", filepath.Ext(found))
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
				NoRename:    true,
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
