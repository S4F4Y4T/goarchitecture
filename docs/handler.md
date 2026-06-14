# Handler

## Purpose

Handlers translate HTTP requests into service calls and HTTP responses. They are the only layer that knows about HTTP.

```go
type UserHandler struct {
    service *service.UserService
}
```

## Handler Lifecycle Pattern

Every handler follows the same steps:

```
1. Extract path params (r.PathValue("id"))
2. Decode request body (request.DecodeJSON)
3. Validate DTO (validation.Validate)
4. Call service
5. Respond (response.Success / response.Error)
```

Error at any step → `response.Error(w, r, err)` → returns. Never falls through.

## Example: CreateUser

```go
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req dto.CreateUserRequest
    if err := request.DecodeJSON(w, r, &req); err != nil {
        response.Error(w, r, err)
        return
    }
    if err := validation.Validate(&req); err != nil {
        response.Error(w, r, err)
        return
    }
    user, err := h.service.CreateUser(r.Context(), &model.User{
        Name: req.Name, Email: req.Email,
    })
    if err != nil {
        response.Error(w, r, err)
        return
    }
    response.Success(w, http.StatusCreated, "User created successfully", user)
}
```

## List Handler: Query Parsing

`GetAllUsers` also parses pagination and query options from URL params:

```go
page  := parseIntParam(r, "page", 1)
limit := parseIntParam(r, "limit", 10)
params := pagination.NewParams(page, limit)

opts := query.Parse(r.URL.Query(), model.UserListSchema)

users, total, err := h.service.GetAllUsers(r.Context(), params, opts)
// ...
meta := pagination.NewMeta(params, total)
response.SuccessWithMeta(w, http.StatusOK, "Users retrieved", users, meta)
```

## Logging in Handlers

Handlers log significant actions (create, update, delete) at INFO level using the request-scoped logger:

```go
log := logger.FromContext(r.Context())
log.Info("creating user", "email", req.Email)
```

Read operations (GET) are not logged by the handler — the access log from the Logger middleware is sufficient.

## What Handlers Don't Do

- **Business logic**: never check uniqueness, never load related resources. That's the service's job.
- **Database access**: never hold or use `*gorm.DB`. Only the repository touches the DB.
- **Error mapping**: never write `if err.Code == "23505"`. Errors flow to `response.Error()` which handles mapping.

## HTTP Method Semantics

| Method | Handler | Success Code | Body |
|---|---|---|---|
| GET | GetAll, GetByID | 200 | resource(s) |
| POST | Create | 201 | created resource |
| PUT | Update | 200 | updated resource |
| DELETE | Delete | 204 | empty |

## Path Parameter Parsing

```go
idStr := r.PathValue("id")
id, err := strconv.ParseUint(idStr, 10, 64)
if err != nil {
    response.Error(w, r, apperror.InvalidInput("invalid user ID"))
    return
}
```

`r.PathValue()` is the Go 1.22 standard library API for reading path parameters from `ServeMux` patterns like `/users/{id}`.

## Alternatives Considered

- **Handler as a function (not a method)** — `CreateUser(svc service.UserService) http.HandlerFunc`. Closures instead of struct methods. Slightly more functional; same logic. Methods on a struct are more readable when there are many endpoints.
- **Combining decode + validate** — one function that decodes and validates in one call. Convenient but loses the ability to log the decoded (but invalid) request for debugging. Kept separate.
- **Returning errors from handlers** — some patterns return `(interface{}, error)` from handler-like functions. Requires a wrapper that calls `response.Error`. More indirection for the same result. Standard `http.HandlerFunc` signature kept.
