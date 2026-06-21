package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
)

const (
	chartW    = 900
	chartH    = 360
	chartPadL = 54
	chartPadR = 22
	chartPadT = 66
	chartPadB = 42
)

var (
	colBG    = color.RGBA{0x0e, 0x12, 0x17, 0xff}
	colPos   = color.RGBA{0x3f, 0xb2, 0x7e, 0xff} // legible green
	colNeg   = color.RGBA{0xff, 0x5d, 0x5d, 0xff} // red: the signal
	colGrid  = color.RGBA{0x20, 0x29, 0x33, 0xff}
	colZero  = color.RGBA{0x45, 0x52, 0x60, 0xff}
	colNow   = color.RGBA{0xf2, 0xb5, 0x3c, 0xff}
	colTitle = color.RGBA{0xed, 0xf1, 0xf6, 0xff}
	colLabel = color.RGBA{0x93, 0x9f, 0xad, 0xff}
)

// RenderPriceChartPNG draws today's quarter-hour spot prices as a clean, legible PNG.
// In-image text is ASCII only (basicfont/inconsolata lack Czech glyphs); the rich
// Czech wording lives in the Telegram caption instead.
func RenderPriceChartPNG(qs []Quarter, now time.Time, threshold float64, loc *time.Location) ([]byte, error) {
	day := now.In(loc)
	var pts []Quarter
	for _, q := range qs {
		s := q.Start.In(loc)
		if s.Year() == day.Year() && s.YearDay() == day.YearDay() {
			pts = append(pts, q)
		}
	}
	if len(pts) == 0 {
		return nil, fmt.Errorf("no quarters for chart")
	}

	img := image.NewRGBA(image.Rect(0, 0, chartW, chartH))
	draw.Draw(img, img.Bounds(), &image.Uniform{colBG}, image.Point{}, draw.Src)

	plotL, plotR := chartPadL, chartW-chartPadR
	plotT, plotB := chartPadT, chartH-chartPadB
	plotW := plotR - plotL
	plotH := plotB - plotT

	rawMin, rawMax := math.Inf(1), math.Inf(-1)
	for _, q := range pts {
		rawMin = min(rawMin, q.Price)
		rawMax = max(rawMax, q.Price)
	}
	if rawMin > 0 {
		rawMin = 0
	}
	if rawMax < 0 {
		rawMax = 0
	}
	pad := (rawMax - rawMin) * 0.10
	if pad == 0 {
		pad = 1
	}
	scaleMax, scaleMin := rawMax+pad, rawMin-pad
	span := scaleMax - scaleMin
	if span == 0 {
		span = 1
	}
	yOf := func(p float64) int { return plotT + int(float64(plotH)*(scaleMax-p)/span) }
	minOfDay := func(t time.Time) int { tt := t.In(loc); return tt.Hour()*60 + tt.Minute() }
	xAtMin := func(m int) int { return plotL + int(float64(plotW)*float64(m)/(24*60)) }
	zeroY := yOf(0)

	num := inconsolata.Regular8x16

	// hairline horizontal gridlines + value labels
	for _, lv := range niceLevels(scaleMin, scaleMax, 4) {
		y := yOf(lv)
		if y < plotT || y > plotB {
			continue
		}
		drawHLine(img, plotL, plotR, y, colGrid)
		drawStringRight(img, num, colLabel, plotL-8, y+5, fmt.Sprintf("%g", lv))
	}

	// hour labels every 3h, faint verticals at 6/12/18
	for h := 0; h <= 24; h += 3 {
		x := plotL + int(float64(plotW)*float64(h)/24.0)
		if h%6 == 0 && h != 0 && h != 24 {
			drawVLine(img, x, plotT, plotB, colGrid)
		}
		drawStringCenter(img, num, colLabel, x, plotB+19, fmt.Sprintf("%02d", h))
	}

	// bars
	for _, q := range pts {
		m0 := minOfDay(q.Start)
		x0, x1 := xAtMin(m0), xAtMin(m0+QuarterMinutes)-1
		if x1 <= x0 {
			x1 = x0 + 1
		}
		c := colPos
		var top, bot int
		if q.Price >= 0 {
			top, bot = yOf(q.Price), zeroY
		} else {
			top, bot, c = zeroY, yOf(q.Price), colNeg
		}
		fillRect(img, x0, top, x1, bot, c)
	}

	// crisp zero baseline over the bars
	drawHLine(img, plotL, plotR, zeroY, colZero)

	// now marker with a filled time tag
	nowFrac := float64(day.Hour()*60+day.Minute()) / float64(24*60)
	nx := plotL + int(float64(plotW)*nowFrac)
	drawVLine(img, nx, plotT, plotB, colNow)
	tag := day.Format("15:04")
	tw := measureString(num, tag)
	tagX := nx + 6
	if tagX+tw+8 > plotR {
		tagX = nx - 6 - (tw + 8)
	}
	fillRect(img, tagX, plotT-1, tagX+tw+8, plotT+19, colNow)
	drawString(img, num, colBG, tagX+4, plotT+14, tag)

	// header: title + date (left), current price + unit (right)
	drawString(img, inconsolata.Bold8x16, colTitle, 22, 30, "SPOT PRICE")
	drawString(img, basicfont.Face7x13, colLabel, 22, 48, day.Format("Mon 2 Jan 2006"))
	if cur, ok := priceAt(pts, now); ok {
		cc := colPos
		if cur <= threshold {
			cc = colNeg
		}
		val := fmt.Sprintf("%.0f", cur)
		drawStringRight(img, inconsolata.Bold8x16, cc, plotR, 30, val)
		unitX := plotR - measureString(inconsolata.Bold8x16, val) - 8
		drawStringRight(img, basicfont.Face7x13, colLabel, unitX, 29, "EUR/MWh")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{c}, image.Point{}, draw.Src)
}

func drawHLine(img *image.RGBA, x0, x1, y int, c color.Color) { fillRect(img, x0, y, x1, y+1, c) }
func drawVLine(img *image.RGBA, x, y0, y1 int, c color.Color) { fillRect(img, x, y0, x+1, y1, c) }

func measureString(face font.Face, s string) int { return font.MeasureString(face, s).Round() }

func drawString(img *image.RGBA, face font.Face, c color.Color, x, y int, s string) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

func drawStringRight(img *image.RGBA, face font.Face, c color.Color, xRight, y int, s string) {
	drawString(img, face, c, xRight-measureString(face, s), y, s)
}

func drawStringCenter(img *image.RGBA, face font.Face, c color.Color, xCenter, y int, s string) {
	drawString(img, face, c, xCenter-measureString(face, s)/2, y, s)
}

func niceLevels(minP, maxP float64, n int) []float64 {
	rng := maxP - minP
	if rng <= 0 {
		return []float64{0}
	}
	raw := rng / float64(n)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	norm := raw / mag
	var step float64
	switch {
	case norm < 1.5:
		step = 1
	case norm < 3:
		step = 2
	case norm < 7:
		step = 5
	default:
		step = 10
	}
	step *= mag
	var out []float64
	for v := math.Ceil(minP/step) * step; v <= maxP+1e-9; v += step {
		out = append(out, math.Round(v*1000)/1000)
	}
	return out
}
