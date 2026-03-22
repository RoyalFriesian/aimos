package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Sarnga/agent-platform/agents/ceo"
	"github.com/Sarnga/agent-platform/app"
)

func main() {
	service, err := ceo.NewServiceFromEnv("")
	if err != nil {
		log.Fatalf("create CEO service: %v", err)
	}
	defer service.Close()

	server, err := app.NewServer(service)
	if err != nil {
		log.Fatalf("create HTTP server: %v", err)
	}

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("CEO API listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("serve HTTP API: %v", err)
	}
}
