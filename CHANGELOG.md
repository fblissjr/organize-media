# Changelog

## 0.1.0

- Initial Go rewrite of organize_media.sh and rename_files.py
- Single static binary with no external dependencies (exiftool, Python)
- EXIF date extraction via bep/imagemeta (HEIC/AVIF/JPEG/PNG/WebP/RAW)
- QuickTime mvhd creation_time extraction for MP4/MOV
- Filename regex date parsing fallback
- Partial hash dedup with gob-encoded file cache (no CGO)
- Scoped directory indexing for NAS-optimized I/O
- Two-pass pipeline: scan/plan then execute
- Cross-device move detection with copy+verify+delete fallback
- Dry-run mode for previewing changes
- Separate image/video rename prefixes
- Year/month/day granularity for directory organization
- Cross-compiles to linux/amd64 and linux/arm64 for Synology NAS
