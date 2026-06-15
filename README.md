# focalstats

English | [简体中文](README.zh-CN.md)

Point it at a directory and it tells you, **efficiently and with low resource usage**, which focal lengths you shoot most — handy for understanding your habits and picking your next lens.

- Reads **EXIF metadata only**, never decoding the full image (via [`evanoberholster/imagemeta`](https://github.com/evanoberholster/imagemeta)), so CPU/memory stay low and independent of image size.
- Concurrent traversal (worker pool = CPU count); hundreds of thousands of photos in seconds.
- Single run: point at a path → print stats → exit.
- Supports **JPEG / HEIC / TIFF** and common **RAW** (DNG/CR2/CR3/NEF/ARW/RW2/RAF/ORF…).
- Outputs a terminal table, **JSON**, **CSV**, or a self-contained **interactive HTML report** (filter by camera/lens/date/aperture/… and every chart recomputes in-browser).
- Multi-arch Docker image (`linux/amd64` + `linux/arm64`) published on GHCR.

## Docker (recommended)

Image: `ghcr.io/t0saki/focalstats`. Mount your photo directory **read-only** and pass the mount point as the argument:

```bash
docker run --rm -v /your/photos:/data:ro ghcr.io/t0saki/focalstats /data

# JSON / CSV
docker run --rm -v /your/photos:/data:ro ghcr.io/t0saki/focalstats --json /data

# HTML report — mount a writable dir and write the file into it
docker run --rm -v /your/photos:/data:ro -v "$PWD":/out \
  ghcr.io/t0saki/focalstats --html -o /out/report.html /data

# Actual focal length, 5mm buckets, top 10
docker run --rm -v /your/photos:/data:ro ghcr.io/t0saki/focalstats \
  --basis actual --round 5 --top 10 /data
```

## Local (Go 1.23+)

```bash
go run . /your/photos
go build -o focalstats . && ./focalstats --html -o report.html /your/photos
```

## Usage

```
focalstats [flags] <path>
```

| Flag        | Default   | Description                                                       |
| ----------- | --------- | ---------------------------------------------------------------- |
| `--basis`   | `35mm`    | Focal basis: `35mm` (35mm-equivalent) or `actual` (true focal)   |
| `--round`   | `1`       | Bucket rounding step, in mm                                       |
| `--top`     | `0`       | Limit focal table & gear lists to N entries (0 = sensible default)|
| `--workers` | `0`       | Parallel workers (0 = CPU count)                                  |
| `--ext`     | built-in  | Comma-separated extensions overriding the built-in set           |
| `--json`    | `false`   | Output JSON                                                       |
| `--csv`     | `false`   | Output CSV                                                        |
| `--html`    | `false`   | Output a self-contained HTML report                              |
| `-o`        | stdout    | Write output to a file instead of stdout                         |

Stats go to stdout, progress/errors to stderr, so piping works cleanly.

## HTML report

`--html` produces a single self-contained, **interactive** file: the per-photo
records are embedded (gzip+base64) and a small bundled script filters and
re-aggregates them entirely in your browser. **No external/CDN assets, no
network — fully offline.**

Filter by **camera body, lens, date range, hour of day, weekday, aperture, ISO,
shutter speed**, and toggle the **focal basis** (35mm-equivalent ⇄ actual); every
chart and table below recomputes live:

- Overview counters and a "matched N / total" indicator.
- Focal-length distribution histogram + top table.
- **Focal length × year** heatmap (how your focal habits evolved).
- **Focal length per camera body** — compare how different bodies / phone models differ.
- By hour of day, weekday, year, and a monthly timeline.
- Camera bodies and lenses (top N); aperture, ISO, shutter-speed distributions.

**File size & requirements.** The report scales with the number of photos that
carry EXIF (≈ a few hundred KB per 100k photos after compression — e.g. ~1 MB
for a 150k-photo library). Decompression uses the browser's built-in
`DecompressionStream`, so open it in a recent Chrome/Edge/Firefox or Safari 16.4+.

## Example (terminal)

```
focalstats — focal-length usage (basis: 35mm-equivalent)
scanned: 22572  with focal: 22572  no focal: 0  failed: 0

  focal        count    share  distribution
  --------  --------  -------  ----------------------------------------
     24mm        10230    45.3%  ████████████████████████████████████████
     26mm         2431    10.8%  █████████
     35mm          984     4.4%  ███
     70mm         2870    12.7%  ███████████
     77mm         1983     8.8%  ███████
```

## How it works

1. `filepath.WalkDir` walks the tree and filters candidates by extension.
2. Paths flow through a buffered channel; each worker goroutine reads only the EXIF header, extracts the fields, and folds them into **private histograms**.
3. After all workers finish, the histograms are merged (no lock contention) and rendered as a table / JSON / CSV / HTML.

Each worker holds a single open file and reads only metadata bytes at any instant, so memory stays bounded regardless of directory size or image dimensions; the aggregates are keyed by low-cardinality values (focal length, hour, camera…), so they too stay small.

## License

MIT
