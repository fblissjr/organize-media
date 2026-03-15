package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bep/imagemeta"
)

const (
	minYear = 1970
	maxYear = 2100
)

// QuickTime epoch: 1904-01-01 00:00:00 UTC
var qtEpoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

var datePatterns = []struct {
	re     *regexp.Regexp
	layout string
}{
	// 2026-03-15T164608 (with optional .NNN suffix)
	{regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{6}`), "2006-01-02T150405"},
	// 20260315_164608
	{regexp.MustCompile(`\d{8}_\d{6}`), "20060102_150405"},
	// 2026-03-15
	{regexp.MustCompile(`\d{4}-\d{2}-\d{2}`), "2006-01-02"},
}

var quickTimeExts = map[string]bool{
	".mp4": true, ".mov": true, ".m4v": true, ".3gp": true,
}

// Map file extensions to imagemeta ImageFormat.
var imageFormatMap = map[string]imagemeta.ImageFormat{
	".jpg":  imagemeta.JPEG,
	".jpeg": imagemeta.JPEG,
	".png":  imagemeta.PNG,
	".tiff": imagemeta.TIFF,
	".webp": imagemeta.WebP,
	".heic": imagemeta.HEIF,
	".heif": imagemeta.HEIF,
	".avif": imagemeta.AVIF,
	".dng":  imagemeta.DNG,
	".cr2":  imagemeta.CR2,
	".nef":  imagemeta.NEF,
	".arw":  imagemeta.ARW,
}

func isReasonableTime(t time.Time) bool {
	return t.Year() >= minYear && t.Year() <= maxYear
}

// extractDate tries multiple strategies to get a creation date for a media file.
// Returns the date and a string indicating the source.
func extractDate(path string, verbose bool) (time.Time, string) {
	ext := strings.ToLower(filepath.Ext(path))

	// 1. Try EXIF via imagemeta (works for JPEG, TIFF, HEIC, PNG, WebP, RAW, etc.)
	if imgFmt, ok := imageFormatMap[ext]; ok {
		if t, err := extractImageMeta(path, imgFmt); err == nil {
			return t, "exif"
		}
	}

	// 2. Try filename regex
	if t, err := extractFromFilename(filepath.Base(path)); err == nil {
		return t, "filename"
	}

	// 3. Try QuickTime mvhd atom (for MP4/MOV)
	if quickTimeExts[ext] {
		if t, err := extractQuickTime(path); err == nil {
			return t, "quicktime"
		}
	}

	// 4. Fall back to file mtime
	if info, err := os.Stat(path); err == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "warning: using mtime for %s\n", path)
		}
		return info.ModTime(), "mtime"
	}

	return time.Now(), "now"
}

func extractImageMeta(path string, format imagemeta.ImageFormat) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	var tags imagemeta.Tags

	_, err = imagemeta.Decode(imagemeta.Options{
		R:           f,
		ImageFormat: format,
		Sources:     imagemeta.EXIF,
		HandleTag: func(tag imagemeta.TagInfo) error {
			tags.Add(tag)
			return nil
		},
	})
	if err != nil {
		return time.Time{}, err
	}

	t, err := tags.GetDateTime()
	if err != nil {
		return time.Time{}, err
	}

	if !isReasonableTime(t) {
		return time.Time{}, fmt.Errorf("unreasonable EXIF year: %d", t.Year())
	}
	return t, nil
}

func extractFromFilename(name string) (time.Time, error) {
	for _, dp := range datePatterns {
		m := dp.re.FindString(name)
		if m == "" {
			continue
		}
		t, err := time.Parse(dp.layout, m)
		if err != nil {
			continue
		}
		if isReasonableTime(t) {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("no date found in filename")
}

func extractQuickTime(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return time.Time{}, err
	}

	return findMvhd(f, info.Size())
}

// findMvhd searches top-level atoms for 'moov', then looks for 'mvhd' inside it.
func findMvhd(r io.ReadSeeker, fileSize int64) (time.Time, error) {
	var pos int64
	for pos < fileSize {
		typ, bodySize, err := readAtomHeader(r)
		if err != nil {
			return time.Time{}, err
		}

		if bodySize < 0 {
			bodySize = fileSize - pos - 8
		}

		switch typ {
		case "moov":
			return findMvhdInContainer(r, bodySize)
		default:
			if _, err := r.Seek(bodySize, io.SeekCurrent); err != nil {
				return time.Time{}, err
			}
		}

		pos += 8 + bodySize
	}
	return time.Time{}, fmt.Errorf("moov atom not found")
}

func findMvhdInContainer(r io.ReadSeeker, containerSize int64) (time.Time, error) {
	var read int64
	for read < containerSize {
		typ, bodySize, err := readAtomHeader(r)
		if err != nil {
			return time.Time{}, err
		}
		if bodySize < 0 {
			bodySize = containerSize - read - 8
		}

		if typ == "mvhd" {
			return parseMvhdCreationTime(r)
		}

		if _, err := r.Seek(bodySize, io.SeekCurrent); err != nil {
			return time.Time{}, err
		}
		read += 8 + bodySize
	}
	return time.Time{}, fmt.Errorf("mvhd atom not found")
}

// readAtomHeader reads a 4-byte size + 4-byte type. Returns type string and body size.
func readAtomHeader(r io.ReadSeeker) (string, int64, error) {
	var size32 uint32
	var atomType [4]byte

	if err := binary.Read(r, binary.BigEndian, &size32); err != nil {
		return "", 0, err
	}
	if _, err := io.ReadFull(r, atomType[:]); err != nil {
		return "", 0, err
	}

	typ := string(atomType[:])

	if size32 == 1 {
		var size64 uint64
		if err := binary.Read(r, binary.BigEndian, &size64); err != nil {
			return "", 0, err
		}
		return typ, int64(size64) - 16, nil
	}

	if size32 == 0 {
		return typ, -1, nil
	}

	if size32 < 8 {
		return "", 0, fmt.Errorf("invalid atom size %d", size32)
	}

	return typ, int64(size32) - 8, nil
}

func parseMvhdCreationTime(r io.Reader) (time.Time, error) {
	var versionAndFlags [4]byte
	if _, err := io.ReadFull(r, versionAndFlags[:]); err != nil {
		return time.Time{}, err
	}
	version := versionAndFlags[0]

	var creationTime uint64
	if version == 0 {
		var ct uint32
		if err := binary.Read(r, binary.BigEndian, &ct); err != nil {
			return time.Time{}, err
		}
		creationTime = uint64(ct)
	} else {
		if err := binary.Read(r, binary.BigEndian, &creationTime); err != nil {
			return time.Time{}, err
		}
	}

	if creationTime == 0 {
		return time.Time{}, fmt.Errorf("creation time is zero")
	}

	t := qtEpoch.Add(time.Duration(creationTime) * time.Second)
	if !isReasonableTime(t) {
		return time.Time{}, fmt.Errorf("unreasonable QuickTime year: %d", t.Year())
	}

	return t, nil
}
