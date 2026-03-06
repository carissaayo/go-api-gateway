package main

import (
	"fmt"
	"os"

	"github.com/carissaayo/go-api-gateway/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Gateway configured on port %d\n", cfg.Server.Port)
}
