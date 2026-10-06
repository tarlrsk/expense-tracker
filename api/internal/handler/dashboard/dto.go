// Package dashboard holds the gin handlers of the dashboard module, one file per endpoint.
package dashboard

import (
	"github.com/google/uuid"

	dashboardoverviewproc "github.com/tarlrsk/expense-tracker/api/internal/dashboard/processor/overview"
)

// overviewResponse is the body of GET /api/dashboard (ADR-0088, ADR-0089). Amounts are text with
// exactly two decimals, so no client turns them into floats; months are YYYY-MM. categories and
// top_descriptions are [] (never null) for a month without spending; trend always has six rows,
// oldest first, ending with month. There are no transaction rows, owner ids or currency.
type overviewResponse struct {
	Month           string            `json:"month"`
	Totals          totalsItem        `json:"totals"`
	Categories      []categoryItem    `json:"categories"`
	Trend           []trendItem       `json:"trend"`
	TopDescriptions []descriptionItem `json:"top_descriptions"`
}

// totalsItem is the month's totals; difference is income minus spent and may be negative.
type totalsItem struct {
	Spent      string `json:"spent"`
	Income     string `json:"income"`
	Difference string `json:"difference"`
}

// categoryItem is one expense category's spending in the month.
type categoryItem struct {
	CategoryID uuid.UUID `json:"category_id"`
	Name       string    `json:"name"`
	Icon       string    `json:"icon"`
	Archived   bool      `json:"archived"`
	Total      string    `json:"total"`
	Count      int       `json:"count"`
}

// trendItem is one month of the trend.
type trendItem struct {
	Month string `json:"month"`
	Spent string `json:"spent"`
}

// descriptionItem is one row of the top descriptions.
type descriptionItem struct {
	Description string `json:"description"`
	Total       string `json:"total"`
	Count       int    `json:"count"`
}

func newOverviewResponse(r dashboardoverviewproc.Response) overviewResponse {
	body := overviewResponse{
		Month: r.Month.String(),
		Totals: totalsItem{
			Spent: r.Totals.Spent.String(), Income: r.Totals.Income.String(), Difference: r.Totals.Difference().String(),
		},
		Categories:      make([]categoryItem, 0, len(r.Categories)),
		Trend:           make([]trendItem, 0, len(r.Trend)),
		TopDescriptions: make([]descriptionItem, 0, len(r.TopDescriptions)),
	}
	for _, c := range r.Categories {
		body.Categories = append(body.Categories, categoryItem{
			CategoryID: c.CategoryID, Name: c.Name, Icon: c.Icon, Archived: c.Archived, Total: c.Total.String(), Count: c.Count,
		})
	}
	for _, m := range r.Trend {
		body.Trend = append(body.Trend, trendItem{Month: m.Month.String(), Spent: m.Spent.String()})
	}
	for _, d := range r.TopDescriptions {
		body.TopDescriptions = append(body.TopDescriptions, descriptionItem{
			Description: d.Description, Total: d.Total.String(), Count: d.Count,
		})
	}
	return body
}
