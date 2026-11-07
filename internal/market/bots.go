package market

import (
	"math"
	"sort"
)

// Bot strategies. Each bot is a Player that trades on its own every tick with
// some probability, so the leaderboard is alive even with one human online.
const (
	stratMomentum   = "momentum"   // chases today's winners, cuts losers fast
	stratContrarian = "contrarian" // buys the dip, sells the rip
	stratSector     = "sector"     // loyal to one or two type sectors
	stratRarity     = "rarity"     // only Legendary / Mythical paper
	stratHodl       = "hodl"       // buys favourites and never sells
	stratChaos      = "chaos"      // Team Rocket: random, over-sized, frequent
	stratIndex      = "index"      // spreads cash across many listings
)

type botSpec struct {
	id       string
	name     string
	tagline  string
	role     string // Gym Leader / Elite Four / Champion, shown as a badge
	strategy string
	sectors  []string // for stratSector
	favs     []string // tickers for stratHodl
	activity float64  // probability of acting on any given tick
	size     float64  // fraction of cash per buy
}

// cast is the roster of anime characters that run accounts on the exchange.
var cast = []botSpec{
	{id: "bot:ash", name: "Ash Ketchum", tagline: "Gotta buy 'em all", role: "Champion", strategy: stratHodl,
		favs: []string{"PIKA", "CHRZ", "GREN", "LUCA", "INFE", "SCEP", "BULB", "SQUI", "CHAR"}, activity: .12, size: .12},
	{id: "bot:misty", name: "Misty", tagline: "Cerulean Water Fund", role: "Gym Leader", strategy: stratSector, sectors: []string{"Water"}, activity: .10, size: .10},
	{id: "bot:brock", name: "Brock", tagline: "Pewter Rock & Ground Partners", role: "Gym Leader", strategy: stratSector, sectors: []string{"Rock", "Ground"}, activity: .08, size: .10},
	{id: "bot:gary", name: "Gary Oak", tagline: "Smell ya later, bagholders", strategy: stratMomentum, activity: .22, size: .15},
	{id: "bot:jessie", name: "Jessie", tagline: "Team Rocket Capital", strategy: stratChaos, activity: .30, size: .25},
	{id: "bot:james", name: "James", tagline: "Team Rocket Capital", strategy: stratChaos, activity: .28, size: .22},
	{id: "bot:meowth", name: "Meowth", tagline: "That's right! Pay Day Arbitrage", strategy: stratContrarian, activity: .20, size: .12},
	{id: "bot:oak", name: "Professor Oak", tagline: "Pallet Town Index", strategy: stratIndex, activity: .18, size: .05},
	{id: "bot:cynthia", name: "Cynthia", tagline: "Sinnoh Champion Holdings", role: "Champion", strategy: stratRarity, activity: .09, size: .18},
	{id: "bot:lance", name: "Lance", tagline: "Blackthorn Dragon Trust", role: "Elite Four", strategy: stratSector, sectors: []string{"Dragon", "Flying"}, activity: .09, size: .14},
	{id: "bot:giovanni", name: "Giovanni", tagline: "Viridian Gym Holdings", role: "Gym Leader", strategy: stratMomentum, activity: .15, size: .20},
	{id: "bot:dawn", name: "Dawn", tagline: "Twinleaf Contest Fund", strategy: stratContrarian, activity: .13, size: .10},
	{id: "bot:may", name: "May", tagline: "Petalburg Growth", strategy: stratSector, sectors: []string{"Fire", "Grass"}, activity: .11, size: .10},
	{id: "bot:serena", name: "Serena", tagline: "Vaniville Fairy Select", strategy: stratSector, sectors: []string{"Fairy", "Fire"}, activity: .10, size: .10},
	{id: "bot:steven", name: "Steven Stone", tagline: "Devon Steel & Minerals", role: "Champion", strategy: stratSector, sectors: []string{"Steel", "Rock"}, activity: .08, size: .15},
	{id: "bot:ltsurge", name: "Lt. Surge", tagline: "Vermilion Electric Co-op", role: "Gym Leader", strategy: stratSector, sectors: []string{"Electric"}, activity: .12, size: .12},
	{id: "bot:sabrina", name: "Sabrina", tagline: "Saffron Psychic Macro", role: "Gym Leader", strategy: stratSector, sectors: []string{"Psychic", "Ghost"}, activity: .07, size: .14},
	{id: "bot:nurse", name: "Nurse Joy", tagline: "Pokémon Center Pension", strategy: stratIndex, activity: .10, size: .04},
	{id: "bot:jenny", name: "Officer Jenny", tagline: "Compliance desk", strategy: stratIndex, activity: .06, size: .06},
	{id: "bot:paul", name: "Paul", tagline: "Only the strong get allocated", strategy: stratMomentum, activity: .18, size: .18},
	{id: "bot:iris", name: "Iris", tagline: "Opelucid Dragon Growth", role: "Champion", strategy: stratSector, sectors: []string{"Dragon"}, activity: .11, size: .12},
	{id: "bot:cilan", name: "Cilan", tagline: "Striaton tasting notes", role: "Gym Leader", strategy: stratSector, sectors: []string{"Grass", "Bug"}, activity: .09, size: .09},
	{id: "bot:goh", name: "Goh", tagline: "Project Mew: catch every ticker", strategy: stratIndex, activity: .25, size: .03},
	{id: "bot:tracey", name: "Tracey Sketchit", tagline: "Orange Islands Research", strategy: stratContrarian, activity: .10, size: .08},
}

type bot struct {
	spec   botSpec
	player *Player
}

// AddBots enrolls the anime cast as autonomous traders.
func (e *Engine) AddBots() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, spec := range cast {
		if _, exists := e.players[spec.id]; exists {
			continue
		}
		p := &Player{ID: spec.id, Name: spec.name, Tagline: spec.tagline, Role: spec.role, Cash: startingCash,
			Holdings: map[string]*Position{}, Created: e.now, Bot: true}
		e.players[spec.id] = p
		e.bots = append(e.bots, &bot{spec: spec, player: p})
	}
}

// runBotsLocked gives each bot a chance to trade. Caller must hold e.mu.
func (e *Engine) runBotsLocked(now int64) {
	if len(e.bots) == 0 {
		return
	}
	var byPct []*Ticker // lazily sorted by % change, best first
	sorted := func() []*Ticker {
		if byPct == nil {
			byPct = append([]*Ticker{}, e.order...)
			sort.Slice(byPct, func(i, j int) bool { return byPct[i].Pct() > byPct[j].Pct() })
		}
		return byPct
	}
	for _, b := range e.bots {
		if e.rng.Float64() > b.spec.activity {
			continue
		}
		e.botActLocked(b, sorted, now)
	}
}

func (e *Engine) botActLocked(b *bot, sorted func() []*Ticker, now int64) {
	p, s := b.player, b.spec
	buy := func(t *Ticker, frac float64) {
		if t == nil {
			return
		}
		ask := t.Price * (1 + spreadFor(t)/2)
		qty := int64(math.Floor(p.Cash * frac / ask))
		if qty < 1 {
			return
		}
		_, _ = e.tradeLocked(p, t, "buy", qty, now)
	}
	sell := func(sym string, frac float64) {
		pos := p.Holdings[sym]
		t := e.tickers[sym]
		if pos == nil || t == nil {
			return
		}
		qty := int64(math.Ceil(float64(pos.Qty) * frac))
		if qty < 1 {
			qty = 1
		}
		if qty > pos.Qty {
			qty = pos.Qty
		}
		_, _ = e.tradeLocked(p, t, "sell", qty, now)
	}
	// gain returns unrealised % P&L of a holding.
	gain := func(sym string) float64 {
		pos := p.Holdings[sym]
		t := e.tickers[sym]
		if pos == nil || t == nil || pos.AvgCost == 0 {
			return 0
		}
		return (t.Price/pos.AvgCost - 1) * 100
	}
	pickFrom := func(pool []*Ticker) *Ticker {
		if len(pool) == 0 {
			return nil
		}
		return pool[e.rng.Intn(len(pool))]
	}

	switch s.strategy {
	case stratMomentum:
		// Cut anything down 3%, take profit above 8%, otherwise chase the top 40.
		for sym := range p.Holdings {
			if g := gain(sym); g < -3 || g > 8 {
				sell(sym, 1)
				return
			}
		}
		buy(pickFrom(sorted()[:40]), s.size)

	case stratContrarian:
		for sym := range p.Holdings {
			if g := gain(sym); g > 5 {
				sell(sym, 1)
				return
			}
		}
		all := sorted()
		buy(pickFrom(all[len(all)-40:]), s.size)

	case stratSector:
		if len(p.Holdings) > 0 && e.rng.Float64() < 0.25 {
			for sym := range p.Holdings {
				if g := gain(sym); g > 6 || g < -10 {
					sell(sym, 0.5)
					return
				}
			}
		}
		var pool []*Ticker
		for _, t := range e.order {
			for _, ty := range t.Types() {
				for _, want := range s.sectors {
					if ty == want {
						pool = append(pool, t)
					}
				}
			}
		}
		buy(pickFrom(pool), s.size)

	case stratRarity:
		if len(p.Holdings) > 0 && e.rng.Float64() < 0.2 {
			for sym := range p.Holdings {
				if gain(sym) > 10 {
					sell(sym, 1)
					return
				}
			}
		}
		var pool []*Ticker
		for _, t := range e.order {
			if t.Rarity != "None" {
				pool = append(pool, t)
			}
		}
		buy(pickFrom(pool), s.size)

	case stratHodl:
		var pool []*Ticker
		for _, sym := range s.favs {
			if t, ok := e.tickers[sym]; ok {
				pool = append(pool, t)
			}
		}
		if len(pool) == 0 {
			pool = e.order
		}
		buy(pickFrom(pool), s.size)

	case stratChaos:
		// Team Rocket: flip a coin, go big, sell at random. Blasting off again.
		if len(p.Holdings) > 0 && e.rng.Intn(2) == 0 {
			for sym := range p.Holdings {
				sell(sym, 1)
				return
			}
		}
		buy(e.pick(), s.size*(0.5+e.rng.Float64()))

	case stratIndex:
		if len(p.Holdings) > 60 || p.Cash < 50 {
			// Fully invested; rebalance by trimming a winner.
			for sym := range p.Holdings {
				if gain(sym) > 7 {
					sell(sym, 0.5)
					return
				}
			}
			return
		}
		buy(e.pick(), s.size)
	}
}
