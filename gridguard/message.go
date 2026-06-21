package main

import (
	"fmt"
	"strings"
	"time"
)

// RenderSwitch builds the localized Telegram-HTML message for an allow<->forbid
// switch, using the message bundle m (see messages.go). o is the price outlook on
// the *current* side (below==d.Forbid).
func RenderSwitch(m Messages, d Decision, info RealtimeInfo, now time.Time, threshold float64, o Outlook, telemetryOK bool) string {
	ts := now.Format(m.ClockLayout)
	date := now.Format(m.DateLayout)
	var b strings.Builder

	if d.Forbid {
		b.WriteString(m.ForbidTitle)
		b.WriteString(fmt.Sprintf(m.ForbidPriceFmt, fmtPrice(d.TriggerPrice), fmtPrice(threshold)))
		if o.HasRun {
			if o.FlipKnown {
				b.WriteString(fmt.Sprintf(m.ForbidRunFmt, fmtClock(m, o.RunStart, now), fmtClock(m, o.FlipAt, now)))
				b.WriteString(fmt.Sprintf(m.ForbidFlipFmt, fmtClock(m, o.FlipAt, now)))
			} else {
				b.WriteString(fmt.Sprintf(m.ForbidRunOpenFmt, fmtClock(m, o.RunStart, now)))
			}
		}
	} else {
		b.WriteString(m.AllowTitle)
		b.WriteString(fmt.Sprintf(m.AllowPriceFmt, fmtPrice(d.TriggerPrice)))
		if o.HasRun && o.FlipKnown {
			b.WriteString(fmt.Sprintf(m.AllowNextFmt, fmtClock(m, o.FlipAt, now)))
		} else {
			b.WriteString(m.AllowNone)
		}
	}
	if telemetryOK {
		b.WriteString(fmt.Sprintf(m.TelemetryFmt, info.Soc, info.FeedinPower))
	}
	b.WriteString(fmt.Sprintf("🕒 %s %s", date, ts))
	return b.String()
}

// fmtClock renders t as HH:MM, prefixing the date when t is on a different day than ref.
func fmtClock(m Messages, t, ref time.Time) string {
	if t.Year() != ref.Year() || t.YearDay() != ref.YearDay() {
		return t.Format(m.ClockDatedLayout)
	}
	return t.Format(m.ClockLayout)
}

func fmtPrice(p float64) string {
	s := fmt.Sprintf("%.2f", p)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
