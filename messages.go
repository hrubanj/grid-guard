package main

// Messages holds every user-facing, localizable string for the Telegram
// notifications, plus the date/clock layouts that vary by locale. The day-price
// chart image stays English/ASCII for all languages because its bitmap fonts
// lack non-ASCII glyphs (see chart.go) — the rich localized wording lives in the
// Telegram caption instead.
//
// Format strings keep their %-verbs and the leading emoji / HTML <b> tags exactly
// in step with how RenderSwitch consumes them; translating a bundle means swapping
// the words, not the structure.
type Messages struct {
	ForbidTitle      string // header line when stopping export
	ForbidPriceFmt   string // %s trigger price, %s threshold
	ForbidRunFmt     string // %s window start, %s window end
	ForbidFlipFmt    string // %s time export resumes
	ForbidRunOpenFmt string // %s window start (end beyond available data)
	AllowTitle       string // header line when resuming export
	AllowPriceFmt    string // %s trigger price
	AllowNextFmt     string // %s start of the next negative-price window
	AllowNone        string // no further negative window in available data
	TelemetryFmt     string // %.0f battery SoC, %.0f grid feed-in power
	DateLayout       string // footer date stamp
	ClockLayout      string // same-day HH:MM clock
	ClockDatedLayout string // clock prefixed with the date when on another day
}

// enMessages is the default bundle for the public release.
var enMessages = Messages{
	ForbidTitle:      "🛑 <b>Stopping grid export</b>\n",
	ForbidPriceFmt:   "Spot price is dropping to <b>%s EUR/MWh</b> (≤ %s) — we don't sell at a loss.\n",
	ForbidRunFmt:     "⏳ Negative-price window: <b>%s–%s</b>\n",
	ForbidFlipFmt:    "▶️ Export resumes at <b>%s</b>\n",
	ForbidRunOpenFmt: "⏳ Negative-price window: <b>from %s</b> (end beyond available data)\n",
	AllowTitle:       "✅ <b>Resuming grid export</b>\n",
	AllowPriceFmt:    "Spot price is positive again: <b>%s EUR/MWh</b>.\n",
	AllowNextFmt:     "⏭️ Next negative-price window: <b>from %s</b>\n",
	AllowNone:        "⏭️ No further negative-price window in available data.\n",
	TelemetryFmt:     "🔋 Battery <b>%.0f %%</b> · grid feed-in <b>%.0f W</b>\n",
	DateLayout:       "Jan 2, 2006",
	ClockLayout:      "15:04",
	ClockDatedLayout: "Jan 2, 15:04",
}

// csMessages is the original Czech copy, preserved verbatim so the existing
// deployment is unchanged when it sets "language": "cs".
var csMessages = Messages{
	ForbidTitle:      "🛑 <b>Zastavuji prodej do sítě</b>\n",
	ForbidPriceFmt:   "Spotová cena klesá na <b>%s €/MWh</b> (≤ %s) — neprodáváme se ztrátou.\n",
	ForbidRunFmt:     "⏳ Záporné období: <b>%s–%s</b>\n",
	ForbidFlipFmt:    "▶️ Prodej obnovíme v <b>%s</b>\n",
	ForbidRunOpenFmt: "⏳ Záporné období: <b>od %s</b> (konec zatím mimo dostupná data)\n",
	AllowTitle:       "✅ <b>Obnovuji prodej do sítě</b>\n",
	AllowPriceFmt:    "Spotová cena je opět kladná: <b>%s €/MWh</b>.\n",
	AllowNextFmt:     "⏭️ Další záporné období: <b>od %s</b>\n",
	AllowNone:        "⏭️ V dostupných datech žádné další záporné období.\n",
	TelemetryFmt:     "🔋 Baterie <b>%.0f %%</b> · tok do sítě <b>%.0f W</b>\n",
	DateLayout:       "2.1.2006",
	ClockLayout:      "15:04",
	ClockDatedLayout: "2.1. 15:04",
}

// bundles maps a config language code to its message bundle.
var bundles = map[string]Messages{
	"en": enMessages,
	"cs": csMessages,
}

// supportedLanguages lists the language codes bundles knows about (for config validation).
func supportedLanguages() []string { return []string{"en", "cs"} }

// messagesFor returns the bundle for lang, falling back to English.
func messagesFor(lang string) Messages {
	if m, ok := bundles[lang]; ok {
		return m
	}
	return enMessages
}
