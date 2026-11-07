package market

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

// Position is a holding in one ticker.
type Position struct {
	Symbol  string  `json:"symbol"`
	Qty     int64   `json:"qty"`
	AvgCost float64 `json:"avg_cost"`
}

// Trade is an executed order.
type Trade struct {
	Time   int64   `json:"time"`
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Qty    int64   `json:"qty"`
	Price  float64 `json:"price"`
	Total  float64 `json:"total"`
}

// Player is a trainer with a brokerage account.
type Player struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Cash      float64              `json:"cash"`
	Holdings  map[string]*Position `json:"-"`
	Trades    []Trade              `json:"-"`
	Created   int64                `json:"created"`
	Realized  float64              `json:"realized"`
	NetWorthH []float64            `json:"-"`
	Bot       bool                 `json:"bot"`
	Tagline   string               `json:"tagline,omitempty"`
	Role      string               `json:"role,omitempty"`
}

// HoldingView is a position with live valuation.
type HoldingView struct {
	Position
	Name   string  `json:"name"`
	Image  string  `json:"image"`
	Sector string  `json:"sector"`
	Price  float64 `json:"price"`
	Value  float64 `json:"value"`
	PnL    float64 `json:"pnl"`
	PnLPct float64 `json:"pnl_pct"`
}

// PortfolioView is the full account statement.
type PortfolioView struct {
	Player
	Equity    float64       `json:"equity"`
	NetWorth  float64       `json:"net_worth"`
	PnL       float64       `json:"pnl"`
	PnLPct    float64       `json:"pnl_pct"`
	Holdings  []HoldingView `json:"holdings"`
	Trades    []Trade       `json:"trades"`
	Rank      int           `json:"rank"`
	Players   int           `json:"players"`
	NetWorthH []float64     `json:"net_worth_history"`
}

// LeaderRow is one leaderboard entry.
type LeaderRow struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	NetWorth float64 `json:"net_worth"`
	PnLPct   float64 `json:"pnl_pct"`
	Holdings int     `json:"holdings"`
	Top      string  `json:"top"`
	Bot      bool    `json:"bot"`
	Tagline  string  `json:"tagline,omitempty"`
	Role     string  `json:"role,omitempty"`
}

var (
	ErrUnknownSymbol = errors.New("unknown symbol")
	ErrBadQty        = errors.New("quantity must be a positive integer")
	ErrInsufficient  = errors.New("insufficient cash")
	ErrNoShares      = errors.New("not enough shares to sell")
	ErrBadSide       = errors.New("side must be buy or sell")
)

var trainerNames = []string{"Red", "Blue", "Leaf", "Ethan", "Lyra", "Brendan", "May", "Lucas", "Dawn", "Hilbert",
	"Hilda", "Calem", "Serena", "Elio", "Selene", "Victor", "Gloria", "Florian", "Juliana", "Wally", "Silver", "Hugh", "Barry"}

// Player returns (creating if needed) the account with the given id.
func (e *Engine) Player(id string) *Player {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.playerLocked(id)
}

func (e *Engine) playerLocked(id string) *Player {
	p, ok := e.players[id]
	if !ok {
		p = &Player{
			ID:       id,
			Name:     fmt.Sprintf("Trainer %s", trainerNames[e.rng.Intn(len(trainerNames))]),
			Cash:     startingCash,
			Holdings: map[string]*Position{},
			Created:  time.Now().Unix(),
		}
		e.players[id] = p
	}
	return p
}

// Rename sets a display name.
func (e *Engine) Rename(id, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	p := e.playerLocked(id)
	if n := []rune(name); len(n) > 24 {
		name = string(n[:24])
	}
	if name != "" {
		p.Name = name
	}
}

// SetTagline sets the short motto shown next to the name on the leaderboard.
// An empty string clears it.
func (e *Engine) SetTagline(id, tag string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	p := e.playerLocked(id)
	if n := []rune(tag); len(n) > 40 {
		tag = string(n[:40])
	}
	p.Tagline = tag
}

// Trade executes a market order. Buys fill at ask, sells at bid.
func (e *Engine) Trade(id, symbol, side string, qty int64) (*Trade, error) {
	if qty <= 0 {
		return nil, ErrBadQty
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.tickers[symbol]
	if !ok {
		return nil, ErrUnknownSymbol
	}
	return e.tradeLocked(e.playerLocked(id), t, side, qty, time.Now().Unix())
}

// tradeLocked fills a market order for p in t. Caller must hold e.mu.
func (e *Engine) tradeLocked(p *Player, t *Ticker, side string, qty int64, now int64) (*Trade, error) {
	if qty <= 0 {
		return nil, ErrBadQty
	}
	half := spreadFor(t) / 2
	symbol := t.Symbol
	switch side {
	case "buy":
		px := round2(t.Price * (1 + half))
		total := round2(px * float64(qty))
		if total > p.Cash+1e-9 {
			return nil, ErrInsufficient
		}
		p.Cash = round2(p.Cash - total)
		pos := p.Holdings[symbol]
		if pos == nil {
			pos = &Position{Symbol: symbol}
			p.Holdings[symbol] = pos
		}
		pos.AvgCost = (pos.AvgCost*float64(pos.Qty) + total) / float64(pos.Qty+qty)
		pos.Qty += qty
		t.Volume += qty
		tr := Trade{Time: now, Symbol: symbol, Side: side, Qty: qty, Price: px, Total: total}
		p.Trades = appendTrade(p.Trades, tr)
		return &tr, nil
	case "sell":
		pos := p.Holdings[symbol]
		if pos == nil || pos.Qty < qty {
			return nil, ErrNoShares
		}
		px := round2(t.Price * (1 - half))
		total := round2(px * float64(qty))
		p.Cash = round2(p.Cash + total)
		p.Realized += round2((px - pos.AvgCost) * float64(qty))
		pos.Qty -= qty
		if pos.Qty == 0 {
			delete(p.Holdings, symbol)
		}
		t.Volume += qty
		tr := Trade{Time: now, Symbol: symbol, Side: side, Qty: qty, Price: px, Total: total}
		p.Trades = appendTrade(p.Trades, tr)
		return &tr, nil
	}
	return nil, ErrBadSide
}

// appendTrade keeps the blotter bounded so long-running bots don't grow forever.
func appendTrade(ts []Trade, tr Trade) []Trade {
	ts = append(ts, tr)
	if len(ts) > 400 {
		ts = ts[len(ts)-300:]
	}
	return ts
}

func (e *Engine) netWorthLocked(p *Player) (equity, net float64) {
	for sym, pos := range p.Holdings {
		if t, ok := e.tickers[sym]; ok {
			equity += t.Price * float64(pos.Qty)
		}
	}
	return round2(equity), round2(p.Cash + equity)
}

// Portfolio returns the account statement for a player.
func (e *Engine) Portfolio(id string) PortfolioView {
	e.mu.Lock()
	defer e.mu.Unlock()
	p := e.playerLocked(id)
	equity, net := e.netWorthLocked(p)
	v := PortfolioView{Player: *p, Equity: equity, NetWorth: net, PnL: round2(net - startingCash),
		PnLPct: round2((net/startingCash - 1) * 100), Players: len(e.players)}
	for _, pos := range p.Holdings {
		t := e.tickers[pos.Symbol]
		if t == nil {
			continue
		}
		val := round2(t.Price * float64(pos.Qty))
		cost := pos.AvgCost * float64(pos.Qty)
		h := HoldingView{Position: *pos, Name: t.Name, Image: t.Image, Sector: t.Sector, Price: t.Price, Value: val,
			PnL: round2(val - cost)}
		if cost > 0 {
			h.PnLPct = round2((val/cost - 1) * 100)
		}
		v.Holdings = append(v.Holdings, h)
	}
	sort.Slice(v.Holdings, func(i, j int) bool { return v.Holdings[i].Value > v.Holdings[j].Value })
	n := len(p.Trades)
	start := n - 50
	if start < 0 {
		start = 0
	}
	v.Trades = make([]Trade, 0, n-start)
	for i := n - 1; i >= start; i-- {
		v.Trades = append(v.Trades, p.Trades[i])
	}
	v.NetWorthH = append(append([]float64{}, p.NetWorthH...), net)
	v.Rank = 1
	for _, o := range e.players {
		if o.ID == p.ID {
			continue
		}
		if _, on := e.netWorthLocked(o); on > net {
			v.Rank++
		}
	}
	return v
}

// Leaderboard ranks all players by net worth.
func (e *Engine) Leaderboard(limit int) []LeaderRow {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rows := make([]LeaderRow, 0, len(e.players))
	for _, p := range e.players {
		_, net := e.netWorthLocked(p)
		row := LeaderRow{ID: p.ID[:min(8, len(p.ID))], Name: p.Name, NetWorth: net, PnLPct: round2((net/startingCash - 1) * 100),
			Holdings: len(p.Holdings), Bot: p.Bot, Tagline: p.Tagline, Role: p.Role}
		best := math.Inf(-1)
		for sym, pos := range p.Holdings {
			if t, ok := e.tickers[sym]; ok {
				if v := t.Price * float64(pos.Qty); v > best {
					best = v
					row.Top = sym
				}
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].NetWorth != rows[j].NetWorth {
			return rows[i].NetWorth > rows[j].NetWorth
		}
		return rows[i].Name < rows[j].Name
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}
