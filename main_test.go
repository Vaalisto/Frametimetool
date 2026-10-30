package main

import (
	"bytes"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
)

func TestReadFrameTimesConvertsMicrosecondsToMilliseconds(t *testing.T) {
	input := "Time,Render CPU (µs),Render GPU (µs)\n0,1250,2500\n1,0,750\n"
	times, err := readFrameTimes(strings.NewReader(input))
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

func TestResolveInputPath(t *testing.T) {
	tests := []struct {
		name      string
		inputFlag string
		args      []string
		want      string
		wantError bool
	}{
		{name: "default", want: defaultCSVPath},
		{name: "positional", args: []string{"custom.csv"}, want: "custom.csv"},
		{name: "input flag", inputFlag: "flag.csv", want: "flag.csv"},
		{name: "conflicting paths", inputFlag: "flag.csv", args: []string{"arg.csv"}, wantError: true},
		{name: "too many paths", args: []string{"one.csv", "two.csv"}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveInputPath(test.inputFlag, test.args)
			if (err != nil) != test.wantError {
				t.Fatalf("resolveInputPath() error = %v, wantError %v", err, test.wantError)
			}
			if err == nil && got != test.want {
				t.Fatalf("resolveInputPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestReadFrameTimesRequiresColumns(t *testing.T) {
	_, err := readFrameTimes(strings.NewReader("CPU,GPU\n1,2\n"))
	if err == nil || !strings.Contains(err.Error(), cpuColumn) || !strings.Contains(err.Error(), gpuColumn) {
		t.Fatalf("expected required-column error, got %v", err)
	}
}

func TestReadFrameTimesRejectsInvalidValues(t *testing.T) {
	input := "Render CPU (µs),Render GPU (µs)\nnot-a-number,2\n"
	_, err := readFrameTimes(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "row 2") {
		t.Fatalf("expected row-specific parse error, got %v", err)
	}
}

func TestHistogramIncludesBoundaryValues(t *testing.T) {
	counts := histogram([]float64{0, 1, 2}, 2, 2)
	if counts[0] != 1 || counts[1] != 2 {
		t.Fatalf("unexpected histogram: %v", counts)
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
	if err := writeChart(&output, times, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(&output); err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}
}

func TestCSVSampleCanBeRead(t *testing.T) {
	file, err := os.Open("resources/example data.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := readFrameTimes(file); err != nil {
		t.Fatal(err)
	}
}
