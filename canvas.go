package main

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// renderScale supersamples the chart for smoother edges; toImage downsamples it again.
const renderScale = 2

// canvas draws in logical chart coordinates. Only its methods convert to render pixels.
type canvas struct {
	img *image.RGBA
}

func newCanvas(width, height int, background color.RGBA) canvas {
	img := image.NewRGBA(image.Rect(0, 0, width*renderScale, height*renderScale))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)
	return canvas{img: img}
}

func (cv canvas) line(x0, y0, x1, y1 int, c color.RGBA) {
	cv.renderLine(x0*renderScale, y0*renderScale, x1*renderScale, y1*renderScale, c)
}

// thickLine draws a line one logical pixel wide, using two render pixels.
func (cv canvas) thickLine(x0, y0, x1, y1 int, c color.RGBA) {
	cv.line(x0, y0, x1, y1, c)
	cv.renderLine(x0*renderScale, y0*renderScale+1, x1*renderScale, y1*renderScale+1, c)
}

// fillUnder fills the area between the segment (x0,y0)-(x1,y1) and a baseline.
func (cv canvas) fillUnder(x0, y0, x1, y1, baseline float64, c color.RGBA) {
	s := float64(renderScale)
	bottom := int(math.Round(baseline * s))
	startX, endX := int(math.Round(x0*s)), int(math.Round(x1*s))
	for x := startX; x <= endX; x++ {
		t := 0.0
		if x1 != x0 {
			t = (float64(x) - x0*s) / ((x1 - x0) * s)
		}
		top := int(math.Round((y0 + t*(y1-y0)) * s))
		for y := top; y <= bottom; y++ {
			cv.img.SetRGBA(x, y, c)
		}
	}
}

// text draws s left to right. scale is the size of one glyph pixel in logical units.
func (cv canvas) text(x, y int, s string, scale int, c color.RGBA) {
	x, y, size := x*renderScale, y*renderScale, scale*renderScale
	for _, r := range s {
		for row, pixels := range glyphFor(r) {
			for column, pixel := range pixels {
				if pixel == '1' {
					cv.fillBlock(x+column*size, y+row*size, size, c)
				}
			}
		}
		x += glyphAdvance * size
	}
}

// verticalText draws s rotated counter-clockwise, so it reads upward. (x, y) is
// the bottom of the text: the first rune sits at y and later runes are above it.
func (cv canvas) verticalText(x, y int, s string, scale int, c color.RGBA) {
	x, y, size := x*renderScale, y*renderScale, scale*renderScale
	for index, r := range []rune(s) {
		bottom := y - index*glyphAdvance*size
		for row, pixels := range glyphFor(r) {
			for column, pixel := range pixels {
				if pixel == '1' {
					cv.fillBlock(x+row*size, bottom-(column+1)*size, size, c)
				}
			}
		}
	}
}

// toImage downsamples the supersampled canvas to logical size.
func (cv canvas) toImage() *image.RGBA {
	width := cv.img.Bounds().Dx() / renderScale
	height := cv.img.Bounds().Dy() / renderScale
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	samples := renderScale * renderScale
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var red, green, blue int
			for dy := 0; dy < renderScale; dy++ {
				for dx := 0; dx < renderScale; dx++ {
					c := cv.img.RGBAAt(x*renderScale+dx, y*renderScale+dy)
					red += int(c.R)
					green += int(c.G)
					blue += int(c.B)
				}
			}
			result.SetRGBA(x, y, color.RGBA{
				R: uint8(red / samples),
				G: uint8(green / samples),
				B: uint8(blue / samples),
				A: 255,
			})
		}
	}
	return result
}

// fillBlock fills a square of render pixels, clipped to the canvas.
func (cv canvas) fillBlock(x, y, size int, c color.RGBA) {
	area := image.Rect(x, y, x+size, y+size).Intersect(cv.img.Bounds())
	if !area.Empty() {
		draw.Draw(cv.img, area, &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
}

// renderLine is Bresenham's algorithm in render pixels.
func (cv canvas) renderLine(x0, y0, x1, y1 int, c color.RGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(cv.img.Bounds()) {
			cv.img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		doubleErr := 2 * err
		if doubleErr >= dy {
			err += dy
			x0 += sx
		}
		if doubleErr <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
