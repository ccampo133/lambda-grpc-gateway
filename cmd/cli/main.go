package main

import (
	"context"
	"fmt"
	"net"

	"github.com/ccampo133/lambda-grpc-gateway/internal/config"
	"github.com/ccampo133/lambda-grpc-gateway/internal/server"
	log "github.com/sirupsen/logrus"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.ParseFlags()
	logLvl, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.WithError(err).Error("error parsing log level; defaulting to info")
		logLvl = log.InfoLevel
	}
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(logLvl)

	// Create network protocol listeners.
	httpListener, err := net.Listen(cfg.HttpNetwork, cfg.HttpAddress)
	defer closeListener(httpListener)
	if err != nil {
		return fmt.Errorf("error creating HTTP listener: %w", err)
	}
	grpcListener, err := net.Listen(cfg.GrpcNetwork, cfg.GrpcAddress)
	defer closeListener(grpcListener)
	if err != nil {
		return fmt.Errorf("error creating gRPC listener: %w", err)
	}

	// Create and initialize the application server.
	app := server.NewApplicationServer(cfg)
	if err := app.Init(ctx); err != nil {
		return fmt.Errorf("error initializing application server: %w", err)
	}

	// Serve the application server.
	if err := app.Serve(ctx, grpcListener, httpListener); err != nil {
		return fmt.Errorf("error serving application server: %w", err)
	}
	return nil
}

func closeListener(listener net.Listener) {
	if listener != nil {
		_ = listener.Close()
	}
}
