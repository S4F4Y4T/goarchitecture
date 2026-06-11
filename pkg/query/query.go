// Package query parses list-endpoint sorting and filtering parameters into a
// safe, allowlisted form. It is deliberately free of any ORM dependency (like
// pkg/pagination): repositories translate the resolved Options into database
// clauses. Because columns are sourced exclusively from a per-resource Schema,
// the resulting clauses are safe to interpolate into ORDER BY / WHERE.
package query

import (
	"net/url"
	"regexp"
	"strings"
)

// filterKey matches query keys of the form filter[field].
var filterKey = regexp.MustCompile(`^filter\[(\w+)\]$`)

// Sort is a resolved ordering directive on an allowlisted column.
type Sort struct {
	Column string
	Desc   bool
}

// Filter is a resolved predicate on an allowlisted column. When Partial is
// true the repository should match case-insensitively (ILIKE %value%);
// otherwise an exact match is used.
type Filter struct {
	Column  string
	Value   string
	Partial bool
}

// Options is the resolved set of sorts and filters for a request.
type Options struct {
	Sorts   []Sort
	Filters []Filter
}

// FieldSpec describes how a single API field maps to the database and what
// operations are permitted on it.
type FieldSpec struct {
	Column     string // database column name
	Sortable   bool
	Filterable bool
	Partial    bool // string field: filter with ILIKE %value% instead of equality
}

// Schema maps API field names to their specs. Anything not present here is
// silently ignored when parsing, which keeps unknown or disallowed params from
// reaching the database.
type Schema map[string]FieldSpec

// Parse extracts ?sort= and ?filter[field]= parameters from values and
// resolves them against schema. Unknown fields, and fields not permitted for
// the requested operation, are dropped silently.
//
// sort accepts a comma-separated list of fields in priority order; a leading
// '-' requests descending order (e.g. ?sort=-price,name).
func Parse(values url.Values, schema Schema) Options {
	var opts Options

	for _, raw := range strings.Split(values.Get("sort"), ",") {
		field := strings.TrimSpace(raw)
		desc := false
		if strings.HasPrefix(field, "-") {
			desc = true
			field = field[1:]
		}
		if field == "" {
			continue
		}
		if spec, ok := schema[field]; ok && spec.Sortable {
			opts.Sorts = append(opts.Sorts, Sort{Column: spec.Column, Desc: desc})
		}
	}

	for key, vals := range values {
		m := filterKey.FindStringSubmatch(key)
		if m == nil || len(vals) == 0 {
			continue
		}
		value := vals[0]
		if value == "" {
			continue
		}
		if spec, ok := schema[m[1]]; ok && spec.Filterable {
			opts.Filters = append(opts.Filters, Filter{
				Column:  spec.Column,
				Value:   value,
				Partial: spec.Partial,
			})
		}
	}

	return opts
}
