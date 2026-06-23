package grpcmiddleware

import (
	"context"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Validation rejects a request before it reaches the handler if the message
// violates its protovalidate (buf.validate) rules — the gRPC equivalent of
// pkg/validation on the HTTP side. Rules are read from the message's own
// descriptor, so any proto.Message is covered with no generated code needed.
func Validation(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if msg, ok := req.(proto.Message); ok {
		if err := protovalidate.Validate(msg); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	return handler(ctx, req)
}
