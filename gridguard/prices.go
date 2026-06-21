package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

// QuarterMinutes is the OTE pricing period length.
const QuarterMinutes = 15

// Quarter is one 15-minute OTE price period.
type Quarter struct {
	Index int       // 1..96 within its day
	Start time.Time // local start instant
	Price float64   // EUR/MWh
}

const oteSeriesTitle = "15min cena (EUR/MWh)"

type oteResponse struct {
	Data struct {
		DataLine []struct {
			Title string `json:"title"`
			Point []struct {
				X string  `json:"x"`
				Y float64 `json:"y"`
			} `json:"point"`
		} `json:"dataLine"`
	} `json:"data"`
}

// parseQuarters turns an OTE chart-data body into Quarters for the given day.
func parseQuarters(body []byte, day time.Time, loc *time.Location) ([]Quarter, error) {
	var resp oteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse OTE json: %w", err)
	}
	midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	for _, line := range resp.Data.DataLine {
		if line.Title != oteSeriesTitle {
			continue
		}
		out := make([]Quarter, 0, len(line.Point))
		for _, p := range line.Point {
			idx, err := strconv.Atoi(p.X)
			if err != nil {
				return nil, fmt.Errorf("bad quarter index %q: %w", p.X, err)
			}
			// Start is local-midnight + (idx-1) real quarter-hours. This is DST-correct:
			// OTE indexes slots by real elapsed quarter from local midnight (92 slots on a
			// spring-forward day, 100 on a fall-back day), which absolute-duration addition
			// tracks exactly. Do NOT switch to time.Date(...,minute:(idx-1)*15,...) — that
			// would mishandle the fall-back repeated 02:00 hour.
			out = append(out, Quarter{
				Index: idx,
				Start: midnight.Add(time.Duration(idx-1) * QuarterMinutes * time.Minute),
				Price: p.Y,
			})
		}
		return out, nil
	}
	return nil, nil // series not present (e.g. day not published yet)
}

// fetchDay downloads and parses one day's quarters; nil slice if unpublished.
func fetchDay(day time.Time, loc *time.Location, httpc *http.Client) ([]Quarter, error) {
	rawURL := "https://www.ote-cr.cz/cs/kratkodobe-trhy/elektrina/denni-trh/@@chart-data?report_date=" +
		day.Format("2006-01-02")
	resp, err := httpc.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("GET OTE %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OTE %s returned %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read OTE body: %w", err)
	}
	return parseQuarters(body, day, loc)
}

// FetchUpcoming returns today's quarters plus tomorrow's if published.
func FetchUpcoming(now time.Time, loc *time.Location, httpc *http.Client) ([]Quarter, error) {
	today, err := fetchDay(now, loc, httpc)
	if err != nil {
		return nil, err
	}
	tomorrow, err := fetchDay(now.AddDate(0, 0, 1), loc, httpc)
	if err != nil {
		logf("warning: tomorrow price fetch failed (best-effort, continuing with today only): %v", err)
		tomorrow = nil
	}
	return append(today, tomorrow...), nil
}

// Decision is the desired guard outcome for an instant.
type Decision struct {
	Forbid       bool    // selling should be forbidden (price <= threshold within window)
	Found        bool    // price data covered the window
	TriggerPrice float64 // when Forbid: first <=threshold quarter's price; else the window minimum
}

func quarterEnd(q Quarter) time.Time {
	return q.Start.Add(QuarterMinutes * time.Minute)
}

// Guard computes the desired forbid/allow decision for [now, now+lookahead].
func Guard(qs []Quarter, now time.Time, lookahead time.Duration, threshold float64) Decision {
	end := now.Add(lookahead)
	d := Decision{}
	windowMin := math.Inf(1)
	haveTrigger := false
	for _, q := range qs {
		qEnd := quarterEnd(q)
		// quarter [q.Start, qEnd) overlaps window [now, end]?
		if !qEnd.After(now) || q.Start.After(end) {
			continue
		}
		d.Found = true
		if q.Price < windowMin {
			windowMin = q.Price
		}
		if q.Price <= threshold && !haveTrigger {
			d.TriggerPrice = q.Price // first quarter at/below threshold
			haveTrigger = true
		}
	}
	if !d.Found {
		return d
	}
	d.Forbid = haveTrigger
	if !haveTrigger {
		// allow: report the price at `now` (window minimum if now is outside the data)
		if p, ok := priceAt(qs, now); ok {
			d.TriggerPrice = p
		} else {
			d.TriggerPrice = windowMin
		}
	}
	return d
}

// Outlook describes the contiguous price run on one side of the threshold and
// when it next flips to the other side.
type Outlook struct {
	HasRun    bool      // a run on the requested side exists at/after `from`
	RunStart  time.Time // start of that contiguous run
	FlipAt    time.Time // instant the price crosses to the other side
	FlipKnown bool      // false if the run reaches the end of available data without flipping
}

// PriceOutlook computes the contiguous run on one side of the threshold starting at
// or after `from` (below==true selects the price<=threshold side), and the next
// crossing. FlipKnown is false when the run extends past the available data, so
// callers must not present FlipAt as a real crossing in that case.
func PriceOutlook(qs []Quarter, from time.Time, threshold float64, below bool) Outlook {
	onSide := func(p float64) bool { return (p <= threshold) == below }
	t := -1
	for i, q := range qs {
		if quarterEnd(q).After(from) && onSide(q.Price) {
			t = i
			break
		}
	}
	if t == -1 {
		return Outlook{}
	}
	s := t
	for s > 0 && onSide(qs[s-1].Price) && quarterEnd(qs[s-1]).Equal(qs[s].Start) {
		s--
	}
	e := t
	for e+1 < len(qs) && onSide(qs[e+1].Price) && qs[e+1].Start.Equal(quarterEnd(qs[e])) {
		e++
	}
	flipKnown := e+1 < len(qs) && qs[e+1].Start.Equal(quarterEnd(qs[e]))
	return Outlook{HasRun: true, RunStart: qs[s].Start, FlipAt: quarterEnd(qs[e]), FlipKnown: flipKnown}
}

// priceAt returns the price of the quarter containing t.
func priceAt(qs []Quarter, t time.Time) (float64, bool) {
	for _, q := range qs {
		if !t.Before(q.Start) && t.Before(quarterEnd(q)) {
			return q.Price, true
		}
	}
	return 0, false
}
