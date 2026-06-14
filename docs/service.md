# Service Layer

## Purpose

The service layer is where **business logic** lives. It sits between the handler (HTTP) and the repository (database), orchestrating domain rules that go beyond simple CRUD.

```go
type UserService struct {
    repo model.UserRepository
}
```

The service depends on `model.UserRepository` (the interface), not on the concrete GORM implementation.

## What Business Logic Belongs Here

- **Pre-create uniqueness check**: before inserting a user, check if the email already exists and return a clear `CONFLICT` error. (The database constraint is still the ultimate enforcer, but checking first gives a better error message.)
- **Pre-update uniqueness check**: when updating a user's email, check if the new email belongs to a *different* user. If the email hasn't changed, no check is needed.
- **Fetching current state before update**: `UpdateUser` fetches the current user, compares emails, then calls the repository's update. This ensures the service has access to the current state without trusting the handler to provide it.

## What Doesn't Belong Here

- **HTTP concepts** (status codes, headers, request decoding) — handler's job
- **Database queries** — repository's job
- **Validation of request format** — handler's job (DTO + `validation.Validate()`)

## Pass-Through Methods

Some service methods are simple delegations to the repository:

```go
func (s *UserService) GetUserByID(ctx context.Context, id uint) (*model.User, error) {
    return s.repo.GetUserByID(ctx, id)
}
```

These exist because the service is the interface that the handler depends on. Even if a method has no logic today, having it on the service means:
- Adding logic later doesn't require changing the handler's dependency.
- Tests mock the service, not the repository — the handler's tests are isolated from DB details.

## Error Handling

The service returns `error` values. It may return `*apperror.AppError` directly (e.g., `apperror.Conflict("email already in use")`) or propagate errors from the repository. The handler's `response.Error()` call normalizes everything via `apperror.From()`.

The service **does not** know about HTTP status codes. It only returns domain errors.

## Context

All service methods accept `context.Context` and pass it to repository calls. This propagates cancellation signals and (future) tracing spans from the HTTP request context all the way to the database query.

## Pre-Check vs. DB Constraint: Why Both?

```go
// Service: pre-check for a clear error message
exists, _ := s.repo.ExistsByEmail(ctx, req.Email)
if exists {
    return nil, apperror.Conflict("email already in use")
}
// Repository: DB unique constraint as the ultimate enforcer
```

The pre-check gives a precise error message. The DB constraint is the safety net in case of a race condition (two concurrent requests both pass the pre-check before either inserts). The repository catches the unique violation and maps it to `CONFLICT` as well.

This pattern means the common case (non-concurrent, single user registration) gets a good error message. The rare race case still works correctly, just with a slightly less specific message ("unique constraint violation" rather than "email already in use").

## Alternatives Considered

- **No service layer — handler calls repository directly** — works for pure CRUD with no business logic. As soon as logic appears (pre-checks, orchestration across two repos), the handler becomes bloated and hard to test.
- **Rich domain model (DDD entities with methods)** — `user.ChangeEmail(newEmail)` on the model itself. More expressive for complex domains; overkill for simple CRUD.
- **Transactional service methods** — wrap multi-step operations (check + insert) in a database transaction. Would eliminate the race condition window on uniqueness checks. Not implemented yet; would be added when needed.
