# Frametime distribution tool

Reads frame timings from a CSV and writes a high-resolution PNG distribution chart comparing render CPU and GPU times. The dark chart uses a filled blue GPU distribution and a cyan CPU curve, with fine gridlines. Input values are in microseconds and are converted to milliseconds. The y-axis shows each bin's percentage of recorded frames.

Run from the repository root:

```sh
go run .
```

By default this reads `resources/example data.csv`, uses 800 histogram bins, and writes `distribution.png`. Pass a CSV path positionally to use another file:

```sh
go run . "path/to/data.csv"
```

You can also use flags to select the input and output:

```sh
go run . -input "path/to/data.csv" -output chart.png -bins 800
```

Build and test with `go build .` and `go test ./...`. The project uses only the Go standard library.
