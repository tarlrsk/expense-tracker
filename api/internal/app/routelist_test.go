package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// defaultAPIDoc is docs/04-api.md in the spec repo, relative to this package directory.
const defaultAPIDoc = "../../../../expense-tracker-spec/docs/04-api.md"

// TestRoutesMatchAPIDoc fails unless the registered routes are exactly the
// rows of docs/04-api.md marked Built = yes (ADR-0032).
func TestRoutesMatchAPIDoc(t *testing.T) {
	path := defaultAPIDoc
	if dir := os.Getenv("EXPENSE_SPEC_DIR"); dir != "" {
		path = filepath.Join(dir, "docs", "04-api.md")
	}
	doc, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec // test reads the spec doc from a developer-set path
	if err != nil {
		t.Fatalf("read the API doc %s: %v (check out the spec repo next to the code repo, or set EXPENSE_SPEC_DIR)", path, err)
	}
	documented, err := parseBuiltEndpoints(string(doc))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	registered := map[string]bool{}
	for _, r := range newTestEngine(t).Routes() {
		registered[r.Method+" "+ginPathToDoc(r.Path)] = true
	}

	var missing, undocumented []string
	for ep := range documented {
		if !registered[ep] {
			missing = append(missing, ep)
		}
	}
	for ep := range registered {
		if !documented[ep] {
			undocumented = append(undocumented, ep)
		}
	}
	slices.Sort(missing)
	slices.Sort(undocumented)
	for _, ep := range undocumented {
		t.Errorf("registered but not marked Built = yes in %s: %s", path, ep)
	}
	for _, ep := range missing {
		t.Errorf("marked Built = yes in %s but not registered: %s", path, ep)
	}
}

var ginParam = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// ginPathToDoc turns gin's :name parameters into the doc's {name} form.
func ginPathToDoc(p string) string {
	return ginParam.ReplaceAllString(p, "{$1}")
}

var builtEndpoint = regexp.MustCompile("^`(GET|POST|PATCH|PUT|DELETE) (/api(?:/[A-Za-z0-9._{}-]+)+)`$")

// parseBuiltEndpoints returns the "METHOD /path" of every row marked Built = yes
// in the markdown table whose header has the columns Endpoint and Built.
func parseBuiltEndpoints(doc string) (map[string]bool, error) {
	lines := strings.Split(doc, "\n")
	tableAt := -1
	var endpointCol, builtCol int
	for i, line := range lines {
		cells, ok := tableCells(line)
		if !ok {
			continue
		}
		e, b := slices.Index(cells, "Endpoint"), slices.Index(cells, "Built")
		if e < 0 || b < 0 {
			continue
		}
		if tableAt >= 0 {
			return nil, errors.New("more than one table with Endpoint and Built columns")
		}
		tableAt, endpointCol, builtCol = i, e, b
	}
	if tableAt < 0 {
		return nil, errors.New("no table with Endpoint and Built columns")
	}

	built := map[string]bool{}
	// Skip the header and the |---| separator row.
	for i := tableAt + 2; i < len(lines); i++ {
		cells, ok := tableCells(lines[i])
		if !ok {
			break
		}
		if len(cells) <= max(endpointCol, builtCol) {
			return nil, fmt.Errorf("line %d: too few columns", i+1)
		}
		endpoint := cells[endpointCol]
		switch cells[builtCol] {
		case "no":
			continue
		case "yes":
		default:
			return nil, fmt.Errorf("line %d: Built must be yes or no, got %q", i+1, cells[builtCol])
		}
		m := builtEndpoint.FindStringSubmatch(endpoint)
		if m == nil {
			return nil, fmt.Errorf("line %d: a built row needs exactly one `METHOD /api/...` endpoint, got %q", i+1, endpoint)
		}
		key := m[1] + " " + m[2]
		if built[key] {
			return nil, fmt.Errorf("line %d: %s is listed twice", i+1, key)
		}
		built[key] = true
	}
	return built, nil
}

// tableCells splits a markdown table row into trimmed cells.
func tableCells(line string) ([]string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") {
		return nil, false
	}
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells, true
}

func TestParseBuiltEndpoints(t *testing.T) {
	const header = "| Endpoint | Module | Built | Purpose |\n|---|---|---|---|\n"
	tests := []struct {
		name    string
		doc     string
		want    []string
		wantErr string
	}{
		{
			name: "good table",
			doc: "# API\n\nIntro text.\n\n" + header +
				"| `GET /api/healthz` | — | yes | Liveness |\n" +
				"| `/api/auth/*` | M1 | no | Paths set later |\n" +
				"| `PATCH /api/categories/{id}` | M3 | yes | Rename |\n" +
				"| `GET /api/export.csv` | M8 | yes | Export |\n" +
				"\nAfter the table.\n| not | a | table | row |\n",
			want: []string{"GET /api/export.csv", "GET /api/healthz", "PATCH /api/categories/{id}"},
		},
		{
			name: "columns in another order",
			doc:  "| Built | Endpoint |\n|---|---|\n| yes | `DELETE /api/me` |\n",
			want: []string{"DELETE /api/me"},
		},
		{
			name: "nothing built",
			doc:  header + "| `GET /api/me` | M1 | no | Profile |\n",
			want: []string{},
		},
		{
			name:    "bad built value",
			doc:     header + "| `GET /api/me` | M1 | Yes | Profile |\n",
			wantErr: "Built must be yes or no",
		},
		{
			name:    "empty built value",
			doc:     header + "| `GET /api/me` | M1 |  | Profile |\n",
			wantErr: "Built must be yes or no",
		},
		{
			name:    "built row without a method",
			doc:     header + "| `/api/auth/*` | M1 | yes | Auth |\n",
			wantErr: "exactly one `METHOD /api/...`",
		},
		{
			name:    "built row with two endpoints",
			doc:     header + "| `GET /api/me`, `PATCH /api/me` | M1 | yes | Profile |\n",
			wantErr: "exactly one `METHOD /api/...`",
		},
		{
			name:    "built row outside /api",
			doc:     header + "| `GET /healthz` | — | yes | Liveness |\n",
			wantErr: "exactly one `METHOD /api/...`",
		},
		{
			name:    "built row with an unknown method",
			doc:     header + "| `HEAD /api/healthz` | — | yes | Liveness |\n",
			wantErr: "exactly one `METHOD /api/...`",
		},
		{
			name:    "built row not in backticks",
			doc:     header + "| GET /api/healthz | — | yes | Liveness |\n",
			wantErr: "exactly one `METHOD /api/...`",
		},
		{
			name:    "duplicate row",
			doc:     header + "| `GET /api/me` | M1 | yes | a |\n| `GET /api/me` | M1 | yes | b |\n",
			wantErr: "listed twice",
		},
		{
			name:    "missing table",
			doc:     "# API\n\n| Path | Purpose |\n|---|---|\n| `GET /api/me` | Profile |\n",
			wantErr: "no table with Endpoint and Built columns",
		},
		{
			name:    "two tables",
			doc:     header + "| `GET /api/me` | M1 | yes | a |\n\n" + header,
			wantErr: "more than one table",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBuiltEndpoints(tt.doc)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			keys := make([]string, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			if !slices.Equal(keys, tt.want) {
				t.Errorf("endpoints = %v, want %v", keys, tt.want)
			}
		})
	}
}

func TestGinPathToDoc(t *testing.T) {
	tests := []struct{ in, want string }{
		{in: "/api/healthz", want: "/api/healthz"},
		{in: "/api/categories/:id", want: "/api/categories/{id}"},
		{in: "/api/imports/:id/pages", want: "/api/imports/{id}/pages"},
		{in: "/api/admin/users/:user_id/set-password-link", want: "/api/admin/users/{user_id}/set-password-link"},
	}
	for _, tt := range tests {
		if got := ginPathToDoc(tt.in); got != tt.want {
			t.Errorf("ginPathToDoc(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
