DATASET ?= $(HOME)/Desktop/Pokemon Dataset_1.xlsx
GO      ?= $(shell command -v go 2>/dev/null || echo $(HOME)/sdk/go/bin/go)
ADDR    ?= :8080

.PHONY: all data run dev build site preview test test-ruby test-go clean

all: data build

## data: run the Ruby listing desk — turns the spreadsheet into data/*.json
data:
	ruby etl/import.rb "$(DATASET)"

## run: start the exchange on $(ADDR) (embedded frontend)
run:
	$(GO) run ./cmd/pokestock -addr $(ADDR)

## dev: start the exchange serving ./web from disk (edit CSS/JS, just reload)
dev:
	$(GO) run ./cmd/pokestock -addr $(ADDR) -dev

## build: single static binary at ./bin/pokestock
build:
	mkdir -p bin && $(GO) build -o bin/pokestock ./cmd/pokestock

## site: browser-only build in ./site for GitHub Pages (the engine runs as WebAssembly, no server)
site:
	rm -rf site && mkdir -p site
	cp -R web/. site/
	GOOS=js GOARCH=wasm $(GO) build -trimpath -ldflags="-s -w" -o site/pokestock.wasm ./cmd/pokestock-web
	cp "$$($(GO) env GOROOT)/lib/wasm/wasm_exec.js" site/ 2>/dev/null || cp "$$($(GO) env GOROOT)/misc/wasm/wasm_exec.js" site/
	cp cmd/pokestock-web/bridge.js site/
	sed 's|<script src="app.js"></script>|<script src="wasm_exec.js"></script><script src="bridge.js"></script><script src="app.js"></script>|' web/index.html > site/index.html
	grep -q 'src="bridge.js"' site/index.html || { echo 'site: could not add bridge.js to site/index.html' >&2; exit 1; }

## preview: build ./site and serve it at http://localhost:8000
preview: site
	cd site && python3 -m http.server 8000

test: test-ruby test-go

test-ruby:
	ruby etl/test/listing_test.rb

test-go:
	$(GO) vet ./... && $(GO) test ./...

clean:
	rm -rf bin site
