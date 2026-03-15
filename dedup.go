package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const partialHashSize = 64 * 1024 // 64KB

// FileRecord stores metadata for a single indexed file.
type FileRecord struct {
	Path  string
	Size  int64
	Mtime int64
}

// DedupCache is a gob-serialized dedup index stored at <target>/.organize-media.cache.
type DedupCache struct {
	// Hash -> list of files with that hash (partial collisions possible)
	Files map[string][]FileRecord
	// Directories already scanned
	IndexedDirs map[string]bool

	path  string // cache file path
	dirty bool
}

// NewDedupCache loads or creates a dedup cache at <targetDir>/.organize-media.cache.
func NewDedupCache(targetDir string, rebuild bool) (*DedupCache, error) {
	cachePath := filepath.Join(targetDir, ".organize-media.cache")

	if rebuild {
		os.Remove(cachePath)
	}

	c := &DedupCache{
		Files:       make(map[string][]FileRecord),
		IndexedDirs: make(map[string]bool),
		path:        cachePath,
	}

	f, err := os.Open(cachePath)
	if err == nil {
		defer f.Close()
		if err := gob.NewDecoder(f).Decode(c); err != nil {
			// Corrupt cache -- start fresh
			c.Files = make(map[string][]FileRecord)
			c.IndexedDirs = make(map[string]bool)
		}
		c.path = cachePath
	}

	return c, nil
}

// Close writes the cache to disk if dirty.
func (c *DedupCache) Close() error {
	if !c.dirty {
		return nil
	}
	tmp := c.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create cache: %w", err)
	}

	if err := gob.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode cache: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	c.dirty = false
	return os.Rename(tmp, c.path)
}

// EnsureIndexed scans dirPath and adds all media files to the cache.
func (c *DedupCache) EnsureIndexed(dirPath string) error {
	dirPath = filepath.Clean(dirPath)
	if c.IndexedDirs[dirPath] {
		return nil
	}

	// Directory might not exist yet
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		c.IndexedDirs[dirPath] = true
		c.dirty = true
		return nil
	}

	err := filepath.WalkDir(dirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if !isMediaFile(filepath.Ext(entry.Name())) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}

		hash, err := partialHash(path)
		if err != nil {
			return nil
		}

		c.addRecord(hash, FileRecord{
			Path:  path,
			Size:  info.Size(),
			Mtime: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return err
	}

	c.IndexedDirs[dirPath] = true
	c.dirty = true
	return nil
}

func (c *DedupCache) addRecord(hash string, rec FileRecord) {
	for _, existing := range c.Files[hash] {
		if existing.Path == rec.Path {
			return
		}
	}
	c.Files[hash] = append(c.Files[hash], rec)
	c.dirty = true
}

// IsDuplicate checks for an existing file with matching hash and size.
func (c *DedupCache) IsDuplicate(partialH string, size int64, verifyFull bool, sourcePath string) (bool, string, error) {
	records, ok := c.Files[partialH]
	if !ok {
		return false, "", nil
	}

	// Build a clean list, removing stale entries
	clean := records[:0]
	var match string
	found := false

	for _, rec := range records {
		if _, err := os.Stat(rec.Path); os.IsNotExist(err) {
			c.dirty = true
			continue
		}
		clean = append(clean, rec)

		if found || rec.Size != size {
			continue
		}

		if verifyFull {
			srcH, err := fullHash(sourcePath)
			if err != nil {
				continue
			}
			dstH, err := fullHash(rec.Path)
			if err != nil {
				continue
			}
			if srcH != dstH {
				continue
			}
		}

		found = true
		match = rec.Path
	}

	if len(clean) != len(records) {
		c.Files[partialH] = clean
	}

	return found, match, nil
}

// Insert adds a file record to the cache.
func (c *DedupCache) Insert(path, partialH string, size, mtime int64) {
	c.addRecord(partialH, FileRecord{
		Path:  path,
		Size:  size,
		Mtime: mtime,
	})
}

// partialHash computes a SHA256 hash of the first 64KB + last 64KB + file size.
// For files <= 128KB, hashes the entire file.
func partialHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	size := info.Size()
	h := sha256.New()

	// Include file size in hash
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(size))
	h.Write(sizeBuf)

	if size <= 2*partialHashSize {
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
	} else {
		buf := make([]byte, partialHashSize)

		// First 64KB
		n, err := io.ReadFull(f, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return "", err
		}
		h.Write(buf[:n])

		// Last 64KB
		if _, err := f.Seek(-partialHashSize, io.SeekEnd); err != nil {
			return "", err
		}
		n, err = io.ReadFull(f, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return "", err
		}
		h.Write(buf[:n])
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// fullHash computes a full SHA256 hash of the entire file.
func fullHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
