package main

import (
	"strings"
	"testing"
	"time"
)

func TestRenderSwitchForbidCS(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Prague")
	now := time.Date(2026, 6, 7, 13, 30, 0, 0, loc)
	d := Decision{Forbid: true, TriggerPrice: -50.8}
	o := Outlook{HasRun: true,
		RunStart:  time.Date(2026, 6, 7, 14, 0, 0, 0, loc),
		FlipAt:    time.Date(2026, 6, 7, 15, 30, 0, 0, loc),
		FlipKnown: true}
	info := RealtimeInfo{Soc: 98, FeedinPower: 64}
	msg := RenderSwitch(csMessages, d, info, now, 0, o, true)

	for _, want := range []string{"Zastavuji prodej", "-50.8", "14:00", "15:30", "obnovíme", "98", "13:30"} {
		if !strings.Contains(msg, want) {
			t.Errorf("forbid message missing %q\n---\n%s", want, msg)
		}
	}
}

func TestRenderSwitchAllowCS(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Prague")
	now := time.Date(2026, 6, 7, 15, 0, 0, 0, loc)
	d := Decision{Forbid: false, TriggerPrice: 42.5}
	o := Outlook{HasRun: true, FlipAt: time.Date(2026, 6, 7, 18, 0, 0, 0, loc), FlipKnown: true}
	msg := RenderSwitch(csMessages, d, RealtimeInfo{Soc: 80, FeedinPower: 1200}, now, 0, o, true)

	for _, want := range []string{"Obnovuji prodej", "42.5", "15:00", "18:00", "Další záporné"} {
		if !strings.Contains(msg, want) {
			t.Errorf("allow message missing %q\n---\n%s", want, msg)
		}
	}
}

func TestRenderSwitchForbidEN(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Prague")
	now := time.Date(2026, 6, 7, 13, 30, 0, 0, loc)
	d := Decision{Forbid: true, TriggerPrice: -50.8}
	o := Outlook{HasRun: true,
		RunStart:  time.Date(2026, 6, 7, 14, 0, 0, 0, loc),
		FlipAt:    time.Date(2026, 6, 7, 15, 30, 0, 0, loc),
		FlipKnown: true}
	info := RealtimeInfo{Soc: 98, FeedinPower: 64}
	msg := RenderSwitch(enMessages, d, info, now, 0, o, true)

	for _, want := range []string{"Stopping grid export", "-50.8", "14:00", "15:30", "Export resumes", "98", "13:30"} {
		if !strings.Contains(msg, want) {
			t.Errorf("forbid message missing %q\n---\n%s", want, msg)
		}
	}
}

func TestRenderSwitchAllowEN(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Prague")
	now := time.Date(2026, 6, 7, 15, 0, 0, 0, loc)
	d := Decision{Forbid: false, TriggerPrice: 42.5}
	o := Outlook{HasRun: true, FlipAt: time.Date(2026, 6, 7, 18, 0, 0, 0, loc), FlipKnown: true}
	msg := RenderSwitch(enMessages, d, RealtimeInfo{Soc: 80, FeedinPower: 1200}, now, 0, o, true)

	for _, want := range []string{"Resuming grid export", "42.5", "15:00", "18:00", "Next negative-price"} {
		if !strings.Contains(msg, want) {
			t.Errorf("allow message missing %q\n---\n%s", want, msg)
		}
	}
}

// messagesFor falls back to English for unknown language codes.
func TestMessagesForFallback(t *testing.T) {
	if got := messagesFor("zz"); got.ForbidTitle != enMessages.ForbidTitle {
		t.Errorf("messagesFor(unknown) = %q, want English default", got.ForbidTitle)
	}
	if got := messagesFor("cs"); got.ForbidTitle != csMessages.ForbidTitle {
		t.Errorf("messagesFor(cs) did not return the Czech bundle")
	}
}
