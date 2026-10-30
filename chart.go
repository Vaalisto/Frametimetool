package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"sort"
	"unicode/utf8"
)

const (
	chartWidth  = 1900
	chartHeight = 700
	renderScale = 2
	plotLeft    = 100
	plotTop     = 100
	plotWidth   = 1540
	plotHeight  = 490
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
	if binCount < 1 || binCount > 5000 {
		return fmt.Errorf("bin count must be between 1 and 5000")
	}
	if len(times.cpuMS) == 0 || len(times.cpuMS) != len(times.gpuMS) {
		return fmt.Errorf("CPU and GPU timings must have the same non-zero number of frames")
	}
	var err error
	title, err = validateTitle(title)
	if err != nil {
		return err
	}

	allTimes := make([]float64, 0, len(times.cpuMS)+len(times.gpuMS))
	for _, series := range [][]float64{times.cpuMS, times.gpuMS} {
		for _, value := range series {
			if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("frame times must be finite, non-negative numbers")
			}
			allTimes = append(allTimes, value)
		}
	}
	// Use a high percentile so a few unusually slow frames do not flatten the
	// useful distribution. Samples beyond the displayed range are reported.
	axisMax := percentile(allTimes, 0.995)
	if axisMax <= math.MaxFloat64/1.05 {
		axisMax *= 1.05
	}
	xMax, xStep := niceAxisRange(axisMax)
	cpuBins := smoothHistogram(histogram(times.cpuMS, binCount, xMax), 1.8)
	gpuBins := smoothHistogram(histogram(times.gpuMS, binCount, xMax), 1.8)
	cpuPercent := toPercent(cpuBins, len(times.cpuMS))
	gpuPercent := toPercent(gpuBins, len(times.gpuMS))
	cpuOverflow := countAbove(times.cpuMS, xMax)
	gpuOverflow := countAbove(times.gpuMS, xMax)
	peakPercent := 0.0
	for i := range cpuPercent {
		peakPercent = math.Max(peakPercent, cpuPercent[i])
		peakPercent = math.Max(peakPercent, gpuPercent[i])
	}
	yMax := math.Ceil(peakPercent * 1.15)
	if yMax < 1 {
		yMax = 1
	}

	canvas := image.NewRGBA(image.Rect(0, 0, chartWidth*renderScale, chartHeight*renderScale))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)
	if title != "" {
		titleScale := 2
		if textWidth(title, titleScale) > chartWidth-80 {
			titleScale = 1
		}
		drawTextAt(canvas, (chartWidth-textWidth(title, titleScale))/2, 22, title, titleScale, textColor)
	}

	plotRight := plotLeft + plotWidth
	plotBottom := plotTop + plotHeight
	drawFilledCurve(canvas, gpuPercent, yMax, gpuColor)
	drawCurve(canvas, cpuPercent, yMax, cpuColor)

	// Fine, subdued gridlines are drawn over the curves like the reference chart.
	for tick := 0; tick <= 10; tick++ {
		y := plotBottom - tick*plotHeight/10
		drawLineAt(canvas, plotLeft, y, plotRight, y, gridColor)
		label := formatPercent(yMax * float64(tick) / 10)
		drawTextAt(canvas, plotLeft-12-textWidth(label, 2), y-7, label, 2, textColor)
	}
	tickCount := int(math.Round(xMax / xStep))
	for tick := 0; tick <= tickCount; tick++ {
		value := float64(tick) * xStep
		if tick == tickCount {
			value = xMax
		}
		x := plotLeft + int(math.Round(value/xMax*plotWidth))
		drawLineAt(canvas, x, plotTop, x, plotBottom, gridColor)
		label := formatAxisValue(value)
		drawTextAt(canvas, x-textWidth(label, 2)/2, plotBottom+8, label, 2, textColor)
	}
	drawLineAt(canvas, plotLeft, plotBottom, plotRight, plotBottom, axisColor)
	drawLineAt(canvas, plotLeft, plotTop, plotLeft, plotBottom, axisColor)

	drawTextAt(canvas, plotLeft+plotWidth/2-textWidth("MS", 2)/2, 650, "MS", 2, textColor)
	drawTextVerticalAt(canvas, 20, plotTop+plotHeight/2-textWidth("% OF FRAMES", 2)/2,
		"% OF FRAMES", 2, textColor)
	drawLegend(canvas, plotRight+42, plotTop+16)
	drawOverflowNotes(canvas, plotRight+42, plotTop+90, xMax, cpuOverflow, gpuOverflow)

	return png.Encode(output, downsample(canvas))
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

func percentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
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
	if sigma <= 0 {
		result := make([]float64, len(counts))
		for i, count := range counts {
			result[i] = float64(count)
		}
		return result
	}
	radius := int(math.Ceil(3 * sigma))
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

	smoothed := make([]float64, len(counts))
	originalTotal := 0.0
	smoothedTotal := 0.0
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

func niceAxisRange(dataMax float64) (maxValue, step float64) {
	if dataMax <= 0 || math.IsNaN(dataMax) || math.IsInf(dataMax, 0) {
		dataMax = 1
	}
	step = niceCeiling(dataMax / 8)
	maxValue = math.Ceil(dataMax/step) * step
	if math.IsInf(maxValue, 0) || maxValue < dataMax {
		return dataMax, dataMax / 8
	}
	return maxValue, step
}

func formatPercent(value float64) string {
	if value == math.Round(value) {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
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

func drawFilledCurve(canvas *image.RGBA, values []float64, yMax float64, fill color.RGBA) {
	baseline := (plotTop + plotHeight) * renderScale
	for i := 0; i < len(values)-1; i++ {
		x0 := float64(plotLeft)*renderScale + float64(i)*float64(plotWidth*renderScale)/float64(len(values)-1)
		x1 := float64(plotLeft)*renderScale + float64(i+1)*float64(plotWidth*renderScale)/float64(len(values)-1)
		y0 := float64(baseline) - values[i]/yMax*float64(plotHeight*renderScale)
		y1 := float64(baseline) - values[i+1]/yMax*float64(plotHeight*renderScale)
		startX, endX := int(math.Round(x0)), int(math.Round(x1))
		for x := startX; x <= endX; x++ {
			t := 0.0
			if x1 != x0 {
				t = (float64(x) - x0) / (x1 - x0)
			}
			y := int(math.Round(y0 + t*(y1-y0)))
			if y < plotTop*renderScale {
				y = plotTop * renderScale
			}
			for py := y; py <= baseline; py++ {
				canvas.SetRGBA(x, py, fill)
			}
		}
	}
}

func drawCurve(canvas *image.RGBA, values []float64, yMax float64, stroke color.RGBA) {
	for i := 0; i < len(values)-1; i++ {
		x0 := plotLeft + int(math.Round(float64(i)*float64(plotWidth)/float64(len(values)-1)))
		x1 := plotLeft + int(math.Round(float64(i+1)*float64(plotWidth)/float64(len(values)-1)))
		y0 := plotTop + plotHeight - int(math.Round(values[i]/yMax*plotHeight))
		y1 := plotTop + plotHeight - int(math.Round(values[i+1]/yMax*plotHeight))
		drawThickLineAt(canvas, x0, y0, x1, y1, stroke)
	}
}

func drawLegend(canvas *image.RGBA, x, y int) {
	drawThickLineAt(canvas, x, y+9, x+25, y+9, gpuColor)
	drawTextAt(canvas, x+34, y, "GPU FRAMETIME", 2, textColor)
	drawThickLineAt(canvas, x, y+37, x+25, y+37, cpuColor)
	drawTextAt(canvas, x+34, y+28, "CPU FRAMETIME", 2, textColor)
}

func drawOverflowNotes(canvas *image.RGBA, x, y int, maxValue float64, cpuCount, gpuCount int) {
	if cpuCount > 0 {
		note := fmt.Sprintf("CPU >%s MS: %d", formatAxisValue(maxValue), cpuCount)
		drawTextAt(canvas, x, y, note, 1, cpuColor)
		y += 20
	}
	if gpuCount > 0 {
		note := fmt.Sprintf("GPU >%s MS: %d", formatAxisValue(maxValue), gpuCount)
		drawTextAt(canvas, x, y, note, 1, gpuColor)
	}
}

func drawLineAt(canvas *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	drawLine(canvas, x0*renderScale, y0*renderScale, x1*renderScale, y1*renderScale, c)
}

func drawThickLineAt(canvas *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	drawLineAt(canvas, x0, y0, x1, y1, c)
	drawLine(canvas, x0*renderScale, y0*renderScale+1, x1*renderScale, y1*renderScale+1, c)
}

func drawLine(canvas *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	error := dx + dy
	for {
		if image.Pt(x0, y0).In(canvas.Bounds()) {
			canvas.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		doubleError := 2 * error
		if doubleError >= dy {
			error += dy
			x0 += sx
		}
		if doubleError <= dx {
			error += dx
			y0 += sy
		}
	}
}

func drawTextAt(canvas *image.RGBA, x, y int, text string, scale int, c color.RGBA) {
	drawText(canvas, x*renderScale, y*renderScale, text, scale*renderScale, c)
}

func drawTextVerticalAt(canvas *image.RGBA, x, y int, text string, scale int, c color.RGBA) {
	drawTextVertical(canvas, x*renderScale, y*renderScale, text, scale*renderScale, c)
}

func drawText(canvas *image.RGBA, x, y int, text string, scale int, c color.RGBA) {
	for _, char := range text {
		glyph, ok := glyphs[char]
		if char >= 'a' && char <= 'z' {
			glyph = glyphs[char-'a'+'A']
			ok = true
		}
		if !ok {
			glyph = glyphs['?']
		}
		for row, pixels := range glyph {
			for col, pixel := range pixels {
				if pixel == '1' {
					fillTextPixel(canvas, x+col*scale, y+row*scale, scale, c)
				}
			}
		}
		x += 6 * scale
	}
}

func drawTextVertical(canvas *image.RGBA, x, y int, text string, scale int, c color.RGBA) {
	// Rotate the label counterclockwise so it reads upward along the y-axis.
	for index, char := range text {
		glyph := glyphs[char]
		if char >= 'a' && char <= 'z' {
			glyph = glyphs[char-'a'+'A']
		}
		originY := y + (len(text)-index-1)*6*scale
		for row, pixels := range glyph {
			for col, pixel := range pixels {
				if pixel == '1' {
					left := x + (6-row)*scale
					top := originY + col*scale
					fillTextPixel(canvas, left, top, scale, c)
				}
			}
		}
	}
}

func fillTextPixel(canvas *image.RGBA, x, y, scale int, c color.RGBA) {
	area := image.Rect(x, y, x+scale, y+scale).Intersect(canvas.Bounds())
	if !area.Empty() {
		draw.Draw(canvas, area, &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
}

func textWidth(text string, scale int) int {
	charCount := utf8.RuneCountInString(text)
	if charCount == 0 {
		return 0
	}
	return (charCount*6 - 1) * scale
}

func downsample(source *image.RGBA) *image.RGBA {
	width, height := source.Bounds().Dx()/renderScale, source.Bounds().Dy()/renderScale
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var red, green, blue uint16
			for dy := 0; dy < renderScale; dy++ {
				for dx := 0; dx < renderScale; dx++ {
					c := source.RGBAAt(x*renderScale+dx, y*renderScale+dy)
					red += uint16(c.R)
					green += uint16(c.G)
					blue += uint16(c.B)
				}
			}
			result.SetRGBA(x, y, color.RGBA{
				R: uint8(red / (renderScale * renderScale)),
				G: uint8(green / (renderScale * renderScale)),
				B: uint8(blue / (renderScale * renderScale)),
				A: 255,
			})
		}
	}
	return result
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

var glyphs = map[rune][7]string{
	' ':  {"00000", "00000", "00000", "00000", "00000", "00000", "00000"},
	'(':  {"00110", "01100", "11000", "11000", "11000", "01100", "00110"},
	')':  {"01100", "00110", "00011", "00011", "00011", "00110", "01100"},
	'%':  {"11001", "11010", "00100", "00100", "01011", "10011", "00000"},
	'&':  {"01100", "10010", "10100", "01000", "10101", "10010", "01101"},
	'>':  {"10000", "01000", "00100", "00010", "00100", "01000", "10000"},
	'!':  {"00100", "00100", "00100", "00100", "00100", "00000", "00100"},
	'+':  {"00000", "00100", "00100", "11111", "00100", "00100", "00000"},
	',':  {"00000", "00000", "00000", "00000", "00110", "00100", "01000"},
	'-':  {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'.':  {"00000", "00000", "00000", "00000", "00000", "01100", "01100"},
	'/':  {"00001", "00010", "00010", "00100", "01000", "01000", "10000"},
	':':  {"00000", "00100", "00100", "00000", "00100", "00100", "00000"},
	'?':  {"01110", "10001", "00001", "00010", "00100", "00000", "00100"},
	'\'': {"00100", "00100", "00000", "00000", "00000", "00000", "00000"},
	'0':  {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1':  {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2':  {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3':  {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4':  {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5':  {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6':  {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7':  {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8':  {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9':  {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'A':  {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B':  {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C':  {"01110", "10001", "10000", "10000", "10000", "10001", "01110"},
	'D':  {"11100", "10010", "10001", "10001", "10001", "10010", "11100"},
	'E':  {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F':  {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G':  {"01110", "10001", "10000", "10111", "10001", "10001", "01110"},
	'H':  {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'I':  {"01110", "00100", "00100", "00100", "00100", "00100", "01110"},
	'J':  {"00111", "00010", "00010", "00010", "00010", "10010", "01100"},
	'K':  {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L':  {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M':  {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N':  {"10001", "11001", "11001", "10101", "10011", "10011", "10001"},
	'O':  {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
	'P':  {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q':  {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R':  {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S':  {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T':  {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U':  {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V':  {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W':  {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'X':  {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y':  {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z':  {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
}
