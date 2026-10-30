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

func TestReadFrameTimesRejectsDuplicateColumns(t *testing.T) {
	input := "Render CPU (µs),Render CPU (µs),Render GPU (µs)\n1,2,3\n"
	_, err := readFrameTimes(strings.NewReader(input), cpuColumn, gpuColumn)
	if err == nil || !strings.Contains(err.Error(), "duplicate column") {
		t.Fatalf("expected duplicate-column error, got %v", err)
	}
}

func TestReadFrameTimesRejectsInvalidValues(t *testing.T) {
	input := "Render CPU (µs),Render GPU (µs)\nnot-a-number,2\n"
	_, err := readFrameTimes(strings.NewReader(input), cpuColumn, gpuColumn)
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected line-specific parse error, got %v", err)
	}
}

func TestReadFrameTimesReportsPhysicalLineNumber(t *testing.T) {
	input := "Note,Render CPU (µs),Render GPU (µs)\n\"multi\nline\",1250,2500\nkeep,not-a-number,2\n"
	_, err := readFrameTimes(strings.NewReader(input), cpuColumn, gpuColumn)
	if err == nil || !strings.Contains(err.Error(), "line 4") {
		t.Fatalf("expected error on physical line 4, got %v", err)
	}
}

func TestReadFrameTimesListsAvailableColumns(t *testing.T) {
	_, err := readFrameTimes(strings.NewReader("Time,App CPU\n1,2\n"), cpuColumn, gpuColumn)
	if err == nil || !strings.Contains(err.Error(), `"App CPU"`) {
		t.Fatalf("expected error listing the found columns, got %v", err)
	}
}

func TestReadFrameTimesEnforcesFrameLimit(t *testing.T) {
	input := "Render CPU (µs),Render GPU (µs)\n1,1\n2,2\n"
	if _, err := readFrameTimesLimit(strings.NewReader(input), cpuColumn, gpuColumn, 1); err == nil {
		t.Fatal("expected frame limit to be enforced")
	}
	if _, err := readFrameTimesLimit(strings.NewReader(input), cpuColumn, gpuColumn, 2); err != nil {
		t.Fatalf("frame limit rejected input at the limit: %v", err)
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
	if _, err := validateTitle("Frametime – overview"); err == nil {
		t.Fatal("expected unsupported title glyph to be rejected")
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
	if got := maxSeriesPercentile(0.995, values, []float64{10, 20}); got != 198 {
		t.Fatalf("maximum per-series 99.5th percentile = %v, want 198", got)
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

func TestNiceAxisRangeUsesOneMillisecondSteps(t *testing.T) {
	tests := []struct {
		dataMax, wantMax, wantStep float64
	}{
		{dataMax: 20.3, wantMax: 21, wantStep: 1},
		{dataMax: 1.5, wantMax: 2, wantStep: 1},
		{dataMax: 120, wantMax: 120, wantStep: 5},
	}
	for _, test := range tests {
		maxValue, step := niceAxisRange(test.dataMax)
		if math.Abs(maxValue-test.wantMax) > 1e-12 || math.Abs(step-test.wantStep) > 1e-12 {
			t.Errorf("niceAxisRange(%v) = (%v, %v), want (%v, %v)", test.dataMax, maxValue, step, test.wantMax, test.wantStep)
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

func TestWriteChartRejectsSingleBin(t *testing.T) {
	times := frameTimes{cpuMS: []float64{1}, gpuMS: []float64{2}}
	if err := writeChart(&bytes.Buffer{}, times, 1, ""); err == nil {
		t.Fatal("expected a single-bin chart to be rejected")
	}
}

func TestWriteChartRendersMinimumBins(t *testing.T) {
	var output bytes.Buffer
	times := frameTimes{cpuMS: []float64{1, 2}, gpuMS: []float64{3, 4}}
	if err := writeChart(&output, times, minBins, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(&output); err != nil {
		t.Fatalf("minimum-bin chart is not a valid PNG: %v", err)
	}
}

func TestSmoothingWidthIsFixedInMilliseconds(t *testing.T) {
	const xMax = 25.0
	for _, bins := range []int{100, 800} {
		widthMS := smoothingSigmaBins(xMax, bins) * xMax / float64(bins)
		if math.Abs(widthMS-smoothingSigmaMS) > 1e-12 {
			t.Fatalf("bins=%d smoothing width = %v ms, want %v", bins, widthMS, smoothingSigmaMS)
		}
	}
}

func TestSmoothHistogramHandlesExtremeSigma(t *testing.T) {
	counts := make([]int, 5000)
	counts[2500] = 10
	smoothed := smoothHistogram(counts, 1e300)
	total := 0.0
	for _, value := range smoothed {
		total += value
	}
	if math.Abs(total-10) > 1e-6 {
		t.Fatalf("extreme smoothing changed total weight: %f", total)
	}
}

func TestNiceCeilingRoundsUpToOneTwoTwoFiveFive(t *testing.T) {
	tests := map[float64]float64{0: 1, 1: 1, 1.2: 2, 2.2: 2.5, 3: 5, 12.3: 20}
	for input, want := range tests {
		if got := niceCeiling(input); math.Abs(got-want) > 1e-12 {
			t.Errorf("niceCeiling(%v) = %v, want %v", input, got, want)
		}
	}
}

func TestSupportedGlyphMatchesRenderableCharacters(t *testing.T) {
	for _, r := range "aZ 0%!" {
		if !supportedGlyph(r) {
			t.Errorf("supportedGlyph(%q) = false, want true", r)
		}
	}
	for _, r := range "–é" {
		if supportedGlyph(r) {
			t.Errorf("supportedGlyph(%q) = true, want false", r)
		}
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
