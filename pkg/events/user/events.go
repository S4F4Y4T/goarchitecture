// Package user holds the wire contract for domain events the user service
// publishes — shared between the producer (services/user) and any consumer
// (e.g. services/notification) the same way pkg/proto/user shares the gRPC
// contract between user and auth.
package user

// UserCreated is the routing key / event_type published after a user row is
// successfully created, regardless of whether creation happened via the
// HTTP API or the gRPC Create RPC (auth's registration path).
const UserCreated = "user.created"
