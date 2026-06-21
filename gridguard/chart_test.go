package main

import (
	"bytes"
	"image/png"
	"testing"
	"time"
)

func TestRenderPriceChartPNG(t *testing.T) {
	loc := pragueLoc(t)
	prices := make([]float64, 96)
	for i := range prices {
		prices[i] = 50
	}
	prices[50], prices[51], prices[52] = -10, -25, -5 // ~12:30-13:15 negative
	qs := qtrs96(loc, prices)
	now := time.Date(2026, 6, 7, 12, 40, 0, 0, loc)

	pngBytes, err := RenderPriceChartPNG(qs, now, 0, loc)
	if err != nil {
		t.Fatalf("RenderPriceChartPNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 900 || b.Dy() != 360 {
		t.Errorf("chart size = %dx%d, want 900x360", b.Dx(), b.Dy())
	}
}

func TestRenderPriceChartPNGNoDataForToday(t *testing.T) {
	loc := pragueLoc(t)
	// quarters are for 2026-06-07 but "now" is a different day -> no quarters match -> error
	qs := qtrs96(loc, make([]float64, 96))
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, loc)
	if _, err := RenderPriceChartPNG(qs, now, 0, loc); err == nil {
		t.Errorf("expected error when no quarters fall on now's date")
	}
}

func qtrs96(loc *time.Location, prices []float64) []Quarter {
	day := time.Date(2026, 6, 7, 0, 0, 0, 0, loc)
	out := make([]Quarter, len(prices))
	for i, p := range prices {
		out[i] = Quarter{Index: i + 1, Start: day.Add(time.Duration(i) * 15 * time.Minute), Price: p}
	}
	return out
}
