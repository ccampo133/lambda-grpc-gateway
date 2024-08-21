package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	examplev1 "github.com/ccampo133/lambda-grpc-gateway/gen/go/example/v1"
	"github.com/ccampo133/lambda-grpc-gateway/internal/config"
	"github.com/ccampo133/lambda-grpc-gateway/internal/ping"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
)

// ApplicationServer is the container struct representing the actual
// application. It stores the state of all runtime dependencies, such as
// protocol listeners, gRPC service implementations, etc. Once constructed, the
// server should be initialized with the Init method, and then run with one of
// Serve methods (Serve, ServeGrpc, ServeHttp).
type ApplicationServer struct {
	// Mux is the gRPC-Gateway multiplexer. It is exported so that it can be
	//passed to the AWS Lambda adapter (see the top level lambda/main.go).
	Mux *runtime.ServeMux

	cfg         config.Config
	grpcServer  *grpc.Server
	httpServer  *http.Server
	pingService examplev1.PingServiceServer

	initialized bool
}

// NewApplicationServer creates a new ApplicationServer instance with the given
// configuration.
func NewApplicationServer(cfg config.Config) *ApplicationServer {
	return &ApplicationServer{cfg: cfg}
}

// Init calls various initializer methods necessary for proper startup. Note
// that the order of the init method calls are here because there are no
// nil-checks in the initializers. Init works primarily via side effects. The
// global application state is largely stored in the ApplicationServer struct.
// This method serves to populate the runtime dependencies (fields) of the
// ApplicationServer instance. It should be called immediately before Run.
func (app *ApplicationServer) Init(ctx context.Context) error {
	if app.initialized {
		return errors.New("application is already initialized")
	}

	// Create the gRPC server and register all gRPC service implementations.
	app.grpcServer = grpc.NewServer(
		// Add the logging interceptor to the gRPC server.
		grpc.UnaryInterceptor(LogGRPCRequest),
	)
	// Register reflection service on gRPC server (for gRPCurl)
	reflection.Register(app.grpcServer)

	// Init and register gRPC service implementations
	examplev1.RegisterPingServiceServer(app.grpcServer, ping.NewPingService())

	// Create a new gRPC-Gateway mux.
	app.Mux = runtime.NewServeMux(
		// This is necessary to allow escaped slashes in path parameters. See:
		// https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/customizing_your_gateway/#controlling-path-parameter-unescaping
		runtime.WithUnescapingMode(
			runtime.UnescapingModeAllExceptReserved,
		),
	)

	// Register all gRPC -> HTTP gateway handlers.
	// Regarding endpoint naming, see:
	// https://github.com/grpc/grpc/blob/master/doc/naming.md#name-syntax
	endpoint := app.cfg.GrpcAddress
	if app.cfg.GrpcNetwork == "unix" {
		endpoint = "unix://" + app.cfg.GrpcAddress
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if err := examplev1.RegisterPingServiceHandlerFromEndpoint(ctx, app.Mux, endpoint, opts); err != nil {
		return fmt.Errorf("error registering ping service handler: %w", err)
	}

	// Create the HTTP server and set the handler to the gRPC-Gateway mux.
	app.httpServer = &http.Server{
		Handler: app.Mux,
		// gosec wants us to add a ReadHeaderTimeout here to avoid potential
		// Slowloris DoS attacks (see gosec rule G112). The reality is that most
		// "real" services will not be directly exposed to the internet in a
		// production scenario. Rather, they will only be accessible some
		// reverse proxy such as AWS API Gateway, etc., and the proxy will have
		// timeouts in place already. Nevertheless, we add a timeout here as a
		// best practice.
		ReadHeaderTimeout: 1 * time.Minute,
	}

	// Turn off gRPC logging. We don't want to see gRPC logs in the output.
	// Prompted by this issue: https://github.com/grpc-ecosystem/grpc-gateway/issues/4605
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))

	// Done!
	app.initialized = true
	return nil
}

// Serve brings up a gRPC and HTTP API server and continues until there's either
// a failure or the server is otherwise terminated or killed. Requires a prior
// call to Init.
func (app *ApplicationServer) Serve(ctx context.Context, grpcListener, httpListener net.Listener) error {
	if !app.initialized {
		return errors.New("application is not initialized")
	}
	// Serve gRPC and HTTP in separate goroutines.
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return app.ServeGrpc(ctx, grpcListener) })
	g.Go(func() error { return app.ServeHttp(ctx, httpListener) })
	// Block until the servers stop.
	log.Info("Application server started")
	return g.Wait()
}

// ServeHttp starts the HTTP server and blocks until it stops. This method is
// intended to be called in a separate goroutine if necessary. It is a wrapper
// around http.Server.Serve, but it also handles graceful shutdown. Requires a
// prior call to Init.
func (app *ApplicationServer) ServeHttp(ctx context.Context, lis net.Listener) error {
	if !app.initialized {
		return errors.New("application is not initialized")
	}
	return serveWithContext(
		ctx,
		func(context.Context) error {
			log.Infof("HTTP server listening at %s", lis.Addr())
			return app.httpServer.Serve(lis)
		},
		func(ctx context.Context) error {
			log.Info("Shutting down HTTP server")
			return app.httpServer.Shutdown(ctx)
		},
	)
}

// ServeGrpc starts the gRPC server and blocks until it stops. This method is
// intended to be called in a separate goroutine if necessary. It is a wrapper
// around grpc.Server.Serve, but it also handles graceful shutdown. Requires a
// prior call to Init.
func (app *ApplicationServer) ServeGrpc(ctx context.Context, lis net.Listener) error {
	return serveWithContext(
		ctx,
		func(context.Context) error {
			log.Infof("gRPC server listening at %s", lis.Addr())
			return app.grpcServer.Serve(lis)
		},
		func(context.Context) error {
			log.Info("Shutting down gRPC server")
			app.grpcServer.GracefulStop()
			return nil
		},
	)
}

func serveWithContext(ctx context.Context, serve, stop func(context.Context) error) error {
	errChan := make(chan error)
	go func() { errChan <- serve(ctx) }()
	select {
	case <-ctx.Done():
		return stop(ctx)
	case err := <-errChan:
		return err
	}
}
