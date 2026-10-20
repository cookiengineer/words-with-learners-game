package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cookiengineer/words-with-learners-app/internal/config"
	"github.com/cookiengineer/words-with-learners-app/internal/server"
)

func main() {
	cfg := config.Parse()
	app, err := server.New(cfg)
	if err != nil {
		log.Fatalf("startup failed: %v", err)
	}
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          log.New(os.Stderr, "[http] ", log.LstdFlags),
	}
	log.Printf("Words with Learners listening on %s (data: %s)", cfg.Addr, cfg.DataDir)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
