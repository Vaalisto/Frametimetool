# Frametime distribution tool

Reads frame timings from a CSV and writes a high-resolution PNG distribution chart comparing render CPU and GPU times. The dark chart uses a filled blue GPU distribution and a cyan CPU curve, with fine gridlines. Input values are in microseconds and are converted to milliseconds. The y-axis shows each bin's percentage of recorded frames.

Run from the repository root and provide the required input CSV with `-f`:

```sh
go run . -f "resources/example data.csv"
```

`-f` is mandatory. The output defaults to `distribution.png`, histogram resolution to 800 bins, and no title is shown unless `-t` is provided. For example:

```sh
go run . -f "path/to/data.csv" -t "My frametime chart" -o chart.png
```

Use `-?` to list and explain all flags. The long aliases `-input` and `-output` are also accepted. `-bins` controls histogram resolution.

The output is always PNG: if `-o` has no extension, `.png` is appended; other extensions are rejected. The tool validates frame values, requires a regular input file, rejects control characters in paths/titles, limits titles to 200 characters, and refuses to overwrite the input CSV. Images are written through a temporary file before replacing the destination.

Build and test with `go build .` and `go test ./...`. The project uses only the Go standard library.
