package market

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Event is a news item that moved (or will move) prices.
type Event struct {
	ID       int      `json:"id"`
	Tick     int64    `json:"tick"`
	Time     int64    `json:"time"`
	Kind     string   `json:"kind"`
	Headline string   `json:"headline"`
	Body     string   `json:"body"`
	Symbols  []string `json:"symbols"` // most-affected tickers (for linking)
	Impact   float64  `json:"impact"`  // signed, rough magnitude of the headline move in %
}

// generateEvent picks a random market event, applies its price shocks and
// records it to the news feed. Caller must hold e.mu.
func (e *Engine) generateEvent() *Event {
	roll := e.rng.Intn(100)
	switch {
	case roll < 34:
		return e.typeShock()
	case roll < 52:
		return e.sectorMove()
	case roll < 72:
		return e.singleTicker()
	case roll < 82:
		return e.evolutionRumor()
	case roll < 90:
		return e.groupEvent()
	case roll < 96:
		return e.generationRotation()
	default:
		return e.legendaryEvent()
	}
}

// addShock injects a return shock that is applied now and decays over the
// following ticks (see Step). pct is the total intended move in percent.
func addShock(t *Ticker, pct float64) {
	// Geometric series with decay 0.8 sums to 5× the first term.
	t.shock += (pct / 100) / 5
}

func (e *Engine) pick() *Ticker { return e.order[e.rng.Intn(len(e.order))] }

func (e *Engine) pickType() string {
	ts := e.chart.Types()
	return ts[e.rng.Intn(len(ts))]
}

func (e *Engine) topMovers(impacts map[*Ticker]float64, n int) []string {
	type kv struct {
		t *Ticker
		v float64
	}
	var all []kv
	for t, v := range impacts {
		if v != 0 {
			all = append(all, kv{t, v})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if math.Abs(all[i].v) != math.Abs(all[j].v) {
			return math.Abs(all[i].v) > math.Abs(all[j].v)
		}
		return all[i].t.Dex < all[j].t.Dex
	})
	out := make([]string, 0, n)
	for i := 0; i < len(all) && i < n; i++ {
		out = append(out, all[i].t.Symbol)
	}
	return out
}

// typeShock: an attacking type sweeps the market; every ticker moves by
// how (in)effective that type is against it. This is the type chart doing
// the heavy lifting.
func (e *Engine) typeShock() *Event {
	atk := e.pickType()
	k := 1.2 + e.rng.Float64()*1.8 // % per doubling of effectiveness
	impacts := map[*Ticker]float64{}
	weak, resist, immune := 0, 0, 0
	for _, t := range e.order {
		m := e.chart.Effectiveness(atk, t.Types())
		var pct float64
		switch {
		case m == 0:
			pct = 2.5 * k
			immune++
		default:
			pct = -k * math.Log2(m)
			if m > 1 {
				weak++
			} else if m < 1 {
				resist++
			}
		}
		if pct != 0 {
			addShock(t, pct)
			impacts[t] = pct
		}
	}
	heads := []string{
		"%s-type pressure sweeps the exchange",
		"Analysts: %s moves are 'super effective' this quarter",
		"Macro: a %s front rolls through every sector",
		"%s volatility event triggers cross-sector repricing",
	}
	body := fmt.Sprintf("A market-wide %s shock hit the tape. %d listings with %s weakness sold off (up to %.1f%% for double weaknesses) and %d resistant names rallied.",
		atk, weak, atk, 2*k, resist)
	if immune > 0 {
		body += fmt.Sprintf(" %d %s-immune listings caught a %.1f%% safe-haven bid.", immune, atk, 2.5*k)
	}
	ev := Event{
		Kind:     "type",
		Headline: fmt.Sprintf(heads[e.rng.Intn(len(heads))], atk),
		Body:     body,
		Symbols:  e.topMovers(impacts, 8),
		Impact:   -k,
	}
	return e.pushNews(ev)
}

// sectorMove: one type sector rallies or dumps.
func (e *Engine) sectorMove() *Event {
	sector := e.pickType()
	up := e.rng.Intn(2) == 0
	mag := 2 + e.rng.Float64()*3
	if !up {
		mag = -mag
	}
	impacts := map[*Ticker]float64{}
	for _, t := range e.order {
		for i, ty := range t.Types() {
			if ty == sector {
				pct := mag
				if i == 1 {
					pct *= 0.5 // secondary exposure
				}
				addShock(t, pct)
				impacts[t] = pct
			}
		}
	}
	flavor := map[string][2]string{
		"Fire":     {"heats up on strong Blaze guidance", "cools as rain forecast spooks investors"},
		"Water":    {"floods higher on record catch volume", "dries up after Surf demand misses"},
		"Grass":    {"blooms on Chlorophyll productivity gains", "wilts amid reports of a hard frost"},
		"Electric": {"surges after grid-wide Static buildup", "shorts out on Ground-type regulatory scrutiny"},
		"Ice":      {"freezes gains in place after Hail season extension", "melts on unexpected sunny weather"},
		"Fighting": {"punches higher on gym leader endorsements", "takes a hit from a Psychic lobby"},
		"Poison":   {"spreads on Toxic yield expansion", "gets neutralized by an antidote recall"},
		"Ground":   {"quakes higher on Earthquake-proof earnings", "sinks after Flying-type exposure disclosed"},
		"Flying":   {"soars on tailwind commentary", "grounded by Rock-type guidance"},
		"Psychic":  {"levitates on predicted-earnings beat", "crashes amid Dark-type insider trading probe"},
		"Bug":      {"swarms higher on Swarm activity", "squashed by a Flying sell-off"},
		"Rock":     {"solidifies gains on Sturdy outlook", "crumbles under Water pressure"},
		"Ghost":    {"phases higher after haunting Normal-type shorts", "fades on exorcism rumors"},
		"Dragon":   {"roars higher on Multiscale premium", "takes Fairy-type damage"},
		"Dark":     {"climbs after-hours on undisclosed dealings", "exposed by a Fighting-type audit"},
		"Steel":    {"forges ahead on tariff-proof balance sheets", "rusts on Fire-type cost inflation"},
		"Fairy":    {"sparkles on Pixilate demand", "dulled by Steel-type competition"},
		"Normal":   {"performs exactly as expected, which is a relief", "performs exactly as expected, which disappoints"},
	}
	f := flavor[sector]
	verb := f[0]
	if !up {
		verb = f[1]
	}
	ev := Event{
		Kind:     "sector",
		Headline: fmt.Sprintf("%s sector %s", sector, verb),
		Body:     fmt.Sprintf("Listings with primary %s exposure moved about %+.1f%%; secondary-type names about half as much. %d tickers affected.", sector, mag, len(impacts)),
		Symbols:  e.topMovers(impacts, 8),
		Impact:   mag,
	}
	return e.pushNews(ev)
}

// singleTicker: one company has a very specific day. Templates lean on the
// dataset's abilities, categories, dimensions and evolution data.
func (e *Engine) singleTicker() *Event {
	t := e.pick()
	up := e.rng.Intn(2) == 0
	mag := 3 + e.rng.Float64()*6
	if !up {
		mag = -mag
	}
	addShock(t, mag)

	ability := "Run Away"
	if len(t.Abilities) > 0 {
		ability = t.Abilities[e.rng.Intn(len(t.Abilities))]
	}
	category := strings.TrimSuffix(t.Category, " Pokémon")
	weight := 0.0
	if t.WeightKg != nil {
		weight = *t.WeightKg
	}
	height := 0.0
	if t.HeightM != nil {
		height = *t.HeightM
	}

	var templates []string
	if up {
		templates = []string{
			fmt.Sprintf("%s activates %s; shares jump %.1f%%|Traders cheered as %s leaned on its %s operating model. The %s concern is now trading well above the %s-sector average.", t.Name, ability, mag, t.Name, ability, category, t.Sector),
			fmt.Sprintf("%s reports blowout %s numbers|%s's strongest line item, %s, came in ahead of consensus. Book value remains %d BST.", t.Name, bestStat(t.Stats), t.Name, bestStat(t.Stats), t.Stats.BST),
			fmt.Sprintf("%s named 'most promising %s concern' by Gen %d analysts|Coverage initiated with a Buy. Analysts cite a %.1f m frame and a %s moat.", t.Name, category, t.Generation, height, ability),
			fmt.Sprintf("Short squeeze in %s|Bears betting on %s's %s weakness were caught offside; the stock ripped %.1f%% on the day.", t.Name, t.Name, worstStat(t.Stats), mag),
		}
		if t.HiddenAbility != nil {
			templates = append(templates, fmt.Sprintf("%s unveils hidden asset: %s|A previously undisclosed %s capability sent %s shares sharply higher.", t.Name, *t.HiddenAbility, *t.HiddenAbility, t.Name))
		}
		if t.EvolvesFrom != nil {
			templates = append(templates, fmt.Sprintf("%s completes integration of %s|Synergies from the %s spin-out came in ahead of plan.", t.Name, *t.EvolvesFrom, *t.EvolutionMethod))
		}
	} else {
		templates = []string{
			fmt.Sprintf("%s (%.1f kg) reportedly fell through a floor at HQ|Facilities costs are expected to weigh on the %s concern this quarter. Shares slid %.1f%%.", t.Name, weight, category, -mag),
			fmt.Sprintf("%s misses on %s|The company's perennial weak spot, %s, came in below even lowered expectations.", t.Name, worstStat(t.Stats), worstStat(t.Stats)),
			fmt.Sprintf("Regulators question whether %s's %s qualifies as an ability|Shares fell %.1f%% pending clarification.", t.Name, ability, -mag),
			fmt.Sprintf("%s downgraded to 'Not Very Effective'|Analysts flag %s exposure as a persistent drag.", t.Name, riskList(t)),
		}
		if t.PctMale != nil && *t.PctMale == 0 {
			templates = append(templates, fmt.Sprintf("%s hit by Attract-related succession concerns|An all-female board has investors worried about Ditto dependency.", t.Name))
		}
	}
	tpl := templates[e.rng.Intn(len(templates))]
	parts := strings.SplitN(tpl, "|", 2)
	ev := Event{Kind: "ticker", Headline: parts[0], Body: parts[1], Symbols: []string{t.Symbol}, Impact: mag}
	return e.pushNews(ev)
}

// evolutionRumor: an evolution family rallies, final forms the most.
func (e *Engine) evolutionRumor() *Event {
	base := e.pick()
	fam := e.families[base.Family]
	if len(fam) < 2 {
		return e.singleTicker()
	}
	mag := 3 + e.rng.Float64()*4
	impacts := map[*Ticker]float64{}
	for _, t := range fam {
		pct := mag * (0.5 + 0.5*float64(t.Stage))
		addShock(t, pct)
		impacts[t] = pct
	}
	var final *Ticker
	for _, t := range fam {
		if final == nil || t.Stage > final.Stage {
			final = t
		}
	}
	method := "unspecified means"
	if final.EvolutionMethod != nil {
		method = *final.EvolutionMethod
	}
	ev := Event{
		Kind:     "evolution",
		Headline: fmt.Sprintf("Evolution rumor lifts the %s family", base.Family),
		Body:     fmt.Sprintf("Chatter that %s is preparing another round (%s) sent the whole family higher. %s, the final form, led with about %+.1f%%.", base.Family, method, final.Name, impacts[final]),
		Symbols:  e.topMovers(impacts, 8),
		Impact:   mag,
	}
	return e.pushNews(ev)
}

// groupEvent: Starter, Fossil, Paradox, Baby, Ultra Beast indices.
func (e *Engine) groupEvent() *Event {
	groups := []string{"Starter", "Fossil", "Paradox", "Baby", "Ultra Beast"}
	g := groups[e.rng.Intn(len(groups))]
	up := e.rng.Intn(2) == 0
	mag := 2.5 + e.rng.Float64()*3.5
	if !up {
		mag = -mag
	}
	impacts := map[*Ticker]float64{}
	for _, t := range e.order {
		if t.Group == g {
			addShock(t, mag)
			impacts[t] = mag
		}
	}
	text := map[string][2][2]string{
		"Starter":     {{"Starter ETF sees record inflows", "New trainers flooded into the three-choice index."}, {"Starter ETF hit by Professor supply shortage", "Only three are handed out per region; demand is not the problem."}},
		"Fossil":      {{"Fossil revival program expands", "Museum partnerships revive demand for prehistoric paper."}, {"Fossil index cracks on authenticity audit", "Several Old Amber claims could not be verified."}},
		"Paradox":     {{"Paradox index surges on timeline arbitrage", "Investors bet on both past and future at once."}, {"Paradox trading halted amid timeline concerns", "Regulators are unsure which era the filings belong to."}},
		"Baby":        {{"Baby index rallies on daycare optimism", "Growth-stage names are back in favor."}, {"Baby index stumbles; 'too early' say analysts", "High friendship requirements weigh on near-term outlook."}},
		"Ultra Beast": {{"Ultra Beast ETF launches to strong demand", "Beast Boost dynamics attract momentum traders."}, {"Ultra Beast ETF slips as wormholes close", "Interdimensional liquidity has dried up."}},
	}
	tx := text[g][0]
	if !up {
		tx = text[g][1]
	}
	ev := Event{Kind: "group", Headline: tx[0], Body: fmt.Sprintf("%s %d %s-index listings moved about %+.1f%%.", tx[1], len(impacts), g, mag), Symbols: e.topMovers(impacts, 8), Impact: mag}
	return e.pushNews(ev)
}

// generationRotation: funds rotate into one generation and out of the rest.
func (e *Engine) generationRotation() *Event {
	gen := 1 + e.rng.Intn(9)
	mag := 1.5 + e.rng.Float64()*2
	impacts := map[*Ticker]float64{}
	for _, t := range e.order {
		if t.Generation == gen {
			addShock(t, mag)
			impacts[t] = mag
		} else {
			addShock(t, -mag*0.15)
		}
	}
	ev := Event{
		Kind:     "generation",
		Headline: fmt.Sprintf("Fund managers rotate into Generation %d", gen),
		Body:     fmt.Sprintf("Nostalgia flows lifted %d Gen %d listings by about %+.1f%%, funded by light selling everywhere else.", len(impacts), gen, mag),
		Symbols:  e.topMovers(impacts, 8),
		Impact:   mag,
	}
	return e.pushNews(ev)
}

// legendaryEvent: the thin-float rarity names swing.
func (e *Engine) legendaryEvent() *Event {
	up := e.rng.Intn(2) == 0
	mag := 4 + e.rng.Float64()*5
	if !up {
		mag = -mag
	}
	impacts := map[*Ticker]float64{}
	for _, t := range e.order {
		if t.Rarity == "None" {
			continue
		}
		pct := mag
		if t.Rarity == "Mythical" {
			pct *= 1.3
		}
		addShock(t, pct)
		impacts[t] = pct
	}
	head, body := "Legendary sighting confirmed; rarity premium expands", "Thin-float Legendary and Mythical paper ripped higher as collectors chased the tape."
	if !up {
		head, body = "Rarity premium collapses after 'Legendary' label challenged", "A report questioning whether Mythical paper is actually real sent the rarity complex sharply lower."
	}
	ev := Event{Kind: "legendary", Headline: head, Body: fmt.Sprintf("%s %d listings moved about %+.1f%%.", body, len(impacts), mag), Symbols: e.topMovers(impacts, 8), Impact: mag}
	return e.pushNews(ev)
}

func bestStat(s Stats) string  { return extremeStat(s, true) }
func worstStat(s Stats) string { return extremeStat(s, false) }

func extremeStat(s Stats, best bool) string {
	type kv struct {
		n string
		v int
	}
	all := []kv{{"HP", s.HP}, {"Attack", s.Atk}, {"Defense", s.Def}, {"Sp. Atk", s.SpA}, {"Sp. Def", s.SpD}, {"Speed", s.Spe}}
	pick := all[0]
	for _, x := range all[1:] {
		if (best && x.v > pick.v) || (!best && x.v < pick.v) {
			pick = x
		}
	}
	return pick.n
}

func riskList(t *Ticker) string {
	risks := append(append([]string{}, t.Weak4x...), t.Weak2x...)
	if len(risks) == 0 {
		return "general market"
	}
	if len(risks) > 3 {
		risks = risks[:3]
	}
	return strings.Join(risks, "/")
}
