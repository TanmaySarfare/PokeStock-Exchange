package market

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	historyLen   = 240 // tick prices kept per ticker (sparklines)
	candleLen    = 180 // candles kept per ticker
	newsLen      = 120
	startingCash = 10_000.0
)

// Candle is an OHLC bar aggregated over several ticks.
type Candle struct {
	T int64   `json:"t"` // unix seconds at open
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V int64   `json:"v"` // synthetic volume
}

// Ticker is a Listing plus live market state.
type Ticker struct {
	*Listing
	Price   float64
	Open    float64 // session open (= IPO on boot)
	High    float64
	Low     float64
	Volume  int64
	History []float64
	Candles []Candle
	cur     Candle
	curN    int
	shock   float64 // decaying momentum injected by news events
}

// Pct returns percent change from the session open.
func (t *Ticker) Pct() float64 {
	if t.Open == 0 {
		return 0
	}
	return (t.Price/t.Open - 1) * 100
}

// MarketCap is price × float.
func (t *Ticker) MarketCap() float64 { return t.Price * float64(t.Shares) }

// Update is what gets broadcast to SSE subscribers every tick.
type Update struct {
	Tick      int64              `json:"tick"`
	Time      int64              `json:"time"`
	Composite float64            `json:"composite"`
	Prices    map[string]float64 `json:"prices"`
	News      *Event             `json:"news,omitempty"`
}

// Engine runs the simulated exchange.
type Engine struct {
	mu       sync.RWMutex
	tickers  map[string]*Ticker
	order    []*Ticker // by dex
	families map[string][]*Ticker
	chart    TypeChart
	rng      *rand.Rand
	tick     int64
	now      int64 // unix seconds of the current/last tick
	nextNews int64
	baseCap  float64
	news     []Event
	newsSeq  int
	players  map[string]*Player
	subs     map[chan Update]struct{}

	TickInterval time.Duration
	CandleTicks  int
}

// NewEngine builds an engine from listings. seed=0 means time-based.
func NewEngine(listings []*Listing, chart TypeChart, seed int64) *Engine {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	e := &Engine{
		tickers:      make(map[string]*Ticker, len(listings)),
		families:     make(map[string][]*Ticker),
		chart:        chart,
		rng:          rand.New(rand.NewSource(seed)),
		players:      make(map[string]*Player),
		subs:         make(map[chan Update]struct{}),
		TickInterval: 2 * time.Second,
		CandleTicks:  10,
	}
	e.now = time.Now().Unix()
	for _, l := range listings {
		t := &Ticker{Listing: l, Price: l.IPOPrice, Open: l.IPOPrice, High: l.IPOPrice, Low: l.IPOPrice}
		t.History = append(t.History, l.IPOPrice)
		t.cur = Candle{T: e.now, O: l.IPOPrice, H: l.IPOPrice, L: l.IPOPrice, C: l.IPOPrice}
		e.tickers[l.Symbol] = t
		e.order = append(e.order, t)
		e.families[l.Family] = append(e.families[l.Family], t)
		e.baseCap += l.IPOPrice * float64(l.Shares)
	}
	sort.Slice(e.order, func(i, j int) bool { return e.order[i].Dex < e.order[j].Dex })
	e.scheduleNews()
	e.pushNews(Event{Kind: "open", Headline: "Opening bell: the PokéStock Exchange is live",
		Body: "1,025 securities across 18 type sectors begin trading at their IPO price. Trainers start with $10,000 in cash."})
	return e
}

// Run ticks until ctx is done. Blocks.
func (e *Engine) Run(stop <-chan struct{}) {
	tk := time.NewTicker(e.TickInterval)
	defer tk.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tk.C:
			e.Step()
		}
	}
}

// Step advances the market one tick.
func (e *Engine) Step() { e.stepAt(time.Now().Unix()) }

// Warm simulates n ticks of history as if they happened before now, so the
// exchange opens with charts and a news feed already populated.
func (e *Engine) Warm(n int) {
	now := time.Now()
	start := now.Add(-time.Duration(n+1) * e.TickInterval).Unix()
	e.mu.Lock()
	e.now = start
	for _, t := range e.order {
		t.cur.T = start
	}
	for i := range e.news {
		e.news[i].Time = start
	}
	e.mu.Unlock()
	for i := n; i > 0; i-- {
		e.stepAt(now.Add(-time.Duration(i) * e.TickInterval).Unix())
	}
}

func (e *Engine) stepAt(now int64) {
	e.mu.Lock()
	e.tick++
	e.now = now

	// Common factors so tickers are correlated: one market-wide, one per type.
	marketF := e.rng.NormFloat64() * 0.0012
	sectorF := make(map[string]float64, 18)
	for _, t := range e.chart.Types() {
		sectorF[t] = e.rng.NormFloat64() * 0.0025
	}

	var ev *Event
	if e.tick >= e.nextNews {
		ev = e.generateEvent()
		e.scheduleNews()
	}

	prices := make(map[string]float64, len(e.order))
	for _, t := range e.order {
		sec := 0.0
		for _, ty := range t.Types() {
			sec += sectorF[ty]
		}
		sec /= float64(len(t.Types()))

		r := t.Drift + marketF + sec + t.Volatility*e.rng.NormFloat64() + t.shock
		// Mean reversion keeps things from running off to zero or infinity.
		r -= 0.002 * math.Log(t.Price/t.IPOPrice)
		t.shock *= 0.80

		t.Price = round2(t.Price * math.Exp(r))
		if t.Price < 0.05 {
			t.Price = 0.05
		}
		if t.Price > t.High {
			t.High = t.Price
		}
		if t.Price < t.Low {
			t.Low = t.Price
		}
		vol := int64(math.Abs(r)*float64(t.Shares)*0.5) + int64(e.rng.Intn(200))
		t.Volume += vol

		t.History = append(t.History, t.Price)
		if len(t.History) > historyLen {
			t.History = t.History[1:]
		}

		// candles
		if t.Price > t.cur.H {
			t.cur.H = t.Price
		}
		if t.Price < t.cur.L {
			t.cur.L = t.Price
		}
		t.cur.C = t.Price
		t.cur.V += vol
		t.curN++
		if t.curN >= e.CandleTicks {
			t.Candles = append(t.Candles, t.cur)
			if len(t.Candles) > candleLen {
				t.Candles = t.Candles[1:]
			}
			t.cur = Candle{T: now, O: t.Price, H: t.Price, L: t.Price, C: t.Price}
			t.curN = 0
		}
		prices[t.Symbol] = t.Price
	}

	// Track every trainer's net worth over time for their portfolio chart.
	for _, p := range e.players {
		_, net := e.netWorthLocked(p)
		p.NetWorthH = append(p.NetWorthH, net)
		if len(p.NetWorthH) > historyLen {
			p.NetWorthH = p.NetWorthH[1:]
		}
	}

	upd := Update{Tick: e.tick, Time: now, Composite: e.compositeLocked(), Prices: prices, News: ev}
	subs := make([]chan Update, 0, len(e.subs))
	for ch := range e.subs {
		subs = append(subs, ch)
	}
	e.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- upd:
		default: // slow client; drop the frame
		}
	}
}

func (e *Engine) scheduleNews() {
	e.nextNews = e.tick + 6 + int64(e.rng.Intn(10))
}

// compositeLocked is the PKDX composite: total market cap vs IPO cap × 1000.
func (e *Engine) compositeLocked() float64 {
	var cap float64
	for _, t := range e.order {
		cap += t.Price * float64(t.Shares)
	}
	if e.baseCap == 0 {
		return 1000
	}
	return round2(cap / e.baseCap * 1000)
}

// Subscribe returns a channel of tick updates. Call the returned func to stop.
func (e *Engine) Subscribe() (<-chan Update, func()) {
	ch := make(chan Update, 4)
	e.mu.Lock()
	e.subs[ch] = struct{}{}
	e.mu.Unlock()
	return ch, func() {
		e.mu.Lock()
		delete(e.subs, ch)
		e.mu.Unlock()
	}
}

// --- read API ---------------------------------------------------------------

// Snapshot is a lightweight view of a ticker for lists.
type Snapshot struct {
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
	Dex       int       `json:"dex"`
	Sector    string    `json:"sector"`
	Subsector *string   `json:"subsector"`
	Rarity    string    `json:"rarity"`
	Group     string    `json:"group"`
	Stage     int       `json:"stage"`
	Image     string    `json:"image"`
	Price     float64   `json:"price"`
	Open      float64   `json:"open"`
	Pct       float64   `json:"pct"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Volume    int64     `json:"volume"`
	MarketCap float64   `json:"mcap"`
	Vol       float64   `json:"volatility"`
	BST       int       `json:"bst"`
	Spark     []float64 `json:"spark"`
}

func (t *Ticker) snapshot(sparkN int) Snapshot {
	h := t.History
	if len(h) > sparkN {
		h = h[len(h)-sparkN:]
	}
	spark := make([]float64, len(h))
	copy(spark, h)
	return Snapshot{
		Symbol: t.Symbol, Name: t.Name, Dex: t.Dex, Sector: t.Sector, Subsector: t.Subsector,
		Rarity: t.Rarity, Group: t.Group, Stage: t.Stage, Image: t.Image, Price: t.Price, Open: t.Open,
		Pct: round2(t.Pct()), High: t.High, Low: t.Low, Volume: t.Volume, MarketCap: t.MarketCap(),
		Vol: t.Volatility, BST: t.Stats.BST, Spark: spark,
	}
}

// SectorStat aggregates a type sector.
type SectorStat struct {
	Type   string  `json:"type"`
	Count  int     `json:"count"`
	AvgPct float64 `json:"avg_pct"`
	Cap    float64 `json:"mcap"`
	Leader string  `json:"leader"`
}

// MarketView is the overview payload.
type MarketView struct {
	Tick      int64        `json:"tick"`
	Time      int64        `json:"time"`
	Composite float64      `json:"composite"`
	Tickers   []Snapshot   `json:"tickers"`
	Sectors   []SectorStat `json:"sectors"`
}

// Market returns the overview of every ticker.
func (e *Engine) Market() MarketView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := MarketView{Tick: e.tick, Time: time.Now().Unix(), Composite: e.compositeLocked()}
	out.Tickers = make([]Snapshot, 0, len(e.order))
	agg := map[string]*SectorStat{}
	leaderPct := map[string]float64{}
	for _, t := range e.order {
		out.Tickers = append(out.Tickers, t.snapshot(40))
		for i, ty := range t.Types() {
			s := agg[ty]
			if s == nil {
				s = &SectorStat{Type: ty}
				agg[ty] = s
				leaderPct[ty] = math.Inf(-1)
			}
			if i == 0 {
				s.Count++
				s.Cap += t.MarketCap()
			}
			s.AvgPct += t.Pct()
			if t.Pct() > leaderPct[ty] {
				leaderPct[ty] = t.Pct()
				s.Leader = t.Symbol
			}
		}
	}
	for _, ty := range e.chart.Types() {
		if s, ok := agg[ty]; ok {
			n := 0
			for _, t := range e.order {
				for _, tt := range t.Types() {
					if tt == ty {
						n++
					}
				}
			}
			if n > 0 {
				s.AvgPct = round2(s.AvgPct / float64(n))
			}
			out.Sectors = append(out.Sectors, *s)
		}
	}
	return out
}

// Detail is the full payload for a ticker page.
type Detail struct {
	Snapshot
	Listing *Listing   `json:"listing"`
	Candles []Candle   `json:"candles"`
	Current Candle     `json:"current"`
	Family  []Snapshot `json:"family"`
	News    []Event    `json:"news"`
	Spread  float64    `json:"spread"`
}

// Ticker returns the detail view, or nil if unknown.
func (e *Engine) Ticker(sym string) *Detail {
	e.mu.RLock()
	defer e.mu.RUnlock()
	t, ok := e.tickers[sym]
	if !ok {
		return nil
	}
	d := &Detail{Snapshot: t.snapshot(historyLen), Listing: t.Listing, Current: t.cur, Spread: spreadFor(t)}
	d.Candles = append(d.Candles, t.Candles...)
	for _, f := range e.families[t.Family] {
		d.Family = append(d.Family, f.snapshot(40))
	}
	for i := len(e.news) - 1; i >= 0 && len(d.News) < 10; i-- {
		for _, s := range e.news[i].Symbols {
			if s == sym {
				d.News = append(d.News, e.news[i])
				break
			}
		}
	}
	return d
}

// News returns the most recent events, newest first.
func (e *Engine) News(limit int) []Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Event, 0, limit)
	for i := len(e.news) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, e.news[i])
	}
	return out
}

func (e *Engine) pushNews(ev Event) *Event {
	e.newsSeq++
	ev.ID = e.newsSeq
	ev.Time = e.now
	ev.Tick = e.tick
	e.news = append(e.news, ev)
	if len(e.news) > newsLen {
		e.news = e.news[1:]
	}
	return &e.news[len(e.news)-1]
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// spreadFor: legendary paper is thinly traded, so it has a wide spread.
func spreadFor(t *Ticker) float64 {
	switch t.Rarity {
	case "Mythical":
		return 0.006
	case "Legendary":
		return 0.004
	}
	return 0.001
}
