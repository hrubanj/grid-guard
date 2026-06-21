package main

import (
	"os"
	"testing"
	"time"
)

func pragueLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestParseQuarters(t *testing.T) {
	loc := pragueLoc(t)
	body, err := os.ReadFile("testdata/ote_sample.json")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 6, 7, 0, 0, 0, 0, loc)

	qs, err := parseQuarters(body, day, loc)
	if err != nil {
		t.Fatalf("parseQuarters: %v", err)
	}
	if len(qs) != 4 {
		t.Fatalf("got %d quarters, want 4", len(qs))
	}
	q3 := qs[2]
	if q3.Index != 3 {
		t.Errorf("q3.Index = %d, want 3", q3.Index)
	}
	wantStart := time.Date(2026, 6, 7, 0, 30, 0, 0, loc)
	if !q3.Start.Equal(wantStart) {
		t.Errorf("q3.Start = %v, want %v", q3.Start, wantStart)
	}
	if q3.Price != -50.8 {
		t.Errorf("q3.Price = %v, want -50.8", q3.Price)
	}
}

func qtrs(loc *time.Location, prices ...float64) []Quarter {
	day := time.Date(2026, 6, 7, 0, 0, 0, 0, loc)
	out := make([]Quarter, len(prices))
	for i, p := range prices {
		out[i] = Quarter{Index: i + 1, Start: day.Add(time.Duration(i) * 15 * time.Minute), Price: p}
	}
	return out
}

func TestGuardForbidsJustBeforeNegativeQuarter(t *testing.T) {
	loc := pragueLoc(t)
	// q1 00:00-00:15=10, q2 00:15-00:30=-5, q3 00:30-00:45=20
	qs := qtrs(loc, 10, -5, 20)
	look := 5 * time.Minute

	// at 00:12, window [00:12,00:17] touches q2 (-5) -> forbid
	d := Guard(qs, time.Date(2026, 6, 7, 0, 12, 0, 0, loc), look, 0)
	if !d.Forbid {
		t.Errorf("expected forbid at 00:12 (negative quarter within 5 min)")
	}
	if d.TriggerPrice != -5 {
		t.Errorf("trigger = %v, want -5", d.TriggerPrice)
	}

	// at 00:05, window [00:05,00:10] is entirely in q1 (10) -> allow
	d = Guard(qs, time.Date(2026, 6, 7, 0, 5, 0, 0, loc), look, 0)
	if d.Forbid {
		t.Errorf("expected allow at 00:05 (still positive)")
	}

	// at 00:20 we are inside q2 (-5) -> forbid
	d = Guard(qs, time.Date(2026, 6, 7, 0, 20, 0, 0, loc), look, 0)
	if !d.Forbid {
		t.Errorf("expected forbid at 00:20 (inside negative quarter)")
	}
}

func TestGuardThresholdInclusiveZero(t *testing.T) {
	loc := pragueLoc(t)
	qs := qtrs(loc, 0, 5) // exactly zero is non-positive -> forbidden
	d := Guard(qs, time.Date(2026, 6, 7, 0, 5, 0, 0, loc), 5*time.Minute, 0)
	if !d.Forbid {
		t.Errorf("price == 0 must be forbidden (<=)")
	}
}

func TestGuardNoDataNotFound(t *testing.T) {
	loc := pragueLoc(t)
	qs := qtrs(loc, 10) // only 00:00-00:15
	d := Guard(qs, time.Date(2026, 6, 7, 23, 59, 0, 0, loc), 5*time.Minute, 0)
	if d.Found {
		t.Errorf("expected Found=false when window has no price data")
	}
}

func TestGuardTriggerIsFirstBelowThreshold(t *testing.T) {
	loc := pragueLoc(t)
	// q1 -3 (00:00-00:15), q2 -20 (00:15-00:30): both negative, first is -3.
	qs := qtrs(loc, -3, -20)
	// window [00:05,00:35] overlaps both negative quarters
	d := Guard(qs, time.Date(2026, 6, 7, 0, 5, 0, 0, loc), 30*time.Minute, 0)
	if !d.Forbid {
		t.Fatalf("expected forbid")
	}
	if d.TriggerPrice != -3 {
		t.Errorf("TriggerPrice = %v, want -3 (first below threshold, not window min -20)", d.TriggerPrice)
	}
}

func TestPriceOutlookNegativeRunWithKnownFlip(t *testing.T) {
	loc := pragueLoc(t)
	// q1 10, q2..q4 = -5, q5 20 ; below side run = q2..q4 (00:15..01:00), flips at 01:00
	qs := qtrs(loc, 10, -5, -5, -5, 20)
	o := PriceOutlook(qs, time.Date(2026, 6, 7, 0, 18, 0, 0, loc), 0, true)
	if !o.HasRun {
		t.Fatal("expected a below-threshold run")
	}
	if !o.RunStart.Equal(time.Date(2026, 6, 7, 0, 15, 0, 0, loc)) {
		t.Errorf("RunStart=%v want 00:15", o.RunStart)
	}
	if !o.FlipAt.Equal(time.Date(2026, 6, 7, 1, 0, 0, 0, loc)) {
		t.Errorf("FlipAt=%v want 01:00", o.FlipAt)
	}
	if !o.FlipKnown {
		t.Errorf("expected FlipKnown true (q5 is positive)")
	}
}

func TestPriceOutlookPositiveRunNextNegative(t *testing.T) {
	loc := pragueLoc(t)
	// positive now, next negative at q3 (00:30)
	qs := qtrs(loc, 10, 20, -5, -5)
	o := PriceOutlook(qs, time.Date(2026, 6, 7, 0, 5, 0, 0, loc), 0, false)
	if !o.HasRun {
		t.Fatal("expected a positive run")
	}
	if !o.FlipKnown {
		t.Errorf("expected FlipKnown true (negative follows)")
	}
	if !o.FlipAt.Equal(time.Date(2026, 6, 7, 0, 30, 0, 0, loc)) {
		t.Errorf("FlipAt=%v want 00:30 (next negative)", o.FlipAt)
	}
}

func TestPriceOutlookRunToDataEndUnknownFlip(t *testing.T) {
	loc := pragueLoc(t)
	// negative run reaches the end of data -> FlipKnown false
	qs := qtrs(loc, 10, -5, -5)
	o := PriceOutlook(qs, time.Date(2026, 6, 7, 0, 18, 0, 0, loc), 0, true)
	if !o.HasRun {
		t.Fatal("expected a below run")
	}
	if o.FlipKnown {
		t.Errorf("expected FlipKnown false at data boundary")
	}
}

func TestPriceOutlookNoRunOnSide(t *testing.T) {
	loc := pragueLoc(t)
	qs := qtrs(loc, 10, 20, 30)                                            // all positive
	o := PriceOutlook(qs, time.Date(2026, 6, 7, 0, 5, 0, 0, loc), 0, true) // ask for below side
	if o.HasRun {
		t.Errorf("expected HasRun false (no below-threshold quarter)")
	}
}

func TestParseQuartersDSTSpringForward(t *testing.T) {
	loc := pragueLoc(t)
	// 2026-03-29: clocks jump 02:00 CET -> 03:00 CEST, so 02:xx does not exist.
	body := []byte(`{"data":{"dataLine":[{"title":"15min cena (EUR/MWh)","point":[{"x":"8","y":10},{"x":"9","y":20}]}]}}`)
	day := time.Date(2026, 3, 29, 0, 0, 0, 0, loc)
	qs, err := parseQuarters(body, day, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := qs[0].Start.Format("15:04 MST"); got != "01:45 CET" {
		t.Errorf("idx 8 start = %s, want 01:45 CET", got)
	}
	if got := qs[1].Start.Format("15:04 MST"); got != "03:00 CEST" {
		t.Errorf("idx 9 start = %s, want 03:00 CEST (spring-forward skips 02:00)", got)
	}
}

func TestParseQuartersDSTFallBack(t *testing.T) {
	loc := pragueLoc(t)
	// 2026-10-25: clocks fall 03:00 CEST -> 02:00 CET, so 02:xx repeats.
	body := []byte(`{"data":{"dataLine":[{"title":"15min cena (EUR/MWh)","point":[{"x":"9","y":10},{"x":"13","y":20}]}]}}`)
	day := time.Date(2026, 10, 25, 0, 0, 0, 0, loc)
	qs, err := parseQuarters(body, day, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got := qs[0].Start.Format("15:04 MST"); got != "02:00 CEST" {
		t.Errorf("idx 9 start = %s, want 02:00 CEST (first pass)", got)
	}
	if got := qs[1].Start.Format("15:04 MST"); got != "02:00 CET" {
		t.Errorf("idx 13 start = %s, want 02:00 CET (repeated hour)", got)
	}
	if !qs[1].Start.After(qs[0].Start) {
		t.Errorf("idx 13 must be a later absolute instant than idx 9 (the repeat)")
	}
}
