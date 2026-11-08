//go:build js && wasm

// Command pokestock-web runs the whole PokéStock Exchange inside the browser.
//
// It is the same engine and the same /api handlers as cmd/pokestock, compiled
// to WebAssembly. Instead of listening on a port, it hands two functions to
// JavaScript, and bridge.js (next to this file) sends the frontend's
// fetch("/api/...") calls and its EventSource("/api/stream") to them. The
// result is a folder of static files that GitHub Pages can host for free.
//
// Build it with `make site`.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"syscall/js"

	"pokestock"
	"pokestock/internal/api"
	"pokestock/internal/market"
)

func main() {
	// The API logs every request; in a browser that would flood the console.
	log.SetOutput(io.Discard)

	listings, chart, err := market.LoadData(pokestock.Data)
	if err != nil {
		js.Global().Get("console").Call("error", "pokestock: "+err.Error())
		return // bridge.js notices the program exited and shows an error
	}
	eng := market.NewEngine(listings, chart, 0) // seed 0 = random
	eng.AddBots()                              // the anime cast trades alongside you
	eng.Warm(60)                               // simulate some history so charts aren't empty
	go eng.Run(make(chan struct{}))

	b := &bridge{handler: api.New(eng, noFiles{}), cookies: map[string]string{}}
	js.Global().Set("pokestockRequest", js.FuncOf(b.request))
	js.Global().Set("pokestockStream", js.FuncOf(b.stream))
	if ready := js.Global().Get("pokestockReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}

	select {} // keep the exchange running for as long as the page is open
}

// noFiles is an empty file system. GitHub Pages serves the frontend files,
// so the API handler never has to.
type noFiles struct{}

func (noFiles) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

// bridge runs requests from JavaScript through the real API handler.
type bridge struct {
	handler http.Handler

	mu      sync.Mutex        // one request at a time, like a tiny single-player server
	cookies map[string]string // stands in for the browser's cookie jar (the trainer ID)
}

// request is pokestockRequest(method, url, body), which returns a Promise of
// {status, headers, body}. The work happens on its own goroutine because a
// call from JavaScript into Go that blocks would freeze the page.
func (b *bridge) request(_ js.Value, args []js.Value) any {
	method, url, body := args[0].String(), args[1].String(), args[2].String()

	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, p []js.Value) any {
		resolve := p[0]
		go func() { resolve.Invoke(b.serve(method, url, body)) }()
		return nil
	})
	promise := js.Global().Get("Promise").New(executor)
	executor.Release() // the Promise constructor has already called it
	return promise
}

func (b *bridge) serve(method, url, body string) (res map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			msg, _ := json.Marshal(map[string]string{"error": fmt.Sprint(r)})
			res = map[string]any{
				"status":  http.StatusInternalServerError,
				"headers": map[string]any{"Content-Type": "application/json"},
				"body":    string(msg),
			}
		}
	}()

	b.mu.Lock()
	defer b.mu.Unlock()

	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range b.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	w := newWriter(nil)
	b.handler.ServeHTTP(w, req)

	// Remember any cookie the handler set, as a browser would.
	for _, c := range (&http.Response{Header: w.header}).Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c.Value
		}
	}
	headers := map[string]any{}
	for name, values := range w.header {
		if name != "Set-Cookie" {
			headers[name] = strings.Join(values, ", ")
		}
	}
	return map[string]any{"status": w.code(), "headers": headers, "body": w.buf.String()}
}

// stream is pokestockStream(url, onChunk), which returns a stop function. It
// runs a streaming handler (the /api/stream server-sent events) and passes
// each flushed piece of text to onChunk, or null if the stream ends on its own.
func (b *bridge) stream(_ js.Value, args []js.Value) any {
	url, onChunk := args[0].String(), args[1]

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return js.Null()
	}
	w := newWriter(func(chunk string) { onChunk.Invoke(chunk) })
	go func() {
		defer func() {
			_ = recover() // a broken stream must not take the whole exchange down
			if ctx.Err() == nil {
				onChunk.Invoke(js.Null())
			}
			cancel()
		}()
		b.handler.ServeHTTP(w, req)
	}()

	var stop js.Func
	stop = js.FuncOf(func(js.Value, []js.Value) any {
		cancel()
		stop.Release()
		return nil
	})
	return stop
}

// writer is the http.ResponseWriter the handlers write into. It collects the
// body; for the event stream, every Flush hands the collected text to JS.
type writer struct {
	header  http.Header
	status  int
	buf     bytes.Buffer
	onFlush func(string)
}

func newWriter(onFlush func(string)) *writer {
	return &writer{header: http.Header{}, onFlush: onFlush}
}

func (w *writer) Header() http.Header { return w.header }

func (w *writer) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *writer) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.buf.Write(p)
}

func (w *writer) Flush() {
	if w.onFlush != nil && w.buf.Len() > 0 {
		w.onFlush(w.buf.String())
		w.buf.Reset()
	}
}

func (w *writer) code() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
