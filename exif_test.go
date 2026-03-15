package main

import (
	"testing"
	"time"
)

func TestExtractFromFilename(t *testing.T) {
	tests := []struct {
		name    string
		wantY   int
		wantM   time.Month
		wantD   int
		wantErr bool
	}{
		{"IMG_2026-03-15T164608.950.jpg", 2026, time.March, 15, false},
		{"photo_20260315_143000.jpg", 2026, time.March, 15, false},
		{"vacation_2026-03-15_notes.jpg", 2026, time.March, 15, false},
		{"no_date_here.jpg", 0, 0, 0, true},
		{"IMG_1800-01-01.jpg", 0, 0, 0, true}, // too old
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractFromFilename(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Year() != tt.wantY || got.Month() != tt.wantM || got.Day() != tt.wantD {
				t.Errorf("got %v, want %d-%02d-%02d", got, tt.wantY, tt.wantM, tt.wantD)
			}
		})
	}
}

func TestExtractDateMtimeFallback(t *testing.T) {
	// Create a temp file -- extractDate should fall back to mtime
	dir := t.TempDir()
	path := dir + "/noexif.jpg"
	f, _ := createTestFile(t, path, 100)
	f.Close()

	got, source := extractDate(path, false)
	if source != "mtime" {
		t.Errorf("expected mtime source, got %s", source)
	}
	if got.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestExtractDateFilenameMatch(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/IMG_20260315_120000.jpg"
	f, _ := createTestFile(t, path, 100)
	f.Close()

	got, source := extractDate(path, false)
	if source != "filename" {
		t.Errorf("expected filename source, got %s", source)
	}
	if got.Year() != 2026 || got.Month() != time.March || got.Day() != 15 {
		t.Errorf("got %v, want 2026-03-15", got)
	}
}
