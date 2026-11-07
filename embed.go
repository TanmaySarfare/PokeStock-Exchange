// Package pokestock embeds the generated data and the static frontend so the
// server ships as a single binary.
package pokestock

import "embed"

//go:embed data/listings.json data/typechart.json
var Data embed.FS

//go:embed web
var Web embed.FS
