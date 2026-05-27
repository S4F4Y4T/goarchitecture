package pagination

const (
	defaultPage  = 1
	defaultLimit = 10
	maxLimit     = 100
)

type Params struct {
	Page  int
	Limit int
}

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewParams(page, limit int) Params {
	if page < 1 {
		page = defaultPage
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return Params{Page: page, Limit: limit}
}

func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

func NewMeta(p Params, total int64) Meta {
	totalPages := 0
	if p.Limit > 0 {
		totalPages = int((total + int64(p.Limit) - 1) / int64(p.Limit))
	}
	return Meta{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
