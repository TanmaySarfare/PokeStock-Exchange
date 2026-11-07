# PokéStock Exchange

A fake stock market where every one of the 1,025 Pokémon in `Pokemon Dataset_1.xlsx`
is a publicly traded ticker. Prices random-walk live, the **type chart drives the
macro**: when a "Rock-type shock" hits, Charizard (4× weak) dumps and Steel-types
rally. Buy, sell, and climb the leaderboard with $10,000 of play money.

Two languages, two jobs:

| Layer | Language | Role |
| --- | --- | --- |
| `etl/` | **Ruby** (stdlib only) | The *listing desk*. Reads the `.xlsx` with no gems (unzip + regex), assigns ticker symbols, derives IPO price / volatility / float from stats & rarity, writes a prospectus per Pokémon, exports `data/*.json`. |
| `cmd/`, `internal/` | **Go** (stdlib only) | The *exchange*. Simulation engine, type-chart news events, OHLC candles, SSE price stream, portfolios, leaderboard, embedded frontend — one static binary. |
| `web/` | Vanilla JS/CSS | Bloomberg-terminal-style UI: ticker tape, sector heatmap, candlestick charts, trade box. No build step. |

## Run it

```sh
make data     # Ruby: spreadsheet -> data/listings.json + data/typechart.json
make run      # Go:   http://localhost:8080
```

`make dev` serves `web/` from disk so CSS/JS edits only need a browser reload.
`make test` runs the Ruby minitest suite and the Go tests.

Flags: `-addr :8080`, `-tick 2s` (market speed), `-warm 60` (ticks of pre-simulated
history), `-seed N` (deterministic market), `-dev`.

If `go` isn't on your `PATH`, the Makefile falls back to `~/sdk/go/bin/go`.
If your spreadsheet lives elsewhere: `make data DATASET=/path/to/file.xlsx`.

## How the data becomes a market

- **Ticker symbol** – 4–5 letters from the name (`BULB`, `CHRZ`, `MRMI`), collision-resolved.
- **IPO price** – `BST^1.5 / 100` × rarity multiplier (Legendary 1.6×, Mythical 2.1×) × group multiplier (Paradox, Ultra Beast, Starter…) × evolution-stage bonus. Arceus IPOs at $405; Wishiwashi at $23.
- **Volatility (β)** – fast, frail Pokémon are jumpy (`Speed/BST` up, `HP+Def+SpDef` down). Legendaries swing harder because of the thin float.
- **Float** – 1,000,000 shares for commons, 120,000 for Legendaries, 40,000 for Mythicals. Spread widens with rarity.
- **Drift** – early evolution stages are "growth stocks" with a slight upward bias.
- **Sector** – Type 1 (with Type 2 as secondary exposure). 18 sectors, each with a live heatmap tile.

## News events (all computed from the dataset)

| Kind | What happens |
| --- | --- |
| `type` | An attacking type sweeps the market. Each ticker moves by `-k · log2(effectiveness)` from the type chart: 2× weak falls, 4× weak falls twice as hard, resistant rises, **immune gets a safe-haven bid**. |
| `sector` | One type sector rallies or dumps (secondary-type names move half as much). |
| `ticker` | Single-company news built from abilities, categories, weight/height, best/worst stat, evolution method, hidden ability. |
| `evolution` | An evolution family rallies; the final stage leads. |
| `group` | Starter / Fossil / Paradox / Baby / Ultra Beast indices move together. |
| `generation` | Funds rotate into one generation, out of the rest. |
| `legendary` | The rarity complex swings as a block. |

Prices mean-revert gently toward IPO, so nothing runs off to zero or infinity.

## API

| Route | Description |
| --- | --- |
| `GET /api/market` | All tickers + sector aggregates + composite index |
| `GET /api/ticker/{SYM}` | Full detail: listing, candles, family, related news, spread |
| `GET /api/news?limit=50` | Market wire, newest first |
| `GET /api/stream` | Server-sent events, one `tick` event per market tick |
| `GET /api/me` / `POST /api/me {name}` | Your portfolio (cookie-identified) / rename |
| `POST /api/trade {symbol, side, qty}` | Market order; buys fill at ask, sells at bid |
| `GET /api/leaderboard` | Top trainers by net worth |

Accounts live in server memory and reset on restart.

## Layout

```
etl/            Ruby listing desk (import.rb, lib/xlsx.rb, lib/listing.rb, test/)
data/           Generated JSON (committed; regenerate with `make data`)
cmd/pokestock/  Go entrypoint
internal/market Engine, news generator, players
internal/api    HTTP + SSE handlers
web/            index.html, app.js, style.css (embedded into the binary)
embed.go        go:embed for data/ and web/
```

Artwork is loaded from the `Image URL` column (PokeAPI official artwork on GitHub).
