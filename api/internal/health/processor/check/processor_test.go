package check

import (
	"context"
	"errors"
	"testing"
)

type fakePing struct{ err error }

func (f fakePing) Ping(context.Context) error { return f.err }

func TestExecute(t *testing.T) {
	failure := errors.New("not ready")
	tests := []struct {
		name    string
		pingErr error
		wantErr error
	}{
		{name: "ok", pingErr: nil, wantErr: nil},
		{name: "ping fails", pingErr: failure, wantErr: failure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(fakePing{err: tt.pingErr}).Execute(context.Background(), Request{})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Execute() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && err != nil {
				t.Errorf("Execute() error = %v, want nil", err)
			}
		})
	}
}
