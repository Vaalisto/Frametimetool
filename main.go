package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	cpuColumn = "Render CPU (µs)"
	gpuColumn = "Render GPU (µs)"
)

type frameTimes struct {
	cpuMS []float64
	gpuMS []float64
}

func main() {
	inputPath := flag.String("f", "", "required CSV file containing frame timings")
	flag.StringVar(inputPath, "input", "", "alias for -f")
	cpuHeader := flag.String("cpu", cpuColumn, "CSV column containing render CPU time")
	gpuHeader := flag.String("gpu", gpuColumn, "CSV column containing render GPU time")
	title := flag.String("t", "", "optional chart title")
	outputPath := flag.String("o", "distribution.png", "PNG chart output path (.png is added if omitted)")
	flag.StringVar(outputPath, "output", "distribution.png", "alias for -o")
	bins := flag.Int("bins", 800, "number of histogram bins (2 to 5000)")
	flag.Usage = printUsage
	for _, arg := range os.Args[1:] {
		if arg == "-?" {
			printUsage()
			return
		}
	}
	flag.Parse()
	if *inputPath == "" {
		fatalf("-f is mandatory; use -? to see usage and all available flags")
	}
	if len(flag.Args()) != 0 {
		fatalf("unexpected positional argument %q; use -f to select the CSV file", flag.Arg(0))
	}
	if err := validateNoControlCharacters(*inputPath); err != nil {
		fatalf("-f: %v", err)
	}
	if err := validateColumnNames(*cpuHeader, *gpuHeader); err != nil {
		fatalf("CSV columns: %v", err)
	}

	if *bins < 2 || *bins > 5000 {
		fatalf("-bins must be between 2 and 5000")
	}
	normalizedOutputPath, err := ensurePNGExtension(*outputPath)
	if err != nil {
		fatalf("-o: %v", err)
	}
	titleText, err := validateTitle(*title)
	if err != nil {
		fatalf("-t: %v", err)
	}

	inputInfo, err := os.Stat(*inputPath)
	if err != nil {
		fatalf("inspect input %q: %v", *inputPath, err)
	}
	if !inputInfo.Mode().IsRegular() {
		fatalf("input %q must be a regular file", *inputPath)
	}
	input, err := os.Open(*inputPath)
	if err != nil {
		fatalf("open input %q: %v", *inputPath, err)
	}
	defer input.Close()
	openedInfo, err := input.Stat()
	if err != nil {
		fatalf("inspect opened input %q: %v", *inputPath, err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(inputInfo, openedInfo) {
		fatalf("input %q changed while being opened; retry with a stable regular file", *inputPath)
	}
	if err := checkOutputTarget(input, normalizedOutputPath); err != nil {
		fatalf("unsafe output path: %v", err)
	}

	times, err := readFrameTimes(input, *cpuHeader, *gpuHeader)
	if err != nil {
		fatalf("read CSV: %v", err)
	}

	if err := saveChartFile(normalizedOutputPath, times, *bins, titleText); err != nil {
		fatalf("write chart: %v", err)
	}

	fmt.Printf("Wrote %s (%d frames)\n", normalizedOutputPath, len(times.cpuMS))
}

func printUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -f <input.csv> [flags]\n\nFlags:\n", filepath.Base(os.Args[0]))
	flag.PrintDefaults()
}

func ensurePNGExtension(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("output filename cannot be empty")
	}
	if err := validateNoControlCharacters(path); err != nil {
		return "", err
	}
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, `\`) {
		return "", fmt.Errorf("output path must include a filename")
	}
	extension := filepath.Ext(path)
	if extension == "" {
		return path + ".png", nil
	}
	if !strings.EqualFold(extension, ".png") {
		return "", fmt.Errorf("output must be a PNG file (.png), got %q", extension)
	}
	return path, nil
}

func validateNoControlCharacters(value string) error {
	for _, char := range value {
		if unicode.IsControl(char) {
			return fmt.Errorf("value cannot contain control characters")
		}
	}
	return nil
}

func validateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > 200 {
		return "", fmt.Errorf("title must be 200 characters or fewer")
	}
	for _, char := range title {
		if unicode.IsControl(char) {
			return "", fmt.Errorf("title cannot contain control characters")
		}
		glyphChar := char
		if glyphChar >= 'a' && glyphChar <= 'z' {
			glyphChar -= 'a' - 'A'
		}
		if _, ok := glyphs[glyphChar]; !ok {
			return "", fmt.Errorf("title contains unsupported character %q; use characters supported by the chart font", char)
		}
	}
	return title, nil
}

func validateColumnNames(cpuName, gpuName string) error {
	if strings.TrimSpace(cpuName) == "" {
		return fmt.Errorf("-cpu column name cannot be empty")
	}
	if strings.TrimSpace(gpuName) == "" {
		return fmt.Errorf("-gpu column name cannot be empty")
	}
	if err := validateNoControlCharacters(cpuName); err != nil {
		return fmt.Errorf("-cpu column name: %w", err)
	}
	if err := validateNoControlCharacters(gpuName); err != nil {
		return fmt.Errorf("-gpu column name: %w", err)
	}
	if cpuName == gpuName {
		return fmt.Errorf("-cpu and -gpu must name different columns")
	}
	return nil
}

func checkOutputTarget(input *os.File, outputPath string) error {
	inputInfo, err := input.Stat()
	if err != nil {
		return fmt.Errorf("inspect input file: %w", err)
	}
	outputInfo, err := os.Stat(outputPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect output file: %w", err)
	}
	if os.SameFile(inputInfo, outputInfo) {
		return fmt.Errorf("output would overwrite the input CSV")
	}
	if !outputInfo.Mode().IsRegular() {
		return fmt.Errorf("existing output is not a regular file")
	}
	return nil
}

func saveChartFile(path string, times frameTimes, bins int, title string) error {
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, ".frametimetool-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary image: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := writeChart(temp, times, bins, title); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("flush image: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close image: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace output image: %w", err)
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func readFrameTimes(input io.Reader, cpuName, gpuName string) (frameTimes, error) {
	if err := validateColumnNames(cpuName, gpuName); err != nil {
		return frameTimes{}, err
	}
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
		if name == cpuName {
			if cpuIndex >= 0 {
				return frameTimes{}, fmt.Errorf("CSV contains duplicate column %q", cpuName)
			}
			cpuIndex = i
		}
		if name == gpuName {
			if gpuIndex >= 0 {
				return frameTimes{}, fmt.Errorf("CSV contains duplicate column %q", gpuName)
			}
			gpuIndex = i
		}
	}
	if cpuIndex < 0 || gpuIndex < 0 {
		return frameTimes{}, fmt.Errorf("CSV must contain columns %q and %q", cpuName, gpuName)
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
			return frameTimes{}, fmt.Errorf("row %d, %s: %w", rowNumber, cpuName, err)
		}
		gpu, err := parseMicroseconds(record[gpuIndex])
		if err != nil {
			return frameTimes{}, fmt.Errorf("row %d, %s: %w", rowNumber, gpuName, err)
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
