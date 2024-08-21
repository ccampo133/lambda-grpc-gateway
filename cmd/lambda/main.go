package main

import (
	"context"
	"fmt"
	"net"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
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
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.DebugLevel)

	// Create the gRPC network listener. Even though the Lambda acts as an HTTP
	// API, we only need to run a gRPC server to ensure proper functionality
	// (such as enabling gRPC interceptors, etc.).
	//
	// The HTTP aspect of the application is handled directly by grpc-gateway's
	// HTTP multiplexer (app.Mux), and the Lambda library's internals (see the
	// lambda.Start call below). The way it works is as follows: the gRPC server
	// listens on a Unix socket, and the grpc-gateway HTTP mux is called
	// by Lambda to handle HTTP requests. The grpc-gateway mux then proxies the
	// requests to the gRPC server over the Unix socket. The gRPC server then
	// handles the requests as normal gRPC requests. The gRPC server then
	// returns the response to the grpc-gateway, which then returns the response
	// to the client as an HTTP response. This is all done transparently to the
	// user, and the user only interacts with the HTTP API.
	//
	// The gRPC network listener MUST be set to listen on a Unix socket to work
	// on Lambda, because Lambda explicitly prohibits inbound network
	// connections (even to localhost). Additionally, the sockets must be
	// created in the '/tmp' directory because the rest of the Lambda's
	// filesystem is readonly. That all being said, communication to servers
	// over Unix sockets work without issue on Lambda (somewhat surprisingly!).
	cfg := config.Config{
		GrpcNetwork: "unix",
		GrpcAddress: "/tmp/grpc.sock",
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

	// Start the gRPC server in the background.
	go func() {
		if err := app.ServeGrpc(ctx, grpcListener); err != nil {
			log.Errorf("error serving application server: %v", err)
		}
	}()

	// Start the Lambda handler.
	lambda.Start(httpadapter.New(app.Mux).ProxyWithContext)
	return nil
}

func closeListener(listener net.Listener) {
	if listener != nil {
		_ = listener.Close()
	}
}
