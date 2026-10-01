package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// newTestEngine wires the middleware in the same order as app.NewEngine.
func newTestEngine(logger *slog.Logger, timeout time.Duration) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(RequestLog(logger), Recover(logger), NoStore(), Deadline(timeout))
	e.NoRoute(func(c *gin.Context) { httpx.WriteError(c, apperr.New(apperr.NotFound, "not found")) })
	e.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	e.GET("/conflict", func(c *gin.Context) { httpx.WriteError(c, apperr.New(apperr.Conflict, "taken")) })
	e.GET("/slow", func(c *gin.Context) {
		ctx := c.Request.Context()
		<-ctx.Done()
		httpx.WriteError(c, ctx.Err())
	})
	e.GET("/panic", func(*gin.Context) { panic("boom") })
	e.GET("/items/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return e
}

func serve(t *testing.T, e *gin.Engine, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, target, nil))
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) httpx.ErrorDetail {
	t.Helper()
	var body httpx.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body.Error
}

func TestNoStore(t *testing.T) {
	e := newTestEngine(slog.New(slog.DiscardHandler), time.Second)
	tests := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{name: "success", target: "/ok", wantStatus: http.StatusOK},
		{name: "error", target: "/conflict", wantStatus: http.StatusConflict},
		{name: "unknown route", target: "/nope", wantStatus: http.StatusNotFound},
		{name: "panic", target: "/panic", wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, e, http.MethodGet, tt.target)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
			}
		})
	}
}

func TestDeadline(t *testing.T) {
	e := newTestEngine(slog.New(slog.DiscardHandler), 20*time.Millisecond)
	rec := serve(t, e, http.MethodGet, "/slow")
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", rec.Code)
	}
	if got := decodeError(t, rec); got.Code != "timeout" || got.Message != httpx.TimeoutMessage {
		t.Errorf("error = %+v, want code timeout", got)
	}
}

func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	e := newTestEngine(slog.New(slog.NewJSONHandler(&buf, nil)), time.Second)
	rec := serve(t, e, http.MethodGet, "/panic")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if got := decodeError(t, rec); got.Code != "internal" || got.Message != httpx.InternalMessage {
		t.Errorf("error = %+v, want code internal with the generic message", got)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("panic value leaked to the client: %s", rec.Body.String())
	}
	logs := buf.String()
	if !strings.Contains(logs, `"msg":"panic"`) || !strings.Contains(logs, `"stack":"goroutine`) {
		t.Errorf("panic not logged with a stack:\n%s", logs)
	}
}

func TestRequestLog(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		wantLevel string
		wantRoute string
		wantPath  string
		wantCode  int
		wantError string
	}{
		{name: "success with query", target: "/items/42?email=a@example.com", wantLevel: "INFO", wantRoute: "/items/:id", wantPath: "/items/42", wantCode: 204},
		{name: "client error", target: "/conflict", wantLevel: "INFO", wantRoute: "/conflict", wantPath: "/conflict", wantCode: 409, wantError: "conflict: taken"},
		{name: "unknown route", target: "/nope?token=secret", wantLevel: "INFO", wantRoute: "", wantPath: "/nope", wantCode: 404, wantError: "not_found: not found"},
		{name: "panic", target: "/panic", wantLevel: "ERROR", wantRoute: "/panic", wantPath: "/panic", wantCode: 500, wantError: "internal: panic: boom"},
	}
	hexID := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			e := newTestEngine(slog.New(slog.NewJSONHandler(&buf, nil)), time.Second)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.target, nil)
			req.Header.Set("X-Request-Id", "client-chosen")
			req.Header.Set("Authorization", "Bearer secret-token")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			line := lastRequestLine(t, buf.String())
			if strings.Contains(line, "?") || strings.Contains(line, "secret") || strings.Contains(line, "Bearer") {
				t.Errorf("log line leaks query or headers: %s", line)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(line), &got); err != nil {
				t.Fatalf("decode log line %q: %v", line, err)
			}
			id, _ := got["request_id"].(string)
			if !hexID.MatchString(id) {
				t.Errorf("request_id = %q, want 32 hex chars", id)
			}
			if rec.Header().Get("X-Request-Id") != id {
				t.Errorf("X-Request-Id header = %q, want %q", rec.Header().Get("X-Request-Id"), id)
			}
			want := map[string]any{
				"level":  tt.wantLevel,
				"method": "GET",
				"route":  tt.wantRoute,
				"path":   tt.wantPath,
				"status": float64(tt.wantCode),
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("%s = %v, want %v", k, got[k], v)
				}
			}
			if _, ok := got["duration_ms"].(float64); !ok {
				t.Errorf("duration_ms missing or not a number: %v", got["duration_ms"])
			}
			gotErr, hasErr := got["error"]
			switch {
			case tt.wantError == "" && hasErr:
				t.Errorf("unexpected error field %v", gotErr)
			case tt.wantError != "" && gotErr != tt.wantError:
				t.Errorf("error = %v, want %q", gotErr, tt.wantError)
			}
		})
	}
}

func lastRequestLine(t *testing.T, logs string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], `"msg":"request"`) {
			return lines[i]
		}
	}
	t.Fatalf("no request log line in:\n%s", logs)
	return ""
}
