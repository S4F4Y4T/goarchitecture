package grpcmiddleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validator is satisfied by any protoc-gen-validate-generated message.
type validator interface {
	Validate() error
}

// Validation rejects a request before it reaches the handler if the message
// implements the protoc-gen-validate Validate() method and reports a
// violation — the gRPC equivalent of pkg/validation on the HTTP side.
func Validation(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if v, ok := req.(validator); ok {
		if err := v.Validate(); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	return handler(ctx, req)
}
