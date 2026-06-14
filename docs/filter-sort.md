# Filter & Sort

## Query Parameter Format

```
GET /v1/users?sort=-name,id&filter[name]=alice&filter[email]=example.com
```

**Sort**: comma-separated field names. A leading `-` means descending.
```
?sort=name        → ORDER BY name ASC
?sort=-name       → ORDER BY name DESC
?sort=-name,id    → ORDER BY name DESC, id ASC
```

**Filter**: bracket notation per field.
```
?filter[name]=alice    → WHERE name ILIKE '%alice%'  (partial match)
?filter[id]=5          → WHERE id = 5                (exact match)
```

## Allowlist Schema

Each model defines a `Schema` — a map of API field name → column spec:

```go
var UserListSchema = query.Schema{
    "id":         {Column: "id",         Sortable: true,  Filterable: true,  Partial: false},
    "name":       {Column: "name",       Sortable: true,  Filterable: true,  Partial: true},
    "email":      {Column: "email",      Sortable: true,  Filterable: true,  Partial: true},
    "created_at": {Column: "created_at", Sortable: true,  Filterable: false, Partial: false},
}
```

`query.Parse(r.URL.Query(), schema)` reads the URL params and returns `[]Sort` and `[]Filter` — with unknown or disallowed fields **silently dropped**. A client that sends `?filter[password]=x` gets an empty filter, not an error.

## Why Allowlisting?

SQL injection prevention. Sort column names and filter column names are interpolated directly into SQL strings (GORM `Order()`, raw `WHERE` clauses). Using an allowlist means only known-safe column names ever reach the query.

```go
// safe: "name" comes from the allowlist, not from raw user input
db.Order("name DESC")
db.Where("name ILIKE ?", "%alice%")
```

Without an allowlist, a sort parameter of `id; DROP TABLE users` could be catastrophic.

## Partial vs. Exact Match

- `Partial: true` → ILIKE `%value%` (case-insensitive substring match). Used for string fields where users expect to search by partial name/email.
- `Partial: false` → exact match (`=`). Used for IDs, prices, and other fields where substring search makes no sense.

## GORM Bridge (`pkg/query/gorm`)

`pkg/query` is ORM-agnostic — it just parses URL params and returns Go structs. `pkg/query/gorm` bridges to GORM scopes:

```go
db.Scopes(
    querygorm.Filters(opts),
    querygorm.Sorts(opts),
).Find(&users)
```

This separation means if we ever replace GORM with `pgx` or `sqlc`, only the bridge package changes. `pkg/query` itself is unaffected.

## Silent Drop vs. Error on Unknown Fields

Unknown or disallowed sort/filter fields are silently ignored (not rejected with a 400). The reasoning:

- Clients often evolve ahead of or behind the API. A client that sends `?sort=phone` against a service that doesn't have a `phone` field should still get a valid (unsorted) response, not an error.
- For security, silently ignoring is strictly safer than exposing "which fields exist" via error messages.

**Tradeoff**: client-side typos in field names are harder to detect. A developer who sends `?sort=creted_at` gets an unsorted response with no indication that the field name was wrong. Acceptable for now; debug logging can be added if this becomes a pain point.

## Alternatives Considered

- **OData `$filter` / `$orderby`** — a full query language over HTTP. Very expressive but complex to parse safely. Overkill for CRUD APIs.
- **GraphQL** — queries define exactly what fields to filter and sort. Would replace REST entirely. Deferred.
- **Prisma-style `where` JSON** — send filter criteria as a JSON body on GET requests. Non-standard HTTP (GET with a body is uncommon). Rejected.
- **Unrestricted column names in sort** — letting users pass any column name directly to `ORDER BY`. Never acceptable; SQL injection risk.
