package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Config holds all CLI options for the organize pipeline.
type Config struct {
	SourceDir   string
	TargetDir   string
	Granularity string
	Prefix      string
	ImagePrefix string
	VideoPrefix string
	DryRun      bool
	Force       bool
	NoDedup     bool
	VerifyFull  bool
	RebuildDB   bool
	Verbose     bool
}

// strictPattern matches the rename pattern: prefix__YYYYMMDD_HHMMSS[_N].ext
var strictPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+__\d{8}_\d{6}(_\d+)?\.[^.]+$`)

// organize runs the two-pass pipeline.
func organize(cfg Config) error {
	sourceDir, err := filepath.Abs(cfg.SourceDir)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	targetDir := sourceDir
	if cfg.TargetDir != "" {
		targetDir, err = filepath.Abs(cfg.TargetDir)
		if err != nil {
			return fmt.Errorf("resolve target: %w", err)
		}
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	// Discover source files
	files, err := discoverFiles(sourceDir, cfg.Verbose)
	if err != nil {
		return fmt.Errorf("discover files: %w", err)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no media files found")
		return nil
	}
	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "found %d media files\n", len(files))
	}

	// Open dedup cache (unless disabled)
	var dedup *DedupCache
	if !cfg.NoDedup {
		dedup, err = NewDedupCache(targetDir, cfg.RebuildDB)
		if err != nil {
			return fmt.Errorf("open dedup cache: %w", err)
		}
		defer dedup.Close()
	}

	// Pass 1: Scan and plan
	actions := make([]Action, 0, len(files))
	for _, path := range files {
		a := planFile(path, targetDir, cfg, dedup)
		actions = append(actions, a)
	}

	// Pass 2: Execute
	return execute(actions, targetDir, cfg, dedup)
}

func planFile(path, targetDir string, cfg Config, dedup *DedupCache) Action {
	ext := filepath.Ext(path)
	mtype := classifyFile(ext)
	name := filepath.Base(path)

	info, err := os.Stat(path)
	if err != nil {
		return Action{SourcePath: path, Skip: true, SkipReason: fmt.Sprintf("stat: %v", err)}
	}

	a := Action{
		SourcePath: path,
		Type:       mtype,
		FileSize:   info.Size(),
		FileMtime:  info.ModTime().Unix(),
	}

	// Check if already matches strict naming pattern (skip unless --force)
	prefix := choosePrefix(mtype, cfg)
	if prefix != "" && strictPattern.MatchString(name) && !cfg.Force {
		a.Skip = true
		a.SkipReason = "already named"
		return a
	}

	// Extract date
	date, dateSource := extractDate(path, cfg.Verbose)
	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "  %s: date=%s source=%s\n", name, date.Format("2006-01-02 15:04:05"), dateSource)
	}

	// Compute target subdir once
	subdir := targetSubdir(date, cfg.Granularity)
	targetDirFull := filepath.Join(targetDir, subdir)

	// Compute partial hash for dedup
	if dedup != nil {
		hash, err := partialHash(path)
		if err != nil {
			a.Skip = true
			a.SkipReason = fmt.Sprintf("hash: %v", err)
			return a
		}
		a.PartialHash = hash

		// Index target subtree
		if err := dedup.EnsureIndexed(targetDirFull); err != nil && cfg.Verbose {
			fmt.Fprintf(os.Stderr, "warning: index %s: %v\n", targetDirFull, err)
		}

		// Check for duplicates
		isDup, existingPath, err := dedup.IsDuplicate(hash, info.Size(), cfg.VerifyFull, path)
		if err != nil && cfg.Verbose {
			fmt.Fprintf(os.Stderr, "warning: dedup check: %v\n", err)
		}
		if isDup {
			a.Skip = true
			a.SkipReason = fmt.Sprintf("duplicate of %s", existingPath)
			return a
		}
	}

	// Compute target path
	var targetName string
	if prefix != "" {
		targetName = buildFilename(prefix, date, strings.ToLower(ext))
	} else {
		targetName = name
	}

	targetPath := filepath.Join(targetDirFull, targetName)

	// Skip if source == target (already in the right place)
	if path == targetPath {
		a.Skip = true
		a.SkipReason = "already in place"
		return a
	}

	// Handle collisions
	targetPath, err = resolveCollision(targetPath)
	if err != nil {
		a.Skip = true
		a.SkipReason = fmt.Sprintf("collision: %v", err)
		return a
	}

	a.TargetPath = targetPath
	return a
}

func choosePrefix(mtype MediaType, cfg Config) string {
	switch mtype {
	case MediaImage:
		if cfg.ImagePrefix != "" {
			return cfg.ImagePrefix
		}
	case MediaVideo:
		if cfg.VideoPrefix != "" {
			return cfg.VideoPrefix
		}
	}
	return cfg.Prefix
}

func targetSubdir(t time.Time, granularity string) string {
	switch granularity {
	case "year":
		return fmt.Sprintf("%d", t.Year())
	case "day":
		return fmt.Sprintf("%d/%02d/%02d", t.Year(), t.Month(), t.Day())
	default: // "month"
		return fmt.Sprintf("%d/%02d", t.Year(), t.Month())
	}
}

func buildFilename(prefix string, t time.Time, ext string) string {
	return fmt.Sprintf("%s__%s%s", prefix, t.Format("20060102_150405"), ext)
}

// resolveCollision finds a unique filename by appending _N if the target exists.
func resolveCollision(targetPath string) (string, error) {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath, nil
	}

	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for i := 1; i <= 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("exhausted 1000 collision attempts for %s", targetPath)
}

func execute(actions []Action, targetDir string, cfg Config, dedup *DedupCache) error {
	// Collect unique target directories
	dirs := make(map[string]bool)
	for _, a := range actions {
		if !a.Skip && a.TargetPath != "" {
			dirs[filepath.Dir(a.TargetPath)] = true
		}
	}

	// Dry-run mode
	if cfg.DryRun {
		var moved, skipped int
		for _, a := range actions {
			if a.Skip {
				skipped++
				if cfg.Verbose {
					fmt.Fprintf(os.Stderr, "  skip: %s (%s)\n", filepath.Base(a.SourcePath), a.SkipReason)
				}
				continue
			}
			moved++
			rel, _ := filepath.Rel(targetDir, a.TargetPath)
			fmt.Printf("%s -> %s\n", a.SourcePath, rel)
		}
		fmt.Fprintf(os.Stderr, "\ndry-run: %d would move, %d would skip\n", moved, skipped)
		return nil
	}

	// Create directories
	for dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// Detect cross-device
	crossDevice := false
	if len(actions) > 0 {
		crossDevice = !sameDevice(sourceOf(actions), targetDir)
	}

	var moved, skipped, errCount int
	for _, a := range actions {
		if a.Skip {
			skipped++
			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "  skip: %s (%s)\n", filepath.Base(a.SourcePath), a.SkipReason)
			}
			continue
		}

		var err error
		if crossDevice {
			err = crossDeviceMove(a.SourcePath, a.TargetPath)
		} else {
			err = os.Rename(a.SourcePath, a.TargetPath)
		}

		if err != nil {
			errCount++
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", filepath.Base(a.SourcePath), err)
			continue
		}

		moved++
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "  move: %s -> %s\n", filepath.Base(a.SourcePath), a.TargetPath)
		}

		if dedup != nil && a.PartialHash != "" {
			dedup.Insert(a.TargetPath, a.PartialHash, a.FileSize, a.FileMtime)
		}
	}

	if dedup != nil {
		dedup.Close()
	}

	fmt.Fprintf(os.Stderr, "\n%d moved, %d skipped, %d errors\n", moved, skipped, errCount)
	return nil
}

// sourceOf returns the directory of the first non-skipped action's source, for device comparison.
func sourceOf(actions []Action) string {
	for _, a := range actions {
		if !a.Skip {
			return filepath.Dir(a.SourcePath)
		}
	}
	return actions[0].SourcePath
}

func sameDevice(path1, path2 string) bool {
	var stat1, stat2 syscall.Stat_t
	if err := syscall.Stat(path1, &stat1); err != nil {
		return false
	}
	if err := syscall.Stat(path2, &stat2); err != nil {
		return false
	}
	return stat1.Dev == stat2.Dev
}

func crossDeviceMove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		// Only close if not already closed by the success path
		out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return err
	}

	if err := out.Sync(); err != nil {
		os.Remove(dst)
		return err
	}

	// Close explicitly to check for write errors before verifying
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}

	// Verify size
	srcInfo, err := os.Stat(src)
	if err != nil {
		os.Remove(dst)
		return err
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		os.Remove(dst)
		return err
	}
	if srcInfo.Size() != dstInfo.Size() {
		os.Remove(dst)
		return fmt.Errorf("size mismatch after copy: %d != %d", srcInfo.Size(), dstInfo.Size())
	}

	// Preserve modification time
	if err := os.Chtimes(dst, time.Now(), srcInfo.ModTime()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: chtimes %s: %v\n", dst, err)
	}

	return os.Remove(src)
}
