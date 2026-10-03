// Package categories holds the gin handlers of the categories module, one file per endpoint.
package categories

import (
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// categoryItem is one category in every response of /api/categories (ADR-0069). It has no
// owner_id: categories stay per user (docs/05-roadmap.md groups guardrails).
type categoryItem struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Kind      string    `json:"kind"`
	Archived  bool      `json:"archived"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newCategoryItem(c domain.Category) categoryItem {
	return categoryItem{
		ID: c.ID, Name: c.Name, Icon: c.Icon, Kind: string(c.Kind), Archived: c.Archived, SortOrder: c.SortOrder,
		CreatedAt: c.CreatedAt.UTC(), UpdatedAt: c.UpdatedAt.UTC(),
	}
}

// listResponse is the body of GET /api/categories and PUT /api/categories/order. The list is
// wrapped, so fields can be added later without changing the shape (ADR-0069).
type listResponse struct {
	Categories []categoryItem `json:"categories"`
}

func newListResponse(cats []domain.Category) listResponse {
	body := listResponse{Categories: make([]categoryItem, 0, len(cats))}
	for _, c := range cats {
		body.Categories = append(body.Categories, newCategoryItem(c))
	}
	return body
}
