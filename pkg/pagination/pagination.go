// Package pagination holds the query parsing + metadata used by list endpoints.
package pagination

import (
	"math"
	"strconv"
	"strings"
)

const (
	defaultPage    = 1
	defaultPerPage = 15
	maxPerPage     = 100
)

// Params is the normalised, already-sanitised form of a list request.
type Params struct {
	Page    int
	PerPage int
	Search  string
	SortBy  string
	SortDir string
}

// Meta is what the client receives alongside the data array.
type Meta struct {
	Page     int   `json:"page"`
	PerPage  int   `json:"per_page"`
	Total    int64 `json:"total"`
	LastPage int   `json:"last_page"`
}

func (p Params) Offset() int { return (p.Page - 1) * p.PerPage }

// OrderClause returns a safe `column direction` string. Callers pass the
// whitelist of sortable columns — anything outside it falls back to the
// first entry, so user input never reaches the SQL builder verbatim.
func (p Params) OrderClause(allowed []string, fallback string) string {
	column := fallback
	for _, a := range allowed {
		if strings.EqualFold(a, p.SortBy) {
			column = a
			break
		}
	}
	dir := "desc"
	if strings.EqualFold(p.SortDir, "asc") {
		dir = "asc"
	}
	return column + " " + dir
}

func NewMeta(p Params, total int64) Meta {
	last := max(int(math.Ceil(float64(total)/float64(p.PerPage))), 1)
	return Meta{Page: p.Page, PerPage: p.PerPage, Total: total, LastPage: last}
}

// FromQuery builds Params from raw query-string values, clamping everything
// into a sane range. getter is typically c.Query.
func FromQuery(getter func(string) string) Params {
	page, _ := strconv.Atoi(getter("page"))
	if page < 1 {
		page = defaultPage
	}

	perPage, _ := strconv.Atoi(getter("per_page"))
	switch {
	case perPage < 1:
		perPage = defaultPerPage
	case perPage > maxPerPage:
		perPage = maxPerPage
	}

	return Params{
		Page:    page,
		PerPage: perPage,
		Search:  strings.TrimSpace(getter("search")),
		SortBy:  getter("sort_by"),
		SortDir: getter("sort_dir"),
	}
}
