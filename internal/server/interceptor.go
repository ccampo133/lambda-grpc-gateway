package server

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	log "github.com/sirupsen/logrus"
)

// LogGRPCRequest is a grpc.UnaryServerInterceptor which logs all gRPC requests
// at the debug level. If the request returned an error, it is also logged in a
// separate log message. If the error is an internal or unexpected error, it is
// logged at the error level, otherwise it is logged at the debug level.
func LogGRPCRequest(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	resp, err = handler(ctx, req)
	log.Debugf("rpc call: method = %s", info.FullMethod)
	if err != nil {
		lvl := log.ErrorLevel
		if s, ok := status.FromError(err); ok {
			if s.Code() != codes.Internal && s.Code() != codes.Unknown {
				// We don't want to log non-internal or non-unknown errors at
				// the error level because they are expected to occur.
				lvl = log.DebugLevel
			}
		}
		log.WithError(err).Logf(lvl, "rpc error: method = %s", info.FullMethod)
	}
	return resp, err
}
