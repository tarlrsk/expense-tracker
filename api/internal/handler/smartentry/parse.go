// Package smartentry holds the gin handlers of the smart entry module, one file per endpoint.
package smartentry

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	smartentryparseorch "github.com/tarlrsk/expense-tracker/api/internal/smartentry/orchestrator/parse"
)

// parseRequest is the body of POST /api/entry/parse. Any other field is refused by DecodeJSON; a
// missing text is empty, and refused.
type parseRequest struct {
	Text string `json:"text"`
}

// parseResponse is the answer: the proposals in the order typed, and what the AI did
// (not_needed, used, limit_reached, unavailable, not_configured).
type parseResponse struct {
	Items []parseItem `json:"items"`
	AI    string      `json:"ai"`
}

// parseItem is one proposal. A field that could not be read is "" (category_id: null). amount is
// text with two decimals (ADR-0071); occurred_on is YYYY-MM-DD.
type parseItem struct {
	Text       string     `json:"text"`
	Amount     string     `json:"amount"`
	OccurredOn string     `json:"occurred_on"`
	Merchant   string     `json:"merchant"`
	CategoryID *uuid.UUID `json:"category_id"`
	Confidence string     `json:"confidence"`
	ResolvedBy string     `json:"resolved_by"`
}

// Parse handles POST /api/entry/parse (authed): 200 with the proposals; nothing is saved. The
// description style is the default until PLAN-0003 T7.
func Parse(o smartentryparseorch.Orchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		var req parseRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := o.Execute(c.Request.Context(), smartentryparseorch.Request{UserID: caller.UserID, Text: req.Text})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newParseResponse(resp))
	}
}

func newParseResponse(resp smartentryparseorch.Response) parseResponse {
	body := parseResponse{Items: make([]parseItem, 0, len(resp.Items)), AI: string(resp.AI)}
	for _, it := range resp.Items {
		item := parseItem{
			Text: it.Text, OccurredOn: it.OccurredOn.String(), Merchant: it.Merchant,
			Confidence: string(it.Confidence), ResolvedBy: string(it.ResolvedBy),
		}
		if it.HasAmount {
			item.Amount = it.Amount.String()
		}
		if it.CategoryID != uuid.Nil {
			id := it.CategoryID
			item.CategoryID = &id
		}
		body.Items = append(body.Items, item)
	}
	return body
}
