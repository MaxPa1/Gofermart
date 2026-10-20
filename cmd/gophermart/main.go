package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/MaxPa1/Gofermart/internal/app"
	"github.com/MaxPa1/Gofermart/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:], os.Environ())
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return app.Run(ctx, cfg)
}
