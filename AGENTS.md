# Repository notes

- Dependency-free Go CLI (standard library only). Run from the repository root, e.g. `go run . -f "resources/example data.csv"`. `-?` lists all flags.
- Verify with `go test ./...`, `go vet ./...`, and `go build .`.
- `go env GO111MODULE` is `off` on this machine, so builds use GOPATH mode and `go.mod` is ignored. This is harmless while there are no external dependencies; don't add any without changing this setup.
- Layout: `main.go` (CLI, validation, CSV parsing, safe output), `distribution.go` (histogram and axis math), `chart.go` (chart layout and drawing), `canvas.go` (rasterizer; takes logical coordinates and scales by `renderScale`), `font.go` (5x7 bitmap glyphs).
- Titles may only use characters in the bitmap font (`supportedGlyph`). Add glyphs to `font.go` to support more.
- `-f` is required; `-o` gets `.png` appended if missing and rejects other extensions; `-cpu`/`-gpu` default to `Render CPU (µs)`/`Render GPU (µs)`; `-bins` is 2–5000 (default 800); input is capped at 10,000,000 frames.
- Histogram smoothing is fixed at 0.05 ms, so `-bins` changes resolution without changing smoothing.
- The x-axis uses the higher per-series 99.5th percentile; frames beyond it are counted beside the legend, not drawn.
- `resources/example data.csv` is the sample input. `resources/distributionchart.jpg` is a visual reference only, not generated output.
