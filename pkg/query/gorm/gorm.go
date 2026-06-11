// Package gorm bridges pkg/query with GORM by turning resolved Options into
// reusable scope functions. pkg/query itself stays ORM-free and importable by
// any layer that doesn't touch a database.
package gorm

import (
	"microservice/pkg/query"

	"gorm.io/gorm"
)

// Filters returns a GORM scope that applies every resolved filter from opts.
// Columns originate from a query.Schema allowlist, so interpolating them into
// the clause is safe. Apply to both the count and the fetch so totals stay
// consistent with the returned page.
func Filters(opts query.Options) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		for _, f := range opts.Filters {
			if f.Partial {
				db = db.Where(f.Column+" ILIKE ?", "%"+f.Value+"%")
			} else {
				db = db.Where(f.Column+" = ?", f.Value)
			}
		}
		return db
	}
}

// Sorts returns a GORM scope that applies the resolved sort order from opts.
// Apply only on the fetch query, not the count.
func Sorts(opts query.Options) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		for _, s := range opts.Sorts {
			direction := "ASC"
			if s.Desc {
				direction = "DESC"
			}
			db = db.Order(s.Column + " " + direction)
		}
		return db
	}
}
