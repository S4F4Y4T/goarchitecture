package repository

import (
	"microservice/pkg/query"

	"gorm.io/gorm"
)

// applyFilters returns a scope that applies the resolved filters to a query.
// Columns originate from a query.Schema allowlist, so interpolating them into
// the clause is safe. It is applied to both the count and the fetch so totals
// stay consistent with the returned page.
func applyFilters(opts query.Options) func(*gorm.DB) *gorm.DB {
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

// applySorts returns a scope that applies the resolved sort order to a query.
// Only used on the fetch, not the count.
func applySorts(opts query.Options) func(*gorm.DB) *gorm.DB {
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
