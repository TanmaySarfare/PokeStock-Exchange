// Package api exposes the market engine over HTTP (JSON + SSE) and serves
// the embedded frontend.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pokestock/internal/market"
)

const playerCookie = "pst_trainer"

// Server wires routes to the engine.
type Server struct {
	engine *market.Engine
	web    fs.FS
	mux    *http.ServeMux
}

// New builds the handler. web is the embedded frontend rooted at "web/".
func New(engine *market.Engine, web fs.FS) *Server {
	s := &Server{engine: engine, web: web, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mux.ServeHTTP(w, r)
	if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/stream" {
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Microsecond))
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/market", s.handleMarket)
	s.mux.HandleFunc("GET /api/ticker/{symbol}", s.handleTicker)
	s.mux.HandleFunc("GET /api/news", s.handleNews)
	s.mux.HandleFunc("GET /api/stream", s.handleStream)
	s.mux.HandleFunc("GET /api/me", s.handlePortfolio)
	s.mux.HandleFunc("POST /api/me", s.handleRename)
	s.mux.HandleFunc("POST /api/trade", s.handleTrade)
	s.mux.HandleFunc("GET /api/leaderboard", s.handleLeaderboard)
	s.mux.HandleFunc("GET /api/trainer/{id}", s.handleTrainer)
	files := http.FileServer(http.FS(s.web))
	s.mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/img/") || strings.HasSuffix(r.URL.Path, ".ico") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else {
			// HTML/CSS/JS change together; keep them fresh so UI updates land on reload.
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	}))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// playerID reads or issues the trainer cookie.
func (s *Server) playerID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(playerCookie); err == nil && len(c.Value) >= 16 {
		return c.Value
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: playerCookie, Value: id, Path: "/", MaxAge: 60 * 60 * 24 * 365, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return id
}

func (s *Server) handleMarket(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.engine.Market())
}

func (s *Server) handleTicker(w http.ResponseWriter, r *http.Request) {
	sym := strings.ToUpper(r.PathValue("symbol"))
	d := s.engine.Ticker(sym)
	if d == nil {
		writeErr(w, 404, market.ErrUnknownSymbol)
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) handleNews(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 120 {
		limit = v
	}
	writeJSON(w, 200, s.engine.News(limit))
}

func (s *Server) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	id := s.playerID(w, r)
	writeJSON(w, 200, s.engine.Portfolio(id))
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	id := s.playerID(w, r)
	var body struct {
		Name    *string `json:"name"`
		Tagline *string `json:"tagline"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeErr(w, 400, fmt.Errorf("bad json: %w", err))
		return
	}
	if body.Name != nil {
		s.engine.Rename(id, strings.TrimSpace(*body.Name))
	}
	if body.Tagline != nil {
		s.engine.SetTagline(id, strings.TrimSpace(*body.Tagline))
	}
	writeJSON(w, 200, s.engine.Portfolio(id))
}

func (s *Server) handleTrade(w http.ResponseWriter, r *http.Request) {
	id := s.playerID(w, r)
	var body struct {
		Symbol string `json:"symbol"`
		Side   string `json:"side"`
		Qty    int64  `json:"qty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeErr(w, 400, fmt.Errorf("bad json: %w", err))
		return
	}
	tr, err := s.engine.Trade(id, strings.ToUpper(body.Symbol), strings.ToLower(body.Side), body.Qty)
	if err != nil {
		status := 400
		if errors.Is(err, market.ErrUnknownSymbol) {
			status = 404
		}
		writeErr(w, status, err)
		return
	}
	writeJSON(w, 200, map[string]any{"trade": tr, "portfolio": s.engine.Portfolio(id)})
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.engine.Leaderboard(40))
}

func (s *Server) handleTrainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t := s.engine.Trainer(id)
	if t == nil {
		writeJSON(w, 404, map[string]string{"error": "unknown trainer"})
		return
	}
	writeJSON(w, 200, t)
}

// handleStream pushes every tick as a server-sent event.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	fmt.Fprintf(w, "retry: 2000\n\n")
	flusher.Flush()

	ch, cancel := s.engine.Subscribe()
	defer cancel()
	for {
		select {
		case <-r.Context().Done():
			return
		case upd := <-ch:
			b, err := json.Marshal(upd)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: tick\ndata: %s\n\n", b)
			flusher.Flush()
		}
	}
}
