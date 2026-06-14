# DTO (Data Transfer Objects)

## What DTOs Are

DTOs are structs that represent the **shape of HTTP request bodies**. They live in `internal/dto/` and are separate from domain models.

```go
// internal/dto/user.go
type CreateUserRequest struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type UpdateUserRequest struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}
```

## Why DTOs Are Separate from Domain Models

The domain model (`internal/model/User`) is what the service and repository work with. The DTO is what the client sends over the wire.

Keeping them separate means:
- **Validation tags belong on DTOs, not models.** The model is an internal concern; it shouldn't carry HTTP-layer annotations.
- **The API shape can differ from the storage shape.** A future `CreateUserRequest` might accept a `confirm_password` field that is never stored. A model might have computed fields (`full_name`) that shouldn't be sent by clients.
- **Models can evolve without breaking the API contract.** Adding an internal `deleted_at` field to the model doesn't change the DTO.

## Naming Convention

| DTO Name | HTTP Method | Purpose |
|---|---|---|
| `CreateUserRequest` | POST | Fields accepted when creating a resource |
| `UpdateUserRequest` | PUT | Fields accepted when replacing a resource |

No `Response` DTOs — handlers return domain models directly (via `response.Success()`). The model's `json:` tags control serialization. If the response shape ever needs to differ from the model, a dedicated response DTO can be added then.

## PUT Semantics: All Fields Required

`UpdateUserRequest` requires every field (`validate:"required"`). This reflects **full resource replacement**: the client sends the complete desired state, and the server overwrites all fields.

If a field is omitted, it's treated as invalid input — not as "don't change this field." This is PUT semantics as defined by HTTP (vs. PATCH, which would allow partial updates).

**Why not PATCH?**  
Partial updates via PATCH require distinguishing "field not sent" (don't change) from "field sent as null/empty" (explicitly clear). In Go, this requires pointer fields (`*string`) or a custom JSON decoder. For simple CRUD resources at this stage, full replacement is unambiguous and simpler to implement correctly.

## Alternatives Considered

- **Using the domain model directly** — annotate `User` with both `gorm:` and `validate:` tags. Works for simple cases, but mixes concerns. If the model changes for DB reasons, it affects API validation. Rejected.
- **PATCH with `map[string]interface{}`** — accept arbitrary key-value pairs for partial updates. Loses type safety and validation structure. Rejected.
- **PATCH with pointer fields** — `*string` for every optional field. Valid approach; more memory allocations and `nil` checks throughout. Deferred until PATCH semantics are required.
- **Protocol Buffers as DTOs** — strongly typed, language-agnostic. Requires protobuf toolchain. Only justified if consuming the API from multiple languages or using gRPC.
