package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/carissaayo/go-api-gateway/internal/config"
	"github.com/carissaayo/go-api-gateway/internal/gateway"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	gw := gateway.New(cfg)

	// Start server in a goroutine so it doesn't block signal handling
	errCh := make(chan error, 1)
	go func() {
		if err := gw.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Block until we receive SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}

	// Give in-flight requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := gw.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Gateway stopped gracefully")
}
