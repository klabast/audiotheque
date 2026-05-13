package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	mpdAddr := envOr("MPD_ADDR", ":6600")
	httpAddr := envOr("HTTP_ADDR", ":6601")

	srv := NewMPDServer()

	// Start MPD protocol listener
	go func() {
		log.Printf("MPD protocol listening on %s", mpdAddr)
		if err := srv.ListenAndServe(mpdAddr); err != nil {
			log.Fatalf("MPD server error: %v", err)
		}
	}()

	// Start HTTP observation API
	mux := http.NewServeMux()
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, r *http.Request) {
		state := srv.State()
		log.Printf("HTTP GET /state → %s vol=%d playlist=%d", state.PlayState, state.Volume, len(state.Playlist))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})
	mux.HandleFunc("GET /history", func(w http.ResponseWriter, r *http.Request) {
		history := srv.History()
		log.Printf("HTTP GET /history → %d commands", len(history))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
	})
	mux.HandleFunc("POST /reset", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HTTP POST /reset")
		srv.Reset()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /track-ended", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HTTP POST /track-ended")
		srv.EndCurrentTrack()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /mixer-disable", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HTTP POST /mixer-disable")
		srv.SetMixerless(true)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /mixer-enable", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HTTP POST /mixer-enable")
		srv.SetMixerless(false)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	log.Printf("HTTP observation API listening on %s", httpAddr)
	if err := http.ListenAndServe(httpAddr, mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
