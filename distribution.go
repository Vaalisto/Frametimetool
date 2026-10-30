package main

import (
	"fmt"
	"math"
	"sort"
)

const (
	minBins          = 2
	maxBins          = 5000
	defaultBins      = 800
	upperPercentile  = 0.995
	axisHeadroom     = 1.05
	yAxisHeadroom    = 1.15
	smoothingSigmaMS = 0.05
	maxAxisTicks     = 40
)

// distribution holds everything needed to draw the chart. Percentages are per series.
type distribution struct {
	xMax, xStep float64
	yMax        float64
	cpuPercent  []float64
	gpuPercent  []float64
	cpuOverflow int
	gpuOverflow int
}

func buildDistribution(times frameTimes, binCount int) distribution {
	axisMax := maxSeriesPercentile(upperPercentile, times.cpuMS, times.gpuMS)
	if axisMax <= math.MaxFloat64/axisHeadroom {
		axisMax *= axisHeadroom
	}
	xMax, xStep := niceAxisRange(axisMax)
	sigmaBins := smoothingSigmaBins(xMax, binCount)
	cpuPercent := toPercent(smoothHistogram(histogram(times.cpuMS, binCount, xMax), sigmaBins), len(times.cpuMS))
	gpuPercent := toPercent(smoothHistogram(histogram(times.gpuMS, binCount, xMax), sigmaBins), len(times.gpuMS))
	peak := 0.0
	for i := range cpuPercent {
		peak = max(peak, cpuPercent[i], gpuPercent[i])
	}
	return distribution{
		xMax:        xMax,
		xStep:       xStep,
		yMax:        niceCeiling(peak * yAxisHeadroom),
		cpuPercent:  cpuPercent,
		gpuPercent:  gpuPercent,
		cpuOverflow: countAbove(times.cpuMS, xMax),
		gpuOverflow: countAbove(times.gpuMS, xMax),
	}
}

// smoothingSigmaBins converts the fixed millisecond smoothing width into bins.
func smoothingSigmaBins(xMax float64, binCount int) float64 {
	return smoothingSigmaMS / (xMax / float64(binCount))
}

func histogram(values []float64, bins int, maxValue float64) []int {
	counts := make([]int, bins)
	for _, value := range values {
		if value > maxValue {
			continue
		}
		index := int(value / maxValue * float64(bins))
		if index >= bins {
			index = bins - 1
		}
		if index < 0 {
			index = 0
		}
		counts[index]++
	}
	return counts
}

// percentileInPlace sorts values in place and returns the nearest-rank percentile.
func percentileInPlace(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	index := int(math.Ceil(percentile*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

// maxSeriesPercentile returns the largest per-series percentile. It sorts one
// scratch copy at a time, so the input series keep their original order.
func maxSeriesPercentile(percentile float64, series ...[]float64) float64 {
	maxLength := 0
	for _, values := range series {
		maxLength = max(maxLength, len(values))
	}
	scratch := make([]float64, maxLength)
	maximum := 0.0
	for _, values := range series {
		copy(scratch, values)
		maximum = max(maximum, percentileInPlace(scratch[:len(values)], percentile))
	}
	return maximum
}

func countAbove(values []float64, maxValue float64) int {
	count := 0
	for _, value := range values {
		if value > maxValue {
			count++
		}
	}
	return count
}

func smoothHistogram(counts []int, sigma float64) []float64 {
	smoothed := make([]float64, len(counts))
	if sigma <= 0 {
		for i, count := range counts {
			smoothed[i] = float64(count)
		}
		return smoothed
	}
	// Capping the kernel at the histogram length keeps very wide smoothing from
	// overflowing the int conversion or becoming quadratic in the bin count.
	radius := int(min(math.Ceil(3*sigma), float64(len(counts)-1)))
	weights := make([]float64, 2*radius+1)
	weightSum := 0.0
	for offset := -radius; offset <= radius; offset++ {
		weight := math.Exp(-0.5 * math.Pow(float64(offset)/sigma, 2))
		weights[offset+radius] = weight
		weightSum += weight
	}
	for i := range weights {
		weights[i] /= weightSum
	}

	originalTotal, smoothedTotal := 0.0, 0.0
	for i := range counts {
		originalTotal += float64(counts[i])
		for offset := -radius; offset <= radius; offset++ {
			source := i + offset
			if source >= 0 && source < len(counts) {
				smoothed[i] += float64(counts[source]) * weights[offset+radius]
			}
		}
		smoothedTotal += smoothed[i]
	}
	if smoothedTotal > 0 {
		scale := originalTotal / smoothedTotal
		for i := range smoothed {
			smoothed[i] *= scale
		}
	}
	return smoothed
}

func toPercent(counts []float64, sampleCount int) []float64 {
	percent := make([]float64, len(counts))
	for i, count := range counts {
		percent[i] = 100 * count / float64(sampleCount)
	}
	return percent
}

func niceCeiling(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 1
	}
	power := math.Pow(10, math.Floor(math.Log10(value)))
	normalized := value / power
	factor := 10.0
	switch {
	case normalized <= 1:
		factor = 1
	case normalized <= 2:
		factor = 2
	case normalized <= 2.5:
		factor = 2.5
	case normalized <= 5:
		factor = 5
	}
	ceiling := factor * power
	if math.IsInf(ceiling, 0) {
		return value
	}
	return ceiling
}

// niceAxisRange returns an axis maximum and tick step. Ranges of 1 ms and more
// use 1 ms steps, coarsening to 2, 5, 10 ms... only when that would exceed
// maxAxisTicks. Sub-millisecond ranges use about eight ticks.
func niceAxisRange(dataMax float64) (maxValue, step float64) {
	if dataMax <= 0 || math.IsNaN(dataMax) || math.IsInf(dataMax, 0) {
		dataMax = 1
	}
	if dataMax >= 1 {
		step = millisecondStep(dataMax)
	} else {
		step = niceCeiling(dataMax / 8)
	}
	maxValue = math.Ceil(dataMax/step) * step
	if math.IsInf(maxValue, 0) || maxValue < dataMax {
		return dataMax, dataMax / 8
	}
	return maxValue, step
}

// millisecondStep returns the smallest 1, 2, 5 × 10^n step that keeps the axis
// within maxAxisTicks, so 1 ms is used whenever it fits.
func millisecondStep(dataMax float64) float64 {
	for decade := 1.0; ; decade *= 10 {
		for _, multiplier := range []float64{1, 2, 5} {
			step := multiplier * decade
			if math.Ceil(dataMax/step) <= maxAxisTicks {
				return step
			}
		}
	}
}

func formatAxisValue(value float64) string {
	if math.Abs(value) >= 1e12 {
		return fmt.Sprintf("%.5g", value)
	}
	rounded := math.Round(value)
	if math.Abs(value-rounded) <= 1e-9*math.Max(1, math.Abs(value)) {
		return fmt.Sprintf("%.0f", rounded)
	}
	for decimals := 1; decimals <= 6; decimals++ {
		factor := math.Pow10(decimals)
		rounded = math.Round(value*factor) / factor
		if math.Abs(value-rounded) <= 1e-9*math.Max(1, math.Abs(value)) {
			return fmt.Sprintf("%.*f", decimals, rounded)
		}
	}
	return fmt.Sprintf("%.6g", value)
}
