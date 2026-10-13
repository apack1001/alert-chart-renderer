package alertchartrenderer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"time"
)

var defaultPalette = []color.RGBA{
	{R: 0x57, G: 0x94, B: 0xF2, A: 0xFF}, // Blue
	{R: 0x73, G: 0xBF, B: 0x69, A: 0xFF}, // Green
	{R: 0xFA, G: 0xDE, B: 0x2A, A: 0xFF}, // Yellow
	{R: 0xFF, G: 0x78, B: 0x0A, A: 0xFF}, // Orange
	{R: 0xB8, G: 0x77, B: 0xD9, A: 0xFF}, // Purple
}

// RenderChart builds a high-fidelity time-series snapshot image based on RenderOptions.
func RenderChart(opt RenderOptions) ([]byte, error) {
	width := opt.Width
	if width <= 0 {
		width = 800
	}
	height := opt.Height
	if height <= 0 {
		height = 360
	}

	// Colors setup
	isLight := opt.Theme == "light"
	var bgColor, gridColor, axisColor, textColor, subTextColor color.RGBA

	if isLight {
		bgColor = color.RGBA{R: 0xF8, G: 0xF9, B: 0xFA, A: 0xFF}
		gridColor = color.RGBA{R: 0xE5, G: 0xE7, B: 0xEB, A: 0xFF}
		axisColor = color.RGBA{R: 0x9C, G: 0xA3, B: 0xAF, A: 0xFF}
		textColor = color.RGBA{R: 0x11, G: 0x18, B: 0x27, A: 0xFF}
		subTextColor = color.RGBA{R: 0x6B, G: 0x72, B: 0x80, A: 0xFF}
	} else {
		bgColor = color.RGBA{R: 0x18, G: 0x1B, B: 0x1F, A: 0xFF}
		gridColor = color.RGBA{R: 0x2B, G: 0x30, B: 0x3A, A: 0xFF}
		axisColor = color.RGBA{R: 0x4B, G: 0x55, B: 0x63, A: 0xFF}
		textColor = color.RGBA{R: 0xF3, G: 0xF4, B: 0xF6, A: 0xFF}
		subTextColor = color.RGBA{R: 0x9C, G: 0xA3, B: 0xAF, A: 0xFF}
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Layout Paddings
	padTop := 50
	padBottom := 50
	padLeft := 70
	padRight := 40

	plotW := width - padLeft - padRight
	plotH := height - padTop - padBottom

	// Top Title and Subtitle
	if opt.Title != "" {
		DrawText(img, opt.Title, padLeft, 16, textColor, 2)
	}
	if opt.Subtitle != "" {
		DrawText(img, opt.Subtitle, padLeft, 36, subTextColor, 1)
	}

	// Calculate bounds
	var minX, maxX int64 = math.MaxInt64, math.MinInt64
	var minY, maxY float64 = math.MaxFloat64, -math.MaxFloat64
	totalPoints := 0

	for _, s := range opt.Series {
		for _, pt := range s.Points {
			totalPoints++
			if pt.Timestamp < minX {
				minX = pt.Timestamp
			}
			if pt.Timestamp > maxX {
				maxX = pt.Timestamp
			}
			if pt.Value < minY {
				minY = pt.Value
			}
			if pt.Value > maxY {
				maxY = pt.Value
			}
		}
	}

	for _, th := range opt.Thresholds {
		if th.Value < minY {
			minY = th.Value
		}
		if th.Value > maxY {
			maxY = th.Value
		}
	}

	if totalPoints == 0 {
		DrawText(img, "No Data Points Available", width/2-80, height/2, subTextColor, 2)
		return encodePNG(img)
	}

	if minX == maxX {
		minX -= 60
		maxX += 60
	}
	if minY == maxY {
		minY -= 1
		maxY += 1
	}

	// Add 10% Y padding
	yRange := maxY - minY
	minY -= yRange * 0.05
	maxY += yRange * 0.05
	if minY < 0 && minY > -yRange*0.1 {
		minY = 0 // Align to zero for positive metrics
	}

	mapX := func(t int64) int {
		ratio := float64(t-minX) / float64(maxX-minX)
		return padLeft + int(ratio*float64(plotW))
	}

	mapY := func(v float64) int {
		ratio := (v - minY) / (maxY - minY)
		return padTop + plotH - int(ratio*float64(plotH))
	}

	// Draw Grid Lines (Horizontal & Vertical)
	gridStepsY := 5
	for i := 0; i <= gridStepsY; i++ {
		curY := padTop + int(float64(plotH)/float64(gridStepsY)*float64(i))
		valY := maxY - (maxY-minY)*(float64(i)/float64(gridStepsY))

		// Line
		drawLine(img, padLeft, curY, padLeft+plotW, curY, gridColor, 1)

		// Label
		lbl := fmt.Sprintf("%.1f%s", valY, opt.Unit)
		DrawText(img, lbl, padLeft-len(lbl)*7-8, curY-4, subTextColor, 1)
	}

	gridStepsX := 5
	for i := 0; i <= gridStepsX; i++ {
		curX := padLeft + int(float64(plotW)/float64(gridStepsX)*float64(i))
		timeUnix := minX + int64(float64(maxX-minX)*(float64(i)/float64(gridStepsX)))
		t := time.Unix(timeUnix, 0)

		// Line
		drawLine(img, curX, padTop, curX, padTop+plotH, gridColor, 1)

		// Time label (HH:MM)
		timeStr := t.Format("15:04")
		DrawText(img, timeStr, curX-15, padTop+plotH+12, subTextColor, 1)
	}

	// Draw Axis Borders
	drawLine(img, padLeft, padTop, padLeft, padTop+plotH, axisColor, 1)
	drawLine(img, padLeft, padTop+plotH, padLeft+plotW, padTop+plotH, axisColor, 1)

	// Draw Threshold Lines (Dashed Red)
	for _, th := range opt.Thresholds {
		thY := mapY(th.Value)
		thColor := parseHexColor(th.Color, color.RGBA{R: 0xF2, G: 0x49, B: 0x5C, A: 0xFF})
		drawDashedLine(img, padLeft, thY, padLeft+plotW, thY, thColor, 6, 4)

		// Threshold label
		thLabel := fmt.Sprintf("%s: %.1f", th.Label, th.Value)
		DrawText(img, thLabel, padLeft+plotW-len(thLabel)*7-6, thY-12, thColor, 1)
	}

	// Draw TimeSeries Curves
	for sIdx, s := range opt.Series {
		if len(s.Points) == 0 {
			continue
		}
		seriesColor := parseHexColor(s.Color, defaultPalette[sIdx%len(defaultPalette)])

		for i := 0; i < len(s.Points)-1; i++ {
			p1 := s.Points[i]
			p2 := s.Points[i+1]
			x1, y1 := mapX(p1.Timestamp), mapY(p1.Value)
			x2, y2 := mapX(p2.Timestamp), mapY(p2.Value)

			drawLine(img, x1, y1, x2, y2, seriesColor, 2)
		}

		// Draw Points highlight
		for _, p := range s.Points {
			px, py := mapX(p.Timestamp), mapY(p.Value)
			drawCircle(img, px, py, 2, seriesColor)
		}
	}

	// Draw Legend (Bottom)
	legendX := padLeft
	legendY := height - 16
	for sIdx, s := range opt.Series {
		sColor := parseHexColor(s.Color, defaultPalette[sIdx%len(defaultPalette)])
		drawRect(img, legendX, legendY-1, 10, 8, sColor)
		legendX += 14

		name := s.Name
		if name == "" {
			name = fmt.Sprintf("Series %d", sIdx+1)
		}
		DrawText(img, name, legendX, legendY, textColor, 1)
		legendX += len(name)*7 + 20
		if legendX > width-padRight {
			break
		}
	}

	return encodePNG(img)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color, thickness int) {
	dx := math.Abs(float64(x1 - x0))
	dy := math.Abs(float64(y1 - y0))
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	errVal := dx - dy

	for {
		for tx := -thickness / 2; tx <= thickness/2; tx++ {
			for ty := -thickness / 2; ty <= thickness/2; ty++ {
				px, py := x0+tx, y0+ty
				if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
					img.Set(px, py, c)
				}
			}
		}

		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * errVal
		if e2 > -dy {
			errVal -= dy
			x0 += sx
		}
		if e2 < dx {
			errVal += dx
			y0 += sy
		}
	}
}

func drawDashedLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color, dashLen, gapLen int) {
	dx := float64(x1 - x0)
	dy := float64(y1 - y0)
	dist := math.Hypot(dx, dy)
	if dist == 0 {
		return
	}
	steps := int(dist)
	unitX := dx / dist
	unitY := dy / dist

	drawing := true
	accum := 0

	for i := 0; i <= steps; i++ {
		if drawing {
			px := x0 + int(unitX*float64(i))
			py := y0 + int(unitY*float64(i))
			if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
				img.Set(px, py, c)
			}
			accum++
			if accum >= dashLen {
				drawing = false
				accum = 0
			}
		} else {
			accum++
			if accum >= gapLen {
				drawing = true
				accum = 0
			}
		}
	}
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	for x := -r; x <= r; x++ {
		for y := -r; y <= r; y++ {
			if x*x+y*y <= r*r {
				px, py := cx+x, cy+y
				if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
					img.Set(px, py, c)
				}
			}
		}
	}
}

func drawRect(img *image.RGBA, x, y, w, h int, c color.Color) {
	for px := x; px < x+w; px++ {
		for py := y; py < y+h; py++ {
			if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
				img.Set(px, py, c)
			}
		}
	}
}

func parseHexColor(s string, fallback color.RGBA) color.RGBA {
	if len(s) == 7 && s[0] == '#' {
		r, errR := strconv.ParseUint(s[1:3], 16, 8)
		g, errG := strconv.ParseUint(s[3:5], 16, 8)
		b, errB := strconv.ParseUint(s[5:7], 16, 8)
		if errR == nil && errG == nil && errB == nil {
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xFF}
		}
	}
	return fallback
}
