DATASET ?= $(HOME)/Desktop/Pokemon Dataset_1.xlsx
GO      ?= $(shell command -v go 2>/dev/null || echo $(HOME)/sdk/go/bin/go)
ADDR    ?= :8080

.PHONY: all data run dev build test test-ruby test-go clean

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

test: test-ruby test-go

test-ruby:
	ruby etl/test/listing_test.rb

test-go:
	$(GO) vet ./... && $(GO) test ./...

clean:
	rm -rf bin
