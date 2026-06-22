package grpcmiddleware

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// Timeout returns a unary client interceptor that bounds every outbound RPC
// to d, so a hung or slow peer fails the caller's request instead of
// blocking it indefinitely. It only tightens the deadline — if ctx already
// carries an earlier deadline, that one still wins.
func Timeout(d time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
