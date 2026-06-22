package user

import (
	"context"
	"time"

	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	pb "github.com/s4f4y4t/go-microservice/pkg/proto/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client satisfies auth.UserLookup over gRPC instead of a direct DB
// repository, now that auth and user are separate services.
type Client struct {
	client pb.UserServiceClient
}

func NewClient(client pb.UserServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	resp, err := c.client.ExistsByEmail(ctx, &pb.ExistsByEmailRequest{Email: email})
	if err != nil {
		return false, fromGRPCError(err)
	}
	return resp.Exists, nil
}

func (c *Client) GetByEmail(ctx context.Context, email string) (*User, error) {
	resp, err := c.client.GetByEmail(ctx, &pb.GetByEmailRequest{Email: email})
	if err != nil {
		return nil, fromGRPCError(err)
	}
	return fromProto(resp), nil
}

func (c *Client) Create(ctx context.Context, u *User) (*User, error) {
	resp, err := c.client.Create(ctx, &pb.CreateRequest{
		Name:     u.Name,
		Email:    u.Email,
		Password: u.Password,
	})
	if err != nil {
		return nil, fromGRPCError(err)
	}
	return fromProto(resp), nil
}

func fromProto(r *pb.UserResponse) *User {
	u := &User{
		ID:       int(r.Id),
		Name:     r.Name,
		Email:    r.Email,
		Password: r.Password,
	}
	if t, err := time.Parse(time.RFC3339Nano, r.CreatedAt); err == nil {
		u.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, r.UpdatedAt); err == nil {
		u.UpdatedAt = t
	}
	return u
}

// fromGRPCError reverses toGRPCError on the user service's side, so auth
// sees the same AppError semantics (not found, conflict, ...) it would have
// gotten from a local repository call.
func fromGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return apperror.Internal(err)
	}
	switch st.Code() {
	case codes.NotFound:
		return apperror.NotFound(st.Message())
	case codes.AlreadyExists:
		return apperror.Conflict(st.Message())
	case codes.InvalidArgument:
		return apperror.InvalidInput(st.Message())
	case codes.Unauthenticated:
		return apperror.Unauthorized(st.Message())
	case codes.PermissionDenied:
		return apperror.Forbidden(st.Message())
	default:
		return apperror.Internal(err)
	}
}
