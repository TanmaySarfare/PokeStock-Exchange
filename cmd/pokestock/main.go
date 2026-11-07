// Command pokestock runs the PokéStock Exchange: a fake stock market where
// every Pokémon is a ticker.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pokestock"
	"pokestock/internal/api"
	"pokestock/internal/market"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	tick := flag.Duration("tick", 2*time.Second, "market tick interval")
	seed := flag.Int64("seed", 0, "RNG seed (0 = random)")
	warm := flag.Int("warm", 60, "ticks to simulate before opening, so charts aren't empty")
	dev := flag.Bool("dev", false, "serve ./web from disk instead of the embedded copy (live-reload CSS/JS)")
	flag.Parse()

	listings, chart, err := market.LoadData(pokestock.Data)
	if err != nil {
		log.Fatal(err)
	}
	eng := market.NewEngine(listings, chart, *seed)
	eng.TickInterval = *tick
	eng.Warm(*warm)

	var web fs.FS
	if *dev {
		web = os.DirFS("web")
	} else {
		web, err = fs.Sub(pokestock.Web, "web")
		if err != nil {
			log.Fatal(err)
		}
	}

	stop := make(chan struct{})
	go eng.Run(stop)

	srv := &http.Server{Addr: *addr, Handler: api.New(eng, web), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		close(stop)
		_ = srv.Close()
	}()

	fmt.Printf("PokéStock Exchange open for trading: %d listings, tick every %s\n", len(listings), *tick)
	fmt.Printf("→ http://localhost%s\n", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
