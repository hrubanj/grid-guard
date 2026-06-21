package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "time/tzdata" // embed tzdata so Europe/Prague works on minimal hosts
)

func main() {
	cfgPath := flag.String("config", defaultConfigPath(), "path to config.json")
	dryRun := flag.Bool("dry-run", false, "compute and log, but do not write the inverter or send Telegram")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()

	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "run"
	}

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		fatal(err)
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		fatal(fmt.Errorf("load timezone %q: %w", cfg.Timezone, err))
	}
	app := &App{cfg: cfg, loc: loc, http: &http.Client{Timeout: 30 * time.Second}, dryRun: *dryRun, verbose: *verbose}

	switch cmd {
	case "run":
		err = app.Run()
	case "prices":
		err = app.CmdPrices()
	case "state":
		err = app.CmdState()
	case "forbid":
		err = app.CmdForce(true)
	case "allow":
		err = app.CmdForce(false)
	case "notify-test":
		err = app.CmdNotifyTest()
	default:
		err = fmt.Errorf("unknown command %q (run|prices|state|forbid|allow|notify-test)", cmd)
	}
	if err != nil {
		fatal(err)
	}
}

// App holds shared run context.
type App struct {
	cfg     *Config
	loc     *time.Location
	http    *http.Client
	dryRun  bool
	verbose bool
}

func defaultConfigPath() string {
	if p := os.Getenv("GRID_GUARD_CONFIG"); p != "" {
		return p
	}
	return "/etc/grid-guard/config.json"
}

func logf(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
func fatal(err error)              { logf("grid-guard: %v", err); os.Exit(1) }

// CmdPrices fetches prices and prints the current desired decision (no inverter, no Telegram).
func (a *App) CmdPrices() error {
	now := time.Now().In(a.loc)
	qs, err := FetchUpcoming(now, a.loc, a.http)
	if err != nil {
		return err
	}
	d := Guard(qs, now, time.Duration(a.cfg.LookaheadMinutes)*time.Minute, a.cfg.ThresholdEurMWh)
	cur, found := priceAt(qs, now)
	priceStr := "(no data)"
	if found {
		priceStr = fmt.Sprintf("%.2f", cur)
	}
	logf("now=%s quarters=%d current_price=%s found=%v desired_forbid=%v trigger=%.2f",
		now.Format("2006-01-02 15:04"), len(qs), priceStr, d.Found, d.Forbid, d.TriggerPrice)
	return nil
}

// CmdState logs in and prints live inverter state (read-only).
func (a *App) CmdState() error {
	sc, err := a.solaxSession()
	if err != nil {
		return err
	}
	w, err := sc.ExportControlW()
	if err != nil {
		return err
	}
	info, err := sc.Realtime()
	if err != nil {
		return err
	}
	logf("EXPORT_CONTROL=%d W (selling %s) | soc=%.0f%% feedin=%.0fW ac=%.0fW bat=%.0fW",
		w, sellingLabel(w > 0), info.Soc, info.FeedinPower, info.AcPower, info.BatPower)
	return nil
}

func sellingLabel(on bool) string {
	if on {
		return "ALLOWED"
	}
	return "FORBIDDEN"
}

// Run is the cron entry point: enforce + notify only on a real switch.
func (a *App) Run() error {
	now := time.Now().In(a.loc)
	qs, err := FetchUpcoming(now, a.loc, a.http)
	if err != nil {
		return fmt.Errorf("fetch prices: %w", err)
	}
	if len(qs) == 0 {
		logf("WARNING: OTE returned no usable price quarters — the \"15min cena (EUR/MWh)\" series may have been renamed/reformatted; inverter left unchanged")
		return nil
	}
	d := Guard(qs, now, time.Duration(a.cfg.LookaheadMinutes)*time.Minute, a.cfg.ThresholdEurMWh)
	if !d.Found {
		logf("no price data covering [%s,+%dm]; leaving inverter unchanged",
			now.Format("15:04"), a.cfg.LookaheadMinutes)
		return nil
	}

	sc, err := a.solaxSession()
	if err != nil {
		return err
	}
	curW, err := sc.ExportControlW()
	if err != nil {
		return fmt.Errorf("read export control: %w", err)
	}
	actualForbid := curW <= 0

	if d.Forbid == actualForbid {
		if a.verbose {
			logf("no switch: desired_forbid=%v actual_forbid=%v (export=%dW)", d.Forbid, actualForbid, curW)
		}
		return nil
	}

	logf("SWITCH: forbid %v -> %v (price=%.2f, export was %dW)", actualForbid, d.Forbid, d.TriggerPrice, curW)
	if a.dryRun {
		logf("[dry-run] would set export and send Telegram")
		return nil
	}
	if err := a.applySelling(sc, !d.Forbid); err != nil {
		return err
	}
	info, err := sc.Realtime()
	telemetryOK := err == nil
	if err != nil {
		logf("warning: realtime read failed, sending message without telemetry: %v", err)
	}
	return a.notify(d, info, telemetryOK, now, qs)
}

// notify sends the switch message with the day-price chart, falling back to text.
func (a *App) notify(d Decision, info RealtimeInfo, telemetryOK bool, now time.Time, qs []Quarter) error {
	o := PriceOutlook(qs, now, a.cfg.ThresholdEurMWh, d.Forbid)
	caption := RenderSwitch(messagesFor(a.cfg.Language), d, info, now, a.cfg.ThresholdEurMWh, o, telemetryOK)
	png, err := RenderPriceChartPNG(qs, now, a.cfg.ThresholdEurMWh, a.loc)
	if err != nil {
		logf("warning: chart render failed, sending text only: %v", err)
		return SendTelegram(a.cfg.Telegram, caption)
	}
	if err := SendTelegramPhoto(a.cfg.Telegram, png, caption); err != nil {
		logf("warning: sendPhoto failed, falling back to text: %v", err)
		return SendTelegram(a.cfg.Telegram, caption)
	}
	return nil
}

// solaxSession returns a logged-in Solax client.
func (a *App) solaxSession() (*SolaxClient, error) {
	sc := NewSolaxClient(a.cfg.Solax)
	if err := sc.Login(); err != nil {
		return nil, fmt.Errorf("solax login: %w", err)
	}
	return sc, nil
}

// applySelling sets the inverter to allow or forbid grid export.
func (a *App) applySelling(sc *SolaxClient, allow bool) error {
	target := 0
	if allow {
		target = a.cfg.AllowExportValue
	}
	if err := sc.SetExportControlW(target); err != nil {
		return fmt.Errorf("set export control: %w", err)
	}
	if a.cfg.AlsoSetWorkmode {
		mode := workModeSelfUse
		if allow {
			mode = workModeFeedIn
		}
		if err := sc.SetWorkMode(mode); err != nil {
			return fmt.Errorf("set work mode: %w", err)
		}
	}
	return nil
}

// CmdForce manually forbids/allows selling (honours -dry-run); no Telegram.
func (a *App) CmdForce(forbid bool) error {
	sc, err := a.solaxSession()
	if err != nil {
		return err
	}
	before, err := sc.ExportControlW()
	if err != nil {
		return fmt.Errorf("read export control: %w", err)
	}
	if a.dryRun {
		logf("[dry-run] export=%dW; would set selling allowed=%v", before, !forbid)
		return nil
	}
	if err := a.applySelling(sc, !forbid); err != nil {
		return err
	}
	after, err := sc.ExportControlW()
	if err != nil {
		return err
	}
	logf("export %dW -> %dW (selling %s)", before, after, sellingLabel(after > 0))
	return nil
}

// CmdNotifyTest renders and sends both message variants with a real chart from
// today's prices (honours -dry-run).
func (a *App) CmdNotifyTest() error {
	now := time.Now().In(a.loc)
	qs, err := FetchUpcoming(now, a.loc, a.http)
	if err != nil {
		logf("warning: could not fetch prices for sample chart: %v", err)
	}
	hour := now.Truncate(time.Hour)
	samples := []struct {
		d Decision
		o Outlook
	}{
		{Decision{Forbid: true, TriggerPrice: -50.8},
			Outlook{HasRun: true, RunStart: hour, FlipAt: hour.Add(90 * time.Minute), FlipKnown: true}},
		{Decision{Forbid: false, TriggerPrice: 42.5},
			Outlook{HasRun: true, FlipAt: hour.Add(3 * time.Hour), FlipKnown: true}},
	}
	info := RealtimeInfo{Soc: 98, FeedinPower: 64}
	for _, s := range samples {
		if a.dryRun {
			logf("[dry-run] message:\n%s\n", RenderSwitch(messagesFor(a.cfg.Language), s.d, info, now, a.cfg.ThresholdEurMWh, s.o, true))
			continue
		}
		if err := a.notify(s.d, info, true, now, qs); err != nil {
			return err
		}
	}
	return nil
}
