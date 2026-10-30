package main

import (
	"bytes"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFrameTimesConvertsMicrosecondsToMilliseconds(t *testing.T) {
	input := "Time,Render CPU (µs),Render GPU (µs)\n0,1250,2500\n1,0,750\n"
	times, err := readFrameTimes(strings.NewReader(input), cpuColumn, gpuColumn)
	if err != nil {
		t.Fatal(err)
	}
	if len(times.cpuMS) != 2 || times.cpuMS[0] != 1.25 || times.gpuMS[0] != 2.5 {
		t.Fatalf("unexpected converted timings: %#v", times)
	}
	if times.cpuMS[1] != 0 || times.gpuMS[1] != 0.75 {
		t.Fatalf("unexpected zero/decimal timings: %#v", times)
	}
}

func TestReadFrameTimesRequiresColumns(t *testing.T) {
	_, err := readFrameTimes(strings.NewReader("CPU,GPU\n1,2\n"), cpuColumn, gpuColumn)
	if err == nil || !strings.Contains(err.Error(), cpuColumn) || !strings.Contains(err.Error(), gpuColumn) {
		t.Fatalf("expected required-column error, got %v", err)
	}
}

func TestReadFrameTimesRejectsInvalidValues(t *testing.T) {
	input := "Render CPU (µs),Render GPU (µs)\nnot-a-number,2\n"
	_, err := readFrameTimes(strings.NewReader(input), cpuColumn, gpuColumn)
	if err == nil || !strings.Contains(err.Error(), "row 2") {
		t.Fatalf("expected row-specific parse error, got %v", err)
	}
}

func TestReadFrameTimesUsesCustomColumns(t *testing.T) {
	input := "CPU render,GPU render\n4250,7825\n"
	times, err := readFrameTimes(strings.NewReader(input), "CPU render", "GPU render")
	if err != nil {
		t.Fatal(err)
	}
	if times.cpuMS[0] != 4.25 || times.gpuMS[0] != 7.825 {
		t.Fatalf("unexpected timings for custom columns: %#v", times)
	}
}

func TestValidateColumnNames(t *testing.T) {
	if err := validateColumnNames("CPU", "GPU"); err != nil {
		t.Fatalf("valid column names rejected: %v", err)
	}
	for _, names := range [][2]string{{"", "GPU"}, {"CPU", "CPU"}, {"CPU\n", "GPU"}} {
		if err := validateColumnNames(names[0], names[1]); err == nil {
			t.Errorf("invalid column names %q and %q were accepted", names[0], names[1])
		}
	}
}

func TestParseMicrosecondsRejectsUnsafeNumbers(t *testing.T) {
	for _, value := range []string{"-1", "NaN", "+Inf"} {
		if _, err := parseMicroseconds(value); err == nil {
			t.Errorf("parseMicroseconds(%q) unexpectedly succeeded", value)
		}
	}
}

func TestEnsurePNGExtension(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		want      string
		wantError bool
	}{
		{name: "append missing extension", path: "chart", want: "chart.png"},
		{name: "preserve png extension", path: "chart.PNG", want: "chart.PNG"},
		{name: "reject different image type", path: "chart.jpg", wantError: true},
		{name: "reject empty output", path: "", wantError: true},
		{name: "reject control character", path: "bad\nname", wantError: true},
		{name: "reject directory path", path: "folder/", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ensurePNGExtension(test.path)
			if (err != nil) != test.wantError {
				t.Fatalf("ensurePNGExtension() error = %v, wantError %v", err, test.wantError)
			}
			if err == nil && got != test.want {
				t.Fatalf("ensurePNGExtension() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	if got, err := validateTitle("  Test chart  "); err != nil || got != "Test chart" {
		t.Fatalf("validateTitle() = %q, %v", got, err)
	}
	if _, err := validateTitle("bad\nname"); err == nil {
		t.Fatal("expected control characters in title to be rejected")
	}
	if _, err := validateTitle(strings.Repeat("x", 201)); err == nil {
		t.Fatal("expected overlong title to be rejected")
	}
}

func TestCheckOutputTargetProtectsInput(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "source.csv")
	input, err := os.Create(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := checkOutputTarget(input, inputPath); err == nil {
		t.Fatal("expected output collision with input to be rejected")
	}
	if err := checkOutputTarget(input, filepath.Join(filepath.Dir(inputPath), "chart.png")); err != nil {
		t.Fatalf("unexpected error for separate output path: %v", err)
	}
}

func TestSaveChartFileWritesPNGWithExtension(t *testing.T) {
	outputPath, err := ensurePNGExtension(filepath.Join(t.TempDir(), "chart"))
	if err != nil {
		t.Fatal(err)
	}
	times := frameTimes{cpuMS: []float64{1, 2, 3}, gpuMS: []float64{4, 5, 6}}
	if err := saveChartFile(outputPath, times, 10, "Safe chart"); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := png.Decode(file); err != nil {
		t.Fatalf("saved file is not a PNG: %v", err)
	}
}

func TestHistogramIncludesBoundaryValues(t *testing.T) {
	counts := histogram([]float64{0, 1, 2}, 2, 2)
	if counts[0] != 1 || counts[1] != 2 {
		t.Fatalf("unexpected histogram: %v", counts)
	}
}

func TestHistogramDoesNotPileOverflowIntoLastBin(t *testing.T) {
	counts := histogram([]float64{1, 2, 30}, 3, 2)
	if counts[0] != 0 || counts[1] != 1 || counts[2] != 1 {
		t.Fatalf("unexpected histogram with overflow: %v", counts)
	}
}

func TestPercentileCanExcludeRareSpike(t *testing.T) {
	values := make([]float64, 200)
	for i := range values {
		values[i] = float64(i)
	}
	values[199] = 1000
	if got := percentile(values, 0.995); got != 198 {
		t.Fatalf("99.5th percentile = %v, want 198", got)
	}
	if count := countAbove(values, 198); count != 1 {
		t.Fatalf("countAbove() = %d, want 1", count)
	}
}

func TestFormatAxisValueHidesFloatRoundingArtifacts(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{value: 55.00000000000001, want: "55"},
		{value: 12.5, want: "12.5"},
		{value: 0.05, want: "0.05"},
		{value: 100, want: "100"},
	}
	for _, test := range tests {
		if got := formatAxisValue(test.value); got != test.want {
			t.Errorf("formatAxisValue(%v) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestNiceAxisRangeAdaptsToSmallValues(t *testing.T) {
	maxValue, step := niceAxisRange(0.63)
	if math.Abs(maxValue-0.7) > 1e-12 || math.Abs(step-0.1) > 1e-12 {
		t.Fatalf("niceAxisRange(0.63) = (%v, %v), want (0.7, 0.1)", maxValue, step)
	}
}

func TestSmoothHistogramPreservesSampleWeight(t *testing.T) {
	smoothed := smoothHistogram([]int{0, 0, 10, 0, 0}, 1.8)
	total := 0.0
	for _, value := range smoothed {
		total += value
	}
	if math.Abs(total-10) > 1e-9 {
		t.Fatalf("smoothing changed total weight: %f", total)
	}
	if smoothed[2] <= smoothed[0] || smoothed[2] <= smoothed[4] {
		t.Fatalf("smoothing misplaced peak: %v", smoothed)
	}
}

func TestWriteChartProducesPNG(t *testing.T) {
	var output bytes.Buffer
	times := frameTimes{cpuMS: []float64{1, 2, 3}, gpuMS: []float64{4, 5, 6}}
	if err := writeChart(&output, times, 10, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(&output); err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
}

func TestWriteChartIncludesOptionalTitle(t *testing.T) {
	times := frameTimes{cpuMS: []float64{1, 2, 3}, gpuMS: []float64{4, 5, 6}}
	var withoutTitle, withTitle bytes.Buffer
	if err := writeChart(&withoutTitle, times, 10, ""); err != nil {
		t.Fatal(err)
	}
	if err := writeChart(&withTitle, times, 10, "My Frametime Chart"); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(withoutTitle.Bytes(), withTitle.Bytes()) {
		t.Fatal("title did not change rendered chart")
	}
}

func TestWriteChartRejectsInvalidFrameTimes(t *testing.T) {
	times := frameTimes{cpuMS: []float64{math.NaN()}, gpuMS: []float64{1}}
	if err := writeChart(&bytes.Buffer{}, times, 10, ""); err == nil {
		t.Fatal("expected non-finite frame time to be rejected")
	}
}

func TestCSVSampleCanBeRead(t *testing.T) {
	file, err := os.Open("resources/example data.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := readFrameTimes(file, cpuColumn, gpuColumn); err != nil {
		t.Fatal(err)
	}
}
