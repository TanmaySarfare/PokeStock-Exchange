package market

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// Listing is one tradable security, as produced by the Ruby listing desk.
type Listing struct {
	Symbol          string   `json:"symbol"`
	Name            string   `json:"name"`
	Dex             int      `json:"dex"`
	Generation      int      `json:"generation"`
	Category        string   `json:"category"`
	Sector          string   `json:"sector"`
	Subsector       *string  `json:"subsector"`
	Abilities       []string `json:"abilities"`
	HiddenAbility   *string  `json:"hidden_ability"`
	Stats           Stats    `json:"stats"`
	Weak4x          []string `json:"weak4x"`
	Weak2x          []string `json:"weak2x"`
	Resists         []string `json:"resists"`
	Immune          []string `json:"immune"`
	EvolvesFrom     *string  `json:"evolves_from"`
	EvolutionMethod *string  `json:"evolution_method"`
	Stage           int      `json:"stage"`
	Family          string   `json:"family"`
	HeightM         *float64 `json:"height_m"`
	WeightKg        *float64 `json:"weight_kg"`
	PctMale         *float64 `json:"pct_male"`
	Rarity          string   `json:"rarity"`
	Group           string   `json:"group"`
	Image           string   `json:"image"`
	IPOPrice        float64  `json:"ipo_price"`
	Volatility      float64  `json:"volatility"`
	Drift           float64  `json:"drift"`
	Shares          int64    `json:"shares"`
	Prospectus      string   `json:"prospectus"`
}

// Stats are the six base stats plus the total.
type Stats struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
	BST int `json:"bst"`
}

// Types returns the one or two types of the listing.
func (l *Listing) Types() []string {
	if l.Subsector != nil && *l.Subsector != "" {
		return []string{l.Sector, *l.Subsector}
	}
	return []string{l.Sector}
}

// TypeChart maps attacking type -> defending type -> damage multiplier.
type TypeChart map[string]map[string]float64

// Effectiveness returns the combined multiplier of an attacking type
// against a defender with the given types.
func (tc TypeChart) Effectiveness(attacker string, defender []string) float64 {
	row, ok := tc[attacker]
	if !ok {
		return 1
	}
	m := 1.0
	for _, t := range defender {
		if v, ok := row[t]; ok {
			m *= v
		}
	}
	return m
}

// Types returns all attacking types in a stable order.
func (tc TypeChart) Types() []string {
	order := []string{"Normal", "Fire", "Water", "Electric", "Grass", "Ice", "Fighting", "Poison", "Ground",
		"Flying", "Psychic", "Bug", "Rock", "Ghost", "Dragon", "Dark", "Steel", "Fairy"}
	out := make([]string, 0, len(order))
	for _, t := range order {
		if _, ok := tc[t]; ok {
			out = append(out, t)
		}
	}
	return out
}

// LoadData reads listings and the type chart from an fs (the embedded data dir).
func LoadData(fsys fs.FS) ([]*Listing, TypeChart, error) {
	raw, err := fs.ReadFile(fsys, "data/listings.json")
	if err != nil {
		return nil, nil, fmt.Errorf("read listings: %w", err)
	}
	var listings []*Listing
	if err := json.Unmarshal(raw, &listings); err != nil {
		return nil, nil, fmt.Errorf("parse listings: %w", err)
	}
	raw, err = fs.ReadFile(fsys, "data/typechart.json")
	if err != nil {
		return nil, nil, fmt.Errorf("read typechart: %w", err)
	}
	var chart TypeChart
	if err := json.Unmarshal(raw, &chart); err != nil {
		return nil, nil, fmt.Errorf("parse typechart: %w", err)
	}
	for _, l := range listings {
		l.Symbol = strings.ToUpper(l.Symbol)
	}
	return listings, chart, nil
}
