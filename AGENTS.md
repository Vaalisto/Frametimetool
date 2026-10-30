# Repository notes

- This is a dependency-free Go CLI. `-f` (or `-input`) is mandatory; `-o` defaults to `distribution.png` (800 bins, allowed 2–5000). Optional `-t` sets the title; use `-?` to list all flags. Run from the repository root.
- `-o` appends `.png` when no extension is given and rejects non-PNG extensions. Output is written via a temp file and cannot alias the input CSV.
- `-cpu` and `-gpu` override the CSV column headers; defaults are `Render CPU (µs)` and `Render GPU (µs)`.
- The x-axis adapts to the higher per-series 99.5th percentile; overflow samples are excluded from bins and counted beside the legend.
- Verify changes with `go test ./...` and build with `go build .`.
- `resources/example data.csv` is the sample input. Its default `Render CPU (µs)` and `Render GPU (µs)` values are converted to milliseconds; custom header names are selectable with `-cpu`/`-gpu`.
- `resources/distributionchart.jpg` is a visual reference, not generated output. The chart is rendered using only Go's standard library.
