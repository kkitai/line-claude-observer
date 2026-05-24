package main

import (
	"log"

	"github.com/kkitai/line-claude-observer/app/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("starting server on port %s", cfg.Port)
}
