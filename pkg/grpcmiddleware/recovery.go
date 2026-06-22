package grpcmiddleware

import (
	"context"
	"runtime/debug"

	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery turns a panic in an RPC handler into an Internal error instead of
// crashing the process, mirroring pkgmiddleware.PanicRecovery on the HTTP side.
func Recovery(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.FromContext(ctx).Error("panic recovered",
				"method", info.FullMethod,
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			err = status.Error(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}
