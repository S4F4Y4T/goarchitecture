# Pagination

## Query Parameters

```
GET /v1/users?page=2&limit=25
```

| Param | Default | Min | Max | Behavior |
|---|---|---|---|---|
| `page` | 1 | 1 | — | Page number (1-indexed) |
| `limit` | 10 | 1 | 100 | Items per page |

Out-of-range values are **clamped**, not rejected:
- `page=0` → 1
- `limit=0` → 1
- `limit=500` → 100

## `pkg/pagination` API

```go
params := pagination.NewParams(page, limit)  // clamps values
offset := params.Offset()                    // (page-1) * limit

meta := pagination.NewMeta(params, total)    // calculates total_pages
```

`NewMeta` calculates:
```go
TotalPages = ceil(total / limit)
```

The result is included in the response envelope:
```json
{
  "meta": {
    "page": 2,
    "limit": 25,
    "total": 87,
    "total_pages": 4
  }
}
```

## Why Clamping Instead of Rejecting Invalid Values?

Clamping is more lenient and reduces client friction. A client that sends `limit=0` (maybe a bug) gets a valid response with limit=1 rather than an error that stops it from working at all. The response `meta` tells the client what limit was actually applied, so there's no ambiguity.

If strict validation were preferred, returning 400 for `limit=0` would also be reasonable — it would make bugs more visible. We chose leniency as the default behavior.

## Why 1-Indexed Pages?

Page 1 is more natural for users and UIs than page 0. SQL `OFFSET` is 0-indexed internally, but `Offset()` handles the conversion: `(page - 1) * limit`.

## Why Max Limit 100?

Prevents a client from requesting thousands of rows in a single query, which would:
- Exhaust server memory for large result sets
- Cause long response times
- Hammer the database

100 is a sensible default; services with different requirements can set their own max in a future `NewParams` variant.

## Total Count Query

List endpoints run **two queries**:
1. `COUNT(*)` with filters applied — to compute `total` and `total_pages`
2. `SELECT *` with filters, sorts, and limit/offset applied — to fetch the actual rows

This is intentional: the count query ignores limit/offset and returns the total matching rows, which the client needs to render a page selector.

**Tradeoff**: two DB round-trips per list request. For most tables this is fast. For very large tables, a `COUNT(*)` over all rows is expensive; we can add estimated counts or cursor-based pagination later.

## Alternatives Considered

- **Cursor-based pagination** — uses an opaque cursor (e.g., the last seen `id`) instead of page/offset. More efficient for large tables (no `OFFSET` scan), stable across concurrent inserts. More complex client-side. Deferred for when offset pagination becomes a bottleneck.
- **Keyset pagination** — similar to cursor-based. Same tradeoff.
- **GraphQL `first/after`** — relay-style cursor pagination. Only relevant if adopting GraphQL.
- **`X-Total-Count` header** — some APIs put the total count in a header instead of the body. Breaks the envelope consistency. Rejected.
