package ping

import (
	"context"

	examplev1 "github.com/ccampo133/lambda-grpc-gateway/gen/go/example/v1"
)

// Service is a gRPC service implementation for the examplev1.PingService.
type Service struct {
	examplev1.UnimplementedPingServiceServer
}

// Service explicitly implements examplev1.PingServiceServer.
var _ examplev1.PingServiceServer = (*Service)(nil)

// NewPingService creates a new PingService instance.
func NewPingService() *Service {
	return &Service{}
}

// Ping is a unary rpc which returns a simple "Pong" message.
func (s Service) Ping(_ context.Context, _ *examplev1.PingRequest) (*examplev1.PingResponse, error) {
	return &examplev1.PingResponse{Message: "Pong"}, nil
}
