package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev"

func main() {
	var cfg Config
	var showVersion bool

	flag.StringVar(&cfg.Granularity, "g", "month", "Granularity: year, month, day")
	flag.StringVar(&cfg.Prefix, "p", "", "Rename prefix (format: PREFIX__YYYYMMDD_HHMMSS.ext)")
	flag.StringVar(&cfg.ImagePrefix, "image-prefix", "", "Rename prefix for images (overrides -p)")
	flag.StringVar(&cfg.VideoPrefix, "video-prefix", "", "Rename prefix for videos (overrides -p)")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Preview without acting")
	flag.BoolVar(&cfg.Force, "force", false, "Rename files even if they already match naming pattern")
	flag.BoolVar(&cfg.NoRename, "no-rename", false, "Preserve original filenames")
	flag.BoolVar(&cfg.NoDedup, "no-dedup", false, "Skip dedup checking")
	flag.BoolVar(&cfg.VerifyFull, "verify-full", false, "Full SHA256 verification after partial hash match")
	flag.BoolVar(&cfg.RebuildDB, "rebuild-cache", false, "Force rebuild dedup cache")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Detailed progress on stderr")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: organize-media [flags] <source-dir> [target-dir]\n\n")
		fmt.Fprintf(os.Stderr, "Organize media files into date-based directory structure with deduplication.\n")
		fmt.Fprintf(os.Stderr, "Files are renamed to YYYYMMDD_HHMMSS_<random>.ext by default.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cfg.SourceDir = args[0]
	if len(args) >= 2 {
		cfg.TargetDir = args[1]
	}

	// Validate granularity
	switch cfg.Granularity {
	case "year", "month", "day":
	default:
		fmt.Fprintf(os.Stderr, "error: granularity must be year, month, or day\n")
		os.Exit(1)
	}

	// Validate source directory exists
	info, err := os.Stat(cfg.SourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: %s is not a directory\n", cfg.SourceDir)
		os.Exit(1)
	}

	if err := organize(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
