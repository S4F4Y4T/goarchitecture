package grpcmiddleware

import (
	"context"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Logger is the gRPC equivalent of pkgmiddleware.Logger on the HTTP side: one
// structured line per RPC with method, resulting status code, and duration.
// Register it ahead of Recovery in the interceptor chain so it still logs a
// clean "Internal" outcome (rather than nothing) when a handler panics.
func Logger(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	logger.FromContext(ctx).Info("grpc request completed",
		"method", info.FullMethod,
		"code", status.Code(err).String(),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return resp, err
}
