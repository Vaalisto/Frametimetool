package main

import (
	"image"
	"image/color"
	"image/draw"
	"unicode/utf8"
)

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
