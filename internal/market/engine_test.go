package market

import (
	"math"
	"os"
	"strings"
	"testing"
)

func testData(t *testing.T) ([]*Listing, TypeChart) {
	t.Helper()
	listings, chart, err := LoadData(os.DirFS("../.."))
	if err != nil {
		t.Fatalf("load data: %v", err)
	}
	return listings, chart
}

func TestLoadData(t *testing.T) {
	listings, chart := testData(t)
	if len(listings) != 1025 {
		t.Fatalf("want 1025 listings, got %d", len(listings))
	}
	if len(chart.Types()) != 18 {
		t.Fatalf("want 18 types, got %d", len(chart.Types()))
	}
	seen := map[string]bool{}
	for _, l := range listings {
		if seen[l.Symbol] {
			t.Errorf("duplicate symbol %s", l.Symbol)
		}
		seen[l.Symbol] = true
		if l.IPOPrice <= 0 || l.Volatility <= 0 || l.Shares <= 0 {
			t.Errorf("%s has bad pricing fields: %+v", l.Symbol, l)
		}
	}
}

func TestEffectiveness(t *testing.T) {
	_, chart := testData(t)
	cases := []struct {
		atk  string
		def  []string
		want float64
	}{
		{"Water", []string{"Fire"}, 2},
		{"Electric", []string{"Ground"}, 0},
		{"Rock", []string{"Fire", "Flying"}, 4}, // Charizard's 4× weakness
		{"Ground", []string{"Fire", "Flying"}, 0},
		{"Fire", []string{"Water", "Rock"}, 0.25},
		{"Normal", []string{"Normal"}, 1},
	}
	for _, c := range cases {
		if got := chart.Effectiveness(c.atk, c.def); got != c.want {
			t.Errorf("%s vs %v = %v, want %v", c.atk, c.def, got, c.want)
		}
	}
}

func TestStepMovesPricesAndKeepsThemSane(t *testing.T) {
	listings, chart := testData(t)
	e := NewEngine(listings, chart, 42)
	for i := 0; i < 500; i++ {
		e.Step()
	}
	moved := 0
	for _, tk := range e.order {
		if tk.Price != tk.IPOPrice {
			moved++
		}
		if tk.Price <= 0 || math.IsNaN(tk.Price) || math.IsInf(tk.Price, 0) {
			t.Fatalf("%s has insane price %v", tk.Symbol, tk.Price)
		}
		ratio := tk.Price / tk.IPOPrice
		if ratio < 0.2 || ratio > 5 {
			t.Errorf("%s drifted too far: %.2f× IPO", tk.Symbol, ratio)
		}
		if tk.Low > tk.Price || tk.High < tk.Price {
			t.Errorf("%s high/low inconsistent", tk.Symbol)
		}
	}
	if moved < len(e.order)*9/10 {
		t.Errorf("only %d/%d tickers moved", moved, len(e.order))
	}
	if len(e.order[0].Candles) == 0 {
		t.Error("expected candles after 500 ticks")
	}
	if len(e.News(10)) < 5 {
		t.Errorf("expected news to be generated, got %d", len(e.News(10)))
	}
}

func TestTypeShockRespectsChart(t *testing.T) {
	listings, chart := testData(t)
	e := NewEngine(listings, chart, 7)
	e.mu.Lock()
	// Keep rolling type shocks until the random attacking type is Water.
	var ev *Event
	for i := 0; i < 1000 && (ev == nil || !strings.Contains(ev.Headline, "Water")); i++ {
		for _, tk := range e.order {
			tk.shock = 0
		}
		ev = e.typeShock()
	}
	chrz, blas := e.tickers["CHRZ"], e.tickers["BLAS"]
	e.mu.Unlock()
	if !strings.Contains(ev.Headline, "Water") {
		t.Skip("could not force a Water shock")
	}
	if chrz.shock >= 0 {
		t.Errorf("Charizard should be hurt by a Water shock, shock=%v", chrz.shock)
	}
	if blas.shock <= 0 {
		t.Errorf("Blastoise should benefit from a Water shock, shock=%v", blas.shock)
	}
}

func TestTrading(t *testing.T) {
	listings, chart := testData(t)
	e := NewEngine(listings, chart, 1)
	id := "trainer-1"

	if _, err := e.Trade(id, "NOPE", "buy", 1); err != ErrUnknownSymbol {
		t.Errorf("want ErrUnknownSymbol, got %v", err)
	}
	if _, err := e.Trade(id, "PIKA", "buy", 0); err != ErrBadQty {
		t.Errorf("want ErrBadQty, got %v", err)
	}
	if _, err := e.Trade(id, "PIKA", "sell", 1); err != ErrNoShares {
		t.Errorf("want ErrNoShares, got %v", err)
	}
	if _, err := e.Trade(id, "ARCE", "buy", 1_000_000); err != ErrInsufficient {
		t.Errorf("want ErrInsufficient, got %v", err)
	}

	tr, err := e.Trade(id, "PIKA", "buy", 10)
	if err != nil {
		t.Fatal(err)
	}
	pf := e.Portfolio(id)
	if len(pf.Holdings) != 1 || pf.Holdings[0].Qty != 10 {
		t.Fatalf("bad holdings: %+v", pf.Holdings)
	}
	if math.Abs(pf.Cash-(startingCash-tr.Total)) > 0.01 {
		t.Errorf("cash %v, want %v", pf.Cash, startingCash-tr.Total)
	}
	// Buy at ask / sell at bid: round trip at the same price should lose the spread.
	if _, err := e.Trade(id, "PIKA", "sell", 10); err != nil {
		t.Fatal(err)
	}
	pf = e.Portfolio(id)
	if len(pf.Holdings) != 0 {
		t.Errorf("expected flat book, got %+v", pf.Holdings)
	}
	if pf.Cash >= startingCash {
		t.Errorf("round trip should cost the spread; cash=%v", pf.Cash)
	}
	if lb := e.Leaderboard(10); len(lb) != 1 || lb[0].Name != pf.Name {
		t.Errorf("bad leaderboard: %+v", lb)
	}
}

func TestBotsTradeAndStaySolvent(t *testing.T) {
	listings, chart := testData(t)
	e := NewEngine(listings, chart, 7)
	e.AddBots()
	for i := 0; i < 300; i++ {
		e.Step()
	}
	lb := e.Leaderboard(100)
	if len(lb) != len(cast) {
		t.Fatalf("want %d bots on the leaderboard, got %d", len(cast), len(lb))
	}
	traded := 0
	for _, p := range e.players {
		if !p.Bot {
			t.Errorf("non-bot player %s present", p.ID)
		}
		if p.Cash < -0.01 {
			t.Errorf("%s went negative: %.2f", p.Name, p.Cash)
		}
		if len(p.Trades) > 0 {
			traded++
		}
		for _, pos := range p.Holdings {
			if pos.Qty <= 0 {
				t.Errorf("%s holds %d of %s", p.Name, pos.Qty, pos.Symbol)
			}
		}
	}
	if traded < len(cast)-2 {
		t.Errorf("only %d of %d bots traded", traded, len(cast))
	}
	roles := map[string]int{}
	for _, row := range lb {
		if !row.Bot || row.Tagline == "" {
			t.Errorf("leaderboard row missing bot metadata: %+v", row)
		}
		roles[row.Role]++
	}
	for _, want := range []string{"Gym Leader", "Elite Four", "Champion"} {
		if roles[want] == 0 {
			t.Errorf("no %s on the leaderboard", want)
		}
	}
}
