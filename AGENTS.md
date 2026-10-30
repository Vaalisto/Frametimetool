# Repository notes

- This is a dependency-free Go CLI. Run it from the repository root: `go run .`; defaults read `resources/example data.csv` and write `distribution.png` (800 bins). Pass another CSV positionally or with `-input`; customize output/bin count with `-output` and `-bins`.
- Verify changes with `go test ./...` and build with `go build .`.
- `resources/example data.csv` is the sample input. Its exact `Render CPU (µs)` and `Render GPU (µs)` headers are required; values are converted to milliseconds for the chart.
- `resources/distributionchart.jpg` is a visual reference, not generated output. The chart is rendered using only Go's standard library.
