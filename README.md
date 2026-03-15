# organize-media

Go CLI that organizes media files (photos, videos) into date-based directory trees with deduplication. Single static binary, no external tools required.

Replaces `organize_media.sh` (requires exiftool) and `rename_files.py` (requires Python) with one portable binary that cross-compiles to Synology NAS.

## Install

```bash
# Build from source
make build

# Cross-compile for Synology NAS
make synology-amd64   # x86 Synology
make synology-arm64   # ARM Synology
```

Requires Go 1.25+.

## Usage

```
organize-media [flags] <source-dir> [target-dir]
```

If `target-dir` is omitted, organizes files in-place under `source-dir`.

### Examples

```bash
# Organize photos into YYYY/MM/ structure (dry run first)
organize-media -dry-run ~/Photos

# Actually move them
organize-media ~/Photos

# Organize into a different target directory
organize-media ~/DCIM /volume1/photos

# Rename files with a prefix: photo__20260315_143045.jpg
organize-media -p photo ~/Photos

# Separate prefixes for images and videos
organize-media -image-prefix pic -video-prefix clip ~/Photos

# Organize by year only
organize-media -g year ~/Photos

# Organize by day
organize-media -g day ~/Photos

# Skip dedup checking (faster, no cache)
organize-media -no-dedup ~/Photos

# Full SHA256 verification on dedup matches
organize-media -verify-full ~/Photos

# See detailed progress
organize-media -verbose ~/Photos
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-g` | `month` | Granularity: `year`, `month`, or `day` |
| `-p` | (none) | Rename prefix for all files |
| `-image-prefix` | (none) | Rename prefix for images (overrides `-p`) |
| `-video-prefix` | (none) | Rename prefix for videos (overrides `-p`) |
| `-dry-run` | `false` | Preview changes without moving files |
| `-force` | `false` | Rename files even if they already match naming pattern |
| `-no-dedup` | `false` | Skip dedup checking |
| `-verify-full` | `false` | Full SHA256 after partial hash match |
| `-rebuild-db` | `false` | Force rebuild dedup cache |
| `-verbose` | `false` | Detailed progress on stderr |
| `-version` | | Print version and exit |

## How it works

### Date extraction (fallback chain)

1. **EXIF metadata** -- via `bep/imagemeta` (JPEG, HEIC, AVIF, PNG, WebP, TIFF, RAW)
2. **Filename regex** -- `YYYY-MM-DDTHHMMSS`, `YYYYMMDD_HHMMSS`, `YYYY-MM-DD`
3. **QuickTime mvhd** -- creation_time atom for MP4/MOV
4. **File mtime** -- last resort

### Deduplication

Files are compared using a partial hash: SHA256 of the first 64KB + last 64KB + file size. This reads ~128KB per file instead of the entire contents. Use `-verify-full` for full SHA256 verification on matches.

The dedup cache (`.organize-media.cache`) uses **scoped indexing**: only target subdirectories relevant to source file dates are indexed. Organizing files from March 2026 never reads the 2024/ or 2025/ directories.

### Two-pass pipeline

1. **Scan**: Walk source, extract dates, compute hashes, check dedup, build action list
2. **Execute**: Create directories, move files, update cache

Dry-run (`-dry-run`) only runs pass 1 and prints the plan.

### Cross-device moves

Detects when source and target are on different filesystems (compares device IDs). Falls back to copy + verify + delete instead of `os.Rename`.

## Supported formats

**Images**: jpg, jpeg, png, gif, heic, heif, bmp, tiff, webp, cr2, nef, arw, dng, raf, avif

**Videos**: mp4, mov, avi, mkv, m4v, wmv, flv, webm, mts, m2ts, 3gp
