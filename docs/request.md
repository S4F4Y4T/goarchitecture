# Request Decoding

## What `pkg/request` Does

`DecodeJSON(w, r, dst)` is the single function all handlers use to read the request body:

1. **Limits body size** to 1 MiB (`MaxBodyBytes = 1 << 20`) — prevents memory exhaustion from large payloads.
2. **Rejects unknown fields** (`DisallowUnknownFields()`) — returns `400 INVALID_INPUT` if the client sends a field not in the struct. Prevents typos from silently being ignored.
3. **Rejects trailing data** — only a single JSON value is accepted; `{"name":"x"}{"name":"y"}` is an error.
4. **Returns `*apperror.AppError`** — consistent with the rest of the error system; handlers just forward to `response.Error()`.

```go
var req dto.CreateUserRequest
if err := request.DecodeJSON(w, r, &req); err != nil {
    response.Error(w, r, err)
    return
}
```

## Why 1 MiB Body Limit?

Without a limit, a client can send a multi-gigabyte body and exhaust server memory. `http.MaxBytesReader` wraps the body and returns an error if the limit is exceeded. 1 MiB is generous for JSON API payloads (typical requests are <10 KB) while preventing abuse.

## Why Reject Unknown Fields?

Two reasons:
- **Client-side typo protection**: `{"emial": "x@y.com"}` would silently succeed without this check, and the client would wonder why their update didn't take effect.
- **Security**: unknown fields can indicate an attempt to inject parameters (mass-assignment attacks in some frameworks). Rejecting them is a safe default.

The downside: strict clients must not send extra fields. In practice, API clients should only send documented fields, so this is not a real constraint.

## Alternatives Considered

- **`json.Decoder` without `DisallowUnknownFields`** — silently ignores unknown fields. Easier for clients but hides bugs. Rejected as the default.
- **`json.Unmarshal(body, &dst)`** — reads the entire body into memory before parsing. Combined with no size limit, this is a DoS vector. `json.NewDecoder` with `MaxBytesReader` is safer.
- **Custom parser / `gjson`** — more flexibility for partial updates (PATCH). Not needed since we use full-replacement PUT semantics.
- **`encoding/json/v2`** — stricter semantics, but still experimental at time of writing. Will reconsider when stable.
