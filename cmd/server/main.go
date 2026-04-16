package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"lobster-world-core/internal/gateway"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           gateway.NewApp().Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("lobster-world-core server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
