package schema

import (
	"slices"
	"testing"
)

// Creating an account as app_auth seeds the default categories through the trigger
// (ADR-0036), although app_auth has no rights on categories.
func TestDefaultCategories(t *testing.T) {
	want := []struct{ name, kind string }{
		{"Food", "expense"},
		{"Groceries", "expense"},
		{"Transport", "expense"},
		{"Bills & Utilities", "expense"},
		{"Shopping", "expense"},
		{"Health", "expense"},
		{"Education", "expense"},
		{"Family support", "expense"},
		{"Donations / Tamboon", "expense"},
		{"Entertainment", "expense"},
		{"Other", "expense"},
		{"Salary", "income"},
		{"Other income", "income"},
	}

	s := begin(t)
	other := s.newUser(t)
	user := s.newUser(t)

	rows, err := s.tx.QueryContext(t.Context(), `
		select name, kind, icon, archived, sort_order, uuid_extract_version(id)
		from categories where owner_id = $1 order by id`, user)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []struct{ name, kind string }
	for rows.Next() {
		var name, kind, icon string
		var archived bool
		var sortOrder, version int
		if err := rows.Scan(&name, &kind, &icon, &archived, &sortOrder, &version); err != nil {
			t.Fatal(err)
		}
		if icon != "" || archived || version != 7 {
			t.Errorf("%s: icon %q, archived %v, uuid version %d; want empty icon, not archived, version 7", name, icon, archived, version)
		}
		if want := len(got) + 1; sortOrder != want {
			t.Errorf("%s: sort_order %d, want %d", name, sortOrder, want)
		}
		got = append(got, struct{ name, kind string }{name, kind})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("categories in id order:\n got %v\nwant %v", got, want)
	}

	// The other account got its own 13, and no profile was made by the trigger.
	if n := s.count(t, "select count(*) from categories where owner_id = $1", other); n != len(want) {
		t.Errorf("other user has %d categories, want %d", n, len(want))
	}
	s.asAuth(t)
	var noProfile string
	s.scan(t, "insert into users (email) values (gen_random_uuid()::text || '@example.test') returning id", nil, &noProfile)
	s.asOwner(t)
	if n := s.count(t, "select count(*) from profiles where id = $1", noProfile); n != 0 {
		t.Errorf("trigger created a profile")
	}
}
