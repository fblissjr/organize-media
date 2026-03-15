package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MediaType classifies a file as image, video, or unknown.
type MediaType int

const (
	MediaImage MediaType = iota
	MediaVideo
	MediaUnknown
)

func (m MediaType) String() string {
	switch m {
	case MediaImage:
		return "image"
	case MediaVideo:
		return "video"
	default:
		return "unknown"
	}
}

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".heic": true, ".heif": true, ".bmp": true, ".tiff": true,
	".webp": true, ".cr2": true, ".nef": true, ".arw": true,
	".dng": true, ".raf": true,
}

var videoExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
	".m4v": true, ".wmv": true, ".flv": true, ".webm": true,
	".mts": true, ".m2ts": true, ".3gp": true,
}

// Action represents a planned file operation.
type Action struct {
	SourcePath  string
	TargetPath  string
	Skip        bool
	SkipReason  string
	Type        MediaType
	PartialHash string
	FileSize    int64
	FileMtime   int64
}

func classifyFile(ext string) MediaType {
	ext = strings.ToLower(ext)
	if imageExtensions[ext] {
		return MediaImage
	}
	if videoExtensions[ext] {
		return MediaVideo
	}
	return MediaUnknown
}

func isMediaFile(ext string) bool {
	return classifyFile(ext) != MediaUnknown
}

// discoverFiles walks root recursively and returns all media file paths.
// Skips hidden files/dirs and symlinks.
func discoverFiles(root string, verbose bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
			}
			return nil
		}

		name := d.Name()

		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip symlinks
		if d.Type()&fs.ModeSymlink != 0 {
			if verbose {
				fmt.Fprintf(os.Stderr, "warning: skipping symlink %s\n", path)
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(name)
		if isMediaFile(ext) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
