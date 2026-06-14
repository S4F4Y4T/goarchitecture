# Response Format

## Envelope Structure

Every API response is wrapped in a consistent envelope:

```json
{
  "success": true,
  "status_code": 200,
  "message": "Users retrieved successfully",
  "data": [...],
  "meta": { "page": 1, "limit": 10, "total": 42, "total_pages": 5 }
}
```

For errors:
```json
{
  "success": false,
  "status_code": 400,
  "error": {
    "code": "INVALID_INPUT",
    "message": "Validation failed",
    "fields": {
      "email": "email must be a valid email address"
    }
  }
}
```

## `pkg/response` Functions

| Function | Status | Use case |
|---|---|---|
| `Success(w, status, message, data)` | 200/201 | Single resource or list response |
| `SuccessWithMeta(w, status, message, data, meta)` | 200 | List with pagination metadata |
| `NoContent(w)` | 204 | DELETE — no body |
| `Error(w, r, err)` | 4xx/5xx | All errors, normalizes via `apperror.From()` |

## Why a Response Envelope?

**Consistency for clients**: the client always knows where to find data (`.data`), always knows if the call succeeded (`.success`), and always finds errors in the same place (`.error`). Clients don't have to handle wildly different shapes per endpoint.

**Machine-readable errors**: `.error.code` is a stable string (`"NOT_FOUND"`, `"CONFLICT"`) that client code can switch on. `.error.message` is human-readable. Both are present.

**`status_code` in body**: HTTP status codes are occasionally stripped or transformed by proxies, CDNs, and load balancers. Repeating the status in the body ensures the client always knows the intended status, even if the transport layer modifies it.

## Why `meta` Only on List Endpoints?

`meta` carries pagination data (`total`, `total_pages`). It is meaningless for single-resource endpoints. Omitting it (JSON `omitempty`) keeps single-resource responses clean.

## HTTP Semantics

- `POST` → 201 Created
- `GET` → 200 OK
- `PUT` → 200 OK (returns updated resource)
- `DELETE` → 204 No Content (empty body, `NoContent()`)

DELETE returns 204 rather than 200 with a body because there is nothing useful to return — the resource is gone.

## Content-Type

All responses set `Content-Type: application/json`. This is done once in `JSONResponse()`, not repeated in each handler.

## Alternatives Considered

- **No envelope — return raw data/error** — simpler responses, but every client must inspect the HTTP status code directly. Harder to add metadata (pagination) without breaking existing clients.
- **JSON:API spec** — a formal specification for JSON API responses. More expressive, but significant added complexity (relationship links, included resources). Overkill for internal microservices.
- **HAL / HATEOAS** — hypermedia links in responses. Useful for public, discoverable APIs. Not needed here.
- **Problem+JSON (RFC 7807)** — standardized error format. Compatible with our structure; could migrate to this for errors if consuming a standard parser becomes important.
