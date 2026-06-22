package grpcmiddleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/s4f4y4t/go-microservice/pkg/logger"
	"github.com/s4f4y4t/go-microservice/pkg/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// metadataRequestIDKey is the gRPC metadata key carrying the request ID
// between services — the wire equivalent of pkgmiddleware.RequestIDHeader.
const metadataRequestIDKey = "x-request-id"

// RequestID is the server-side half: it reuses an incoming "x-request-id"
// metadata entry (propagated by PropagateRequestID on the calling service)
// or mints a new UUID, then populates both the plain ID (via
// middleware.WithRequestID) and a request-scoped logger (via
// logger.WithContext) on the context — so Logger and Recovery, which already
// read through logger.FromContext, start carrying request_id for free. Must
// run outermost in the interceptor chain.
func RequestID(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	id := requestIDFromIncoming(ctx)
	if id == "" {
		id = uuid.NewString()
	}

	ctx = middleware.WithRequestID(ctx, id)
	ctx = logger.WithContext(ctx, logger.L().With("request_id", id))

	return handler(ctx, req)
}

func requestIDFromIncoming(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get(metadataRequestIDKey)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// PropagateRequestID is the client-side half: it reads the request ID
// already in ctx (set by the HTTP RequestID middleware further up the call
// chain) and forwards it as outgoing gRPC metadata, so the request ID
// started at the edge survives the hop into the next service's logs.
func PropagateRequestID(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if id := middleware.GetRequestID(ctx); id != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, metadataRequestIDKey, id)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}
