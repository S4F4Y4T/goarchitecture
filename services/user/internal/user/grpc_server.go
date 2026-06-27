package user

import (
	"context"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	pb "github.com/s4f4y4t/go-microservice/pkg/proto/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	pb.UnimplementedUserServiceServer
	svc *UserService
}

func NewGRPCServer(svc *UserService) *GRPCServer {
	return &GRPCServer{svc: svc}
}

func (s *GRPCServer) ExistsByEmail(ctx context.Context, req *pb.ExistsByEmailRequest) (*pb.ExistsByEmailResponse, error) {
	exists, err := s.svc.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ExistsByEmailResponse{Exists: exists}, nil
}

func (s *GRPCServer) GetByEmail(ctx context.Context, req *pb.GetByEmailRequest) (*pb.UserResponse, error) {
	u, err := s.svc.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProto(u), nil
}

func (s *GRPCServer) Create(ctx context.Context, req *pb.CreateRequest) (*pb.UserResponse, error) {
	u, err := s.svc.CreateWithPassword(ctx, req.Name, req.Email, req.Password)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProto(u), nil
}

func toProto(u *User) *pb.UserResponse {
	return &pb.UserResponse{
		Id:        int32(u.ID),
		Name:      u.Name,
		Email:     u.Email,
		Password:  u.Password,
		CreatedAt: u.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: u.UpdatedAt.Format(time.RFC3339Nano),
	}
}

// toGRPCError preserves the AppError's semantic code (not found, conflict, ...)
// across the gRPC boundary so the calling service can react the same way it
// would to a local error instead of seeing everything as Internal.
func toGRPCError(err error) error {
	appErr := apperror.From(err)
	return status.Error(grpcCode(appErr.Code), appErr.Message)
}

func grpcCode(c apperror.Code) codes.Code {
	switch c {
	case apperror.CodeNotFound:
		return codes.NotFound
	case apperror.CodeConflict:
		return codes.AlreadyExists
	case apperror.CodeInvalidInput:
		return codes.InvalidArgument
	case apperror.CodeUnauthorized:
		return codes.Unauthenticated
	case apperror.CodeForbidden:
		return codes.PermissionDenied
	default:
		return codes.Internal
	}
}
