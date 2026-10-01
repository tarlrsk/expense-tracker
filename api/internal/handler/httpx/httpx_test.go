package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

func TestWriteError(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	secret := errors.New("password=hunter2 at db.internal")
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "invalid input", err: apperr.New(apperr.InvalidInput, "amount must be positive"), wantStatus: 400, wantCode: "invalid_input", wantMessage: "amount must be positive"},
		{name: "unauthenticated", err: apperr.New(apperr.Unauthenticated, "log in first"), wantStatus: 401, wantCode: "unauthenticated", wantMessage: "log in first"},
		{name: "forbidden", err: apperr.New(apperr.Forbidden, "operators only"), wantStatus: 403, wantCode: "forbidden", wantMessage: "operators only"},
		{name: "not found", err: apperr.New(apperr.NotFound, "no such category"), wantStatus: 404, wantCode: "not_found", wantMessage: "no such category"},
		{name: "conflict", err: apperr.New(apperr.Conflict, "name taken"), wantStatus: 409, wantCode: "conflict", wantMessage: "name taken"},
		{name: "rate limited", err: apperr.New(apperr.RateLimited, "try later"), wantStatus: 429, wantCode: "rate_limited", wantMessage: "try later"},
		{name: "timeout", err: apperr.Wrap(apperr.Timeout, "custom text is replaced", secret), wantStatus: 504, wantCode: "timeout", wantMessage: TimeoutMessage},
		{name: "internal hides cause and message", err: apperr.Wrap(apperr.Internal, "load failed", secret), wantStatus: 500, wantCode: "internal", wantMessage: InternalMessage},
		{name: "wrapped apperr", err: fmt.Errorf("outer: %w", apperr.New(apperr.NotFound, "no such transaction")), wantStatus: 404, wantCode: "not_found", wantMessage: "no such transaction"},
		{name: "empty message", err: apperr.New(apperr.Conflict, ""), wantStatus: 409, wantCode: "conflict", wantMessage: "Conflict"},
		{name: "plain error", err: secret, wantStatus: 500, wantCode: "internal", wantMessage: InternalMessage},
		{name: "deadline exceeded", err: fmt.Errorf("query: %w", context.DeadlineExceeded), wantStatus: 504, wantCode: "timeout", wantMessage: TimeoutMessage},
		{name: "nil error", err: nil, wantStatus: 500, wantCode: "internal", wantMessage: InternalMessage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)

			WriteError(c, tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body ErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body.String(), err)
			}
			if body.Error.Code != tt.wantCode || body.Error.Message != tt.wantMessage {
				t.Errorf("body = %+v, want code %q message %q", body.Error, tt.wantCode, tt.wantMessage)
			}
			if !c.IsAborted() {
				t.Error("context not aborted")
			}
			if len(c.Errors) != 1 {
				t.Errorf("recorded errors = %d, want 1", len(c.Errors))
			}
		})
	}
}

func TestWriteErrorAfterWrite(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	c.String(http.StatusOK, "partial")

	WriteError(c, errors.New("late failure"))

	if rec.Code != http.StatusOK || rec.Body.String() != "partial" {
		t.Errorf("response changed after write: %d %q", rec.Code, rec.Body.String())
	}
	if !c.IsAborted() || len(c.Errors) != 1 {
		t.Errorf("aborted = %v, errors = %d; want true, 1", c.IsAborted(), len(c.Errors))
	}
}
