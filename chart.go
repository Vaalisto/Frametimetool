package main

import (
	"fmt"
	"image/color"
	"image/png"
	"io"
	"math"
)

const (
	chartWidth  = 1900
	chartHeight = 700
	plotLeft    = 100
	plotTop     = 100
	plotWidth   = 1540
	plotHeight  = 490
	plotRight   = plotLeft + plotWidth
	plotBottom  = plotTop + plotHeight

	titleY      = 22
	titleMargin = 40
	titleScale  = 2

	yTickCount      = 10
	tickLabelGap    = 12
	xTickLabelY     = plotBottom + 8
	labelScale      = 2
	labelHalfHeight = glyphRows * labelScale / 2

	axisTitleY  = 650
	yAxisTitleX = 20

	legendX      = plotRight + 42
	legendY      = plotTop + 16
	legendSwatch = 25
	legendTextX  = 34
	overflowY    = plotTop + 90
	noteScale    = 1
	noteSpacing  = 20
)

var (
	background = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	gridColor  = color.RGBA{R: 27, G: 29, B: 34, A: 255}
	axisColor  = color.RGBA{R: 205, G: 208, B: 214, A: 255}
	textColor  = color.RGBA{R: 232, G: 233, B: 237, A: 255}
	cpuColor   = color.RGBA{R: 38, G: 221, B: 211, A: 255}
	gpuColor   = color.RGBA{R: 0, G: 0, B: 255, A: 255}
)

func writeChart(output io.Writer, times frameTimes, binCount int, title string) error {
	if binCount < minBins || binCount > maxBins {
		return fmt.Errorf("bin count must be between %d and %d", minBins, maxBins)
	}
	if err := validateFrameTimes(times); err != nil {
		return err
	}
	title, err := validateTitle(title)
	if err != nil {
		return err
	}

	d := buildDistribution(times, binCount)
	c := newCanvas(chartWidth, chartHeight, background)
	drawTitle(c, title)
	drawFilledCurve(c, d.gpuPercent, d.yMax, gpuColor)
	drawCurve(c, d.cpuPercent, d.yMax, cpuColor)
	drawGrid(c, d)
	drawAxisTitles(c)
	drawLegend(c, legendX, legendY)
	drawOverflowNotes(c, d)

	return png.Encode(output, c.toImage())
}

func validateFrameTimes(times frameTimes) error {
	if len(times.cpuMS) == 0 || len(times.cpuMS) != len(times.gpuMS) {
		return fmt.Errorf("CPU and GPU timings must have the same non-zero number of frames")
	}
	for _, series := range [][]float64{times.cpuMS, times.gpuMS} {
		for _, value := range series {
			if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("frame times must be finite, non-negative numbers")
			}
		}
	}
	return nil
}

func drawTitle(c canvas, title string) {
	if title == "" {
		return
	}
	scale := titleScale
	if textWidth(title, scale) > chartWidth-2*titleMargin {
		scale = 1
	}
	c.text((chartWidth-textWidth(title, scale))/2, titleY, title, scale, textColor)
}

func plotX(index, count int) float64 {
	return plotLeft + float64(index)*plotWidth/float64(count-1)
}

func plotY(value, yMax float64) float64 {
	return plotBottom - value/yMax*plotHeight
}

func drawFilledCurve(c canvas, values []float64, yMax float64, fill color.RGBA) {
	n := len(values)
	for i := 0; i < n-1; i++ {
		c.fillUnder(plotX(i, n), plotY(values[i], yMax), plotX(i+1, n), plotY(values[i+1], yMax), plotBottom, fill)
	}
}

func drawCurve(c canvas, values []float64, yMax float64, stroke color.RGBA) {
	n := len(values)
	for i := 0; i < n-1; i++ {
		c.thickLine(
			roundInt(plotX(i, n)), roundInt(plotY(values[i], yMax)),
			roundInt(plotX(i+1, n)), roundInt(plotY(values[i+1], yMax)),
			stroke,
		)
	}
}

// drawGrid draws gridlines and tick labels over the curves, as in the reference chart.
func drawGrid(c canvas, d distribution) {
	for tick := 0; tick <= yTickCount; tick++ {
		y := plotBottom - tick*plotHeight/yTickCount
		c.line(plotLeft, y, plotRight, y, gridColor)
		label := formatAxisValue(d.yMax * float64(tick) / yTickCount)
		c.text(plotLeft-tickLabelGap-textWidth(label, labelScale), y-labelHalfHeight, label, labelScale, textColor)
	}

	tickCount := roundInt(d.xMax / d.xStep)
	for tick := 0; tick <= tickCount; tick++ {
		value := float64(tick) * d.xStep
		if tick == tickCount {
			value = d.xMax
		}
		x := plotLeft + roundInt(value/d.xMax*plotWidth)
		c.line(x, plotTop, x, plotBottom, gridColor)
		label := formatAxisValue(value)
		c.text(x-textWidth(label, labelScale)/2, xTickLabelY, label, labelScale, textColor)
	}

	c.line(plotLeft, plotBottom, plotRight, plotBottom, axisColor)
	c.line(plotLeft, plotTop, plotLeft, plotBottom, axisColor)
}

func drawAxisTitles(c canvas) {
	c.text(plotLeft+plotWidth/2-textWidth("MS", labelScale)/2, axisTitleY, "MS", labelScale, textColor)
	c.verticalText(yAxisTitleX, plotTop+plotHeight/2+textWidth("% OF FRAMES", labelScale)/2,
		"% OF FRAMES", labelScale, textColor)
}

func drawLegend(c canvas, x, y int) {
	c.thickLine(x, y+9, x+legendSwatch, y+9, gpuColor)
	c.text(x+legendTextX, y, "GPU FRAMETIME", labelScale, textColor)
	c.thickLine(x, y+37, x+legendSwatch, y+37, cpuColor)
	c.text(x+legendTextX, y+28, "CPU FRAMETIME", labelScale, textColor)
}

// drawOverflowNotes reports frames beyond the x-axis range, which are not drawn.
func drawOverflowNotes(c canvas, d distribution) {
	y := overflowY
	if d.cpuOverflow > 0 {
		c.text(legendX, y, fmt.Sprintf("CPU >%s MS: %d", formatAxisValue(d.xMax), d.cpuOverflow), noteScale, cpuColor)
		y += noteSpacing
	}
	if d.gpuOverflow > 0 {
		c.text(legendX, y, fmt.Sprintf("GPU >%s MS: %d", formatAxisValue(d.xMax), d.gpuOverflow), noteScale, gpuColor)
	}
}

func roundInt(value float64) int {
	return int(math.Round(value))
}
