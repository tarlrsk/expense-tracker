package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	deps := registry.Deps{
		Config: config.Config{Addr: "127.0.0.1:0", RequestTimeout: 5 * time.Second, LogLevel: slog.LevelInfo},
		Logger: slog.New(slog.DiscardHandler),
	}
	engine, err := NewEngine(deps)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return engine
}

func TestEngine(t *testing.T) {
	engine := newTestEngine(t)
	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantBody   string // exact body for success responses
		wantCode   string // error code for error responses
	}{
		{name: "healthz", method: http.MethodGet, target: "/api/healthz", wantStatus: http.StatusOK, wantBody: `{"status":"ok"}`},
		{name: "unknown path", method: http.MethodGet, target: "/api/nope", wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "wrong method", method: http.MethodPost, target: "/api/healthz", wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "outside /api", method: http.MethodGet, target: "/healthz", wantStatus: http.StatusNotFound, wantCode: "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), tt.method, tt.target, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "private, no-store")
			}
			if rec.Header().Get("X-Request-Id") == "" {
				t.Error("X-Request-Id header missing")
			}
			if tt.wantCode == "" {
				if rec.Body.String() != tt.wantBody {
					t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
				}
				return
			}
			var body httpx.ErrorBody
			dec := json.NewDecoder(rec.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if body.Error.Code != tt.wantCode || body.Error.Message == "" {
				t.Errorf("error = %+v, want code %q and a message", body.Error, tt.wantCode)
			}
		})
	}
}
