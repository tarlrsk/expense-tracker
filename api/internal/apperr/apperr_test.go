package apperr

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestKindOf(t *testing.T) {
	cause := errors.New("db down")
	tests := []struct {
		name string
		err  error
		want Kind
	}{
		{name: "nil", err: nil, want: ""},
		{name: "invalid input", err: New(InvalidInput, "bad"), want: InvalidInput},
		{name: "unauthenticated", err: New(Unauthenticated, "who"), want: Unauthenticated},
		{name: "forbidden", err: New(Forbidden, "no"), want: Forbidden},
		{name: "not found", err: New(NotFound, "gone"), want: NotFound},
		{name: "conflict", err: New(Conflict, "dup"), want: Conflict},
		{name: "rate limited", err: New(RateLimited, "slow down"), want: RateLimited},
		{name: "timeout", err: New(Timeout, "late"), want: Timeout},
		{name: "internal", err: Wrap(Internal, "oops", cause), want: Internal},
		{name: "wrapped by fmt", err: fmt.Errorf("outer: %w", New(NotFound, "gone")), want: NotFound},
		{name: "plain error", err: cause, want: Internal},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: Timeout},
		{name: "wrapped deadline", err: fmt.Errorf("query: %w", context.DeadlineExceeded), want: Timeout},
		{name: "canceled", err: context.Canceled, want: Internal},
		{name: "unknown kind", err: New(Kind("weird"), "x"), want: Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindOf(tt.err); got != tt.want {
				t.Errorf("KindOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestError(t *testing.T) {
	cause := errors.New("db down")
	tests := []struct {
		name      string
		err       error
		wantText  string
		wantCause error
	}{
		{name: "new", err: New(NotFound, "category not found"), wantText: "not_found: category not found"},
		{name: "wrap", err: Wrap(Internal, "load failed", cause), wantText: "internal: load failed: db down", wantCause: cause},
		{name: "no message", err: Wrap(Internal, "", cause), wantText: "internal: db down", wantCause: cause},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantText {
				t.Errorf("Error() = %q, want %q", got, tt.wantText)
			}
			if tt.wantCause != nil && !errors.Is(tt.err, tt.wantCause) {
				t.Errorf("errors.Is(err, cause) = false, want true")
			}
		})
	}
}
