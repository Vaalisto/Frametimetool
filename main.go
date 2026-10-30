package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const (
	cpuColumn      = "Render CPU (µs)"
	gpuColumn      = "Render GPU (µs)"
	defaultCSVPath = "resources/example data.csv"
)

type frameTimes struct {
	cpuMS []float64
	gpuMS []float64
}

func main() {
	inputPathFlag := flag.String("input", "", "CSV file containing frame timings (positional path is also supported)")
	outputPath := flag.String("output", "distribution.png", "PNG chart output path")
	bins := flag.Int("bins", 800, "number of histogram bins")
	flag.Parse()
	inputPath, err := resolveInputPath(*inputPathFlag, flag.Args())
	if err != nil {
		fatalf("%v", err)
	}

	if *bins < 1 || *bins > 5000 {
		fatalf("-bins must be between 1 and 5000")
	}

	input, err := os.Open(inputPath)
	if err != nil {
		fatalf("open input %q: %v", inputPath, err)
	}
	defer input.Close()

	times, err := readFrameTimes(input)
	if err != nil {
		fatalf("read CSV: %v", err)
	}

	output, err := os.Create(*outputPath)
	if err != nil {
		fatalf("create output: %v", err)
	}
	if err := writeChart(output, times, *bins); err != nil {
		output.Close()
		fatalf("write chart: %v", err)
	}
	if err := output.Close(); err != nil {
		fatalf("close output: %v", err)
	}

	fmt.Printf("Wrote %s (%d frames)\n", *outputPath, len(times.cpuMS))
}

func resolveInputPath(inputFlag string, args []string) (string, error) {
	if len(args) > 1 {
		return "", fmt.Errorf("expected at most one positional CSV path")
	}
	if inputFlag != "" && len(args) != 0 {
		return "", fmt.Errorf("provide the CSV path either positionally or with -input, not both")
	}
	if inputFlag != "" {
		return inputFlag, nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return defaultCSVPath, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func readFrameTimes(input io.Reader) (frameTimes, error) {
	reader := csv.NewReader(input)
	header, err := reader.Read()
	if err != nil {
		return frameTimes{}, fmt.Errorf("read header: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	cpuIndex, gpuIndex := -1, -1
	for i, name := range header {
		if name == cpuColumn {
			cpuIndex = i
		}
		if name == gpuColumn {
			gpuIndex = i
		}
	}
	if cpuIndex < 0 || gpuIndex < 0 {
		return frameTimes{}, fmt.Errorf("CSV must contain columns %q and %q", cpuColumn, gpuColumn)
	}

	var times frameTimes
	rowNumber := 1
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		rowNumber++
		if err != nil {
			return frameTimes{}, fmt.Errorf("row %d: %w", rowNumber, err)
		}
		cpu, err := parseMicroseconds(record[cpuIndex])
		if err != nil {
			return frameTimes{}, fmt.Errorf("row %d, %s: %w", rowNumber, cpuColumn, err)
		}
		gpu, err := parseMicroseconds(record[gpuIndex])
		if err != nil {
			return frameTimes{}, fmt.Errorf("row %d, %s: %w", rowNumber, gpuColumn, err)
		}
		times.cpuMS = append(times.cpuMS, cpu/1000)
		times.gpuMS = append(times.gpuMS, gpu/1000)
	}
	if len(times.cpuMS) == 0 {
		return frameTimes{}, fmt.Errorf("CSV contains no frame records")
	}
	return times, nil
}

func parseMicroseconds(value string) (float64, error) {
	microseconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", value)
	}
	if math.IsNaN(microseconds) || math.IsInf(microseconds, 0) || microseconds < 0 {
		return 0, fmt.Errorf("expected a finite, non-negative number, got %q", value)
	}
	return microseconds, nil
}
