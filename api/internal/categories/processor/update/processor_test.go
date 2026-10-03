package update

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	categoriesupdateport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/update"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	lockFn   func(ctx context.Context, ownerID uuid.UUID) error
	findFn   func(ctx context.Context, ownerID, id uuid.UUID) (domain.Category, bool, error)
	updateFn func(ctx context.Context, ownerID, id uuid.UUID, ch categoriesupdateport.Changes) (domain.Category, bool, error)
)

func (f lockFn) Lock(ctx context.Context, o uuid.UUID) error { return f(ctx, o) }
func (f findFn) Find(ctx context.Context, o, id uuid.UUID) (domain.Category, bool, error) {
	return f(ctx, o, id)
}
func (f updateFn) Update(ctx context.Context, o, id uuid.UUID, ch categoriesupdateport.Changes) (domain.Category, bool, error) {
	return f(ctx, o, id, ch)
}

func ptr[T any](v T) *T { return &v }

// written is a comparable view of Changes.
type written struct {
	name, icon string
	archived   string // "", "true" or "false"
	toEnd      bool
}

func view(ch categoriesupdateport.Changes) written {
	var w written
	if ch.Name != nil {
		w.name = "=" + *ch.Name
	}
	if ch.Icon != nil {
		w.icon = "=" + *ch.Icon
	}
	if ch.Archived != nil {
		w.archived = map[bool]string{true: "true", false: "false"}[*ch.Archived]
	}
	w.toEnd = ch.ToEnd
	return w
}

func TestExecute(t *testing.T) {
	user, id := uuid.New(), uuid.New()
	active := domain.Category{ID: id, Name: "Food", Icon: "🍜", Kind: domain.KindExpense, SortOrder: 3}
	archived := active
	archived.Archived = true
	tests := []struct {
		name      string
		req       Request
		cur       domain.Category
		missing   bool
		updateErr error
		want      *written // nil: Update must not be called
		wantKind  apperr.Kind
		wantCalls []string
	}{
		{
			name: "rename, normalised", req: Request{Name: ptr("  Eating \t out ")}, cur: active,
			want: &written{name: "=Eating out"}, wantCalls: []string{"lock", "find", "update"},
		},
		{
			name: "rename to its own name in another case", req: Request{Name: ptr("FOOD")}, cur: active,
			want: &written{name: "=FOOD"}, wantCalls: []string{"lock", "find", "update"},
		},
		{
			name: "rename to the same name writes nothing", req: Request{Name: ptr(" Food ")}, cur: active,
			wantCalls: []string{"lock", "find"},
		},
		{
			name: "clear the icon", req: Request{Icon: ptr("  ")}, cur: active,
			want: &written{icon: "="}, wantCalls: []string{"lock", "find", "update"},
		},
		{
			name: "archive keeps the place", req: Request{Archived: ptr(true)}, cur: active,
			want: &written{archived: "true"}, wantCalls: []string{"lock", "find", "update"},
		},
		{
			name: "unarchive goes to the end", req: Request{Archived: ptr(false)}, cur: archived,
			want: &written{archived: "false", toEnd: true}, wantCalls: []string{"lock", "find", "update"},
		},
		{
			name: "archive an archived one writes nothing", req: Request{Archived: ptr(true)}, cur: archived,
			wantCalls: []string{"lock", "find"},
		},
		{
			name: "unarchive an active one writes nothing", req: Request{Archived: ptr(false)}, cur: active,
			wantCalls: []string{"lock", "find"},
		},
		{
			name: "rename and unarchive", req: Request{Name: ptr("Meals"), Archived: ptr(false)}, cur: archived,
			want: &written{name: "=Meals", archived: "false", toEnd: true}, wantCalls: []string{"lock", "find", "update"},
		},
		{
			name: "taken name: conflict", req: Request{Name: ptr("Groceries")}, cur: active,
			updateErr: categoriesupdateport.ErrNameTaken,
			want:      &written{name: "=Groceries"}, wantKind: apperr.Conflict, wantCalls: []string{"lock", "find", "update"},
		},
		{
			name: "unknown id: not found", req: Request{Name: ptr("Meals")}, missing: true,
			wantKind: apperr.NotFound, wantCalls: []string{"lock", "find"},
		},
		{name: "no field: no transaction", req: Request{}, wantKind: apperr.InvalidInput},
		{name: "empty name: no transaction", req: Request{Name: ptr("\t")}, wantKind: apperr.InvalidInput},
		{name: "long icon: no transaction", req: Request{Icon: ptr("123456789012345678901234567890123")}, wantKind: apperr.InvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			var calls []string
			call := func(ctx context.Context, name string, owner uuid.UUID) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user {
					t.Errorf("%s outside the caller's user transaction: %v", name, err)
				}
				if owner != user {
					t.Errorf("%s for %s, want the caller", name, owner)
				}
				calls = append(calls, name)
			}
			after := tt.cur
			after.Name = "after"
			p := New(utx, Ports{
				Lock: lockFn(func(ctx context.Context, o uuid.UUID) error { call(ctx, "lock", o); return nil }),
				Find: findFn(func(ctx context.Context, o, got uuid.UUID) (domain.Category, bool, error) {
					call(ctx, "find", o)
					if got != id {
						t.Errorf("find %s, want %s", got, id)
					}
					return tt.cur, !tt.missing, nil
				}),
				Update: updateFn(func(ctx context.Context, o, got uuid.UUID, ch categoriesupdateport.Changes) (domain.Category, bool, error) {
					call(ctx, "update", o)
					if tt.want == nil {
						t.Errorf("update called with %+v", view(ch))
					} else if view(ch) != *tt.want {
						t.Errorf("changes = %+v, want %+v", view(ch), *tt.want)
					}
					return after, true, tt.updateErr
				}),
			})
			tt.req.UserID, tt.req.ID = user, id
			resp, err := p.Execute(t.Context(), tt.req)
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
			if err != nil {
				return
			}
			want := tt.cur
			if tt.want != nil {
				want = after
			}
			if resp.Category != want {
				t.Errorf("category = %+v, want %+v", resp.Category, want)
			}
		})
	}
}
