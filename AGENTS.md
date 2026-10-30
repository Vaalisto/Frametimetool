# Repository notes

- This is a dependency-free Go CLI. `-f` (or `-input`) is mandatory; `-o` defaults to `distribution.png` (800 bins). Optional `-t` sets the title; use `-?` to list all flags. Run from the repository root.
- `-o` appends `.png` when no extension is given and rejects non-PNG extensions. Output is written via a temp file and cannot alias the input CSV.
- Verify changes with `go test ./...` and build with `go build .`.
- `resources/example data.csv` is the sample input. Its exact `Render CPU (µs)` and `Render GPU (µs)` headers are required; values are converted to milliseconds for the chart.
- `resources/distributionchart.jpg` is a visual reference, not generated output. The chart is rendered using only Go's standard library.
