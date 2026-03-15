# organize-media

Go CLI that organizes media files into date-based directory trees with deduplication.

## Commands

```bash
make build              # Build binary (macOS)
make test               # Run all tests
make synology-amd64     # Cross-compile for Synology x86
make synology-arm64     # Cross-compile for Synology ARM
make clean              # Remove binaries
```

### Go module note

Dependencies are cached locally. If `go mod tidy` fails with network errors, use:

```bash
GONOSUMCHECK='*' GONOSUMDB='*' GOFLAGS='-mod=mod' GOPROXY="file://${HOME}/go/pkg/mod/cache/download,off" go build .
```

## Architecture

Flat package layout (no `cmd/` or `internal/`), one file per concern:

| File | Owns |
|------|------|
| `main.go` | CLI flags, version injection, entry point |
| `media.go` | Extension sets, `MediaType`, `Action` struct, recursive file discovery |
| `exif.go` | Date extraction: EXIF via `bep/imagemeta`, QuickTime mvhd, filename regex, mtime fallback |
| `dedup.go` | Partial hashing (first+last 64KB SHA256), gob-encoded file cache, scoped directory indexing |
| `organize.go` | Two-pass pipeline (scan/plan then execute), cross-device move, collision resolution |

## Key patterns

- **Two-pass pipeline**: Pass 1 builds an `[]Action` plan, pass 2 executes. Dry-run just prints pass 1.
- **Scoped indexing**: Only indexes target subdirectories relevant to source file dates, not the whole tree.
- **Cross-device detection**: Compares `syscall.Stat_t.Dev`; falls back to copy+verify+delete when source/target are on different filesystems.
- **Dedup cache**: Stored at `<target>/.organize-media.cache` (gob-encoded). Not crash-safe mid-run but writes atomically on completion via tmp+rename.

## Dependencies

Single external dep: `github.com/bep/imagemeta` (HEIC/AVIF/JPEG/PNG/WebP/RAW EXIF). No CGO.

## Testing

```bash
go test -v -count=1 ./...           # All tests
go test -run TestOrganizeWithDedup  # Single test
```

Tests use `t.TempDir()` for isolation -- no cleanup needed.
