package archtest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestRealTree checks the API's own internal/ tree.
func TestRealTree(t *testing.T) {
	modulePath := readModulePath(t, filepath.Join("..", "..", "go.mod"))
	internalDir := ".."
	if _, err := os.Stat(filepath.Join(internalDir, "health", "processor", "check")); err != nil {
		t.Fatalf("not run from api/internal/archtest: %v", err)
	}
	vs, err := Check(modulePath, internalDir)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, v := range vs {
		t.Error(v)
	}
}

// TestFixtures runs the checker on small trees under testdata/, at least one per rule.
func TestFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		want    []string // "rule N file"
	}{
		{fixture: "clean", want: nil},
		{fixture: "rule1", want: []string{
			"1 categories/processor/create/processor.go",
			"1 handler/categories/create.go",
			"1 tx/tx_test.go",
		}},
		{fixture: "rule2", want: []string{
			"2 account/processor/login/processor.go",
			"2 categories/port/list/port.go",
			"2 categories/port/list/port_test.go",
			"2 health/processor/check/processor.go",
		}},
		{fixture: "rule3", want: []string{
			"3 stray.go",
			"3 util",
		}},
		{fixture: "rule4", want: []string{
			"4 categories/processor/create/processor.go",
			"4 categories/processor/create/processor.go",
			"4 transactions/processor/update/processor.go",
		}},
		{fixture: "rule5", want: []string{
			"5 transactions/orchestrator/importrow/orchestrator.go",
			"5 transactions/orchestrator/importrow/orchestrator_test.go",
		}},
		{fixture: "rule6", want: []string{
			"6 categories/domain/category.go",
			"6 config/config.go",
			"6 external/mail/send/port.go",
			"6 health/port/ping/port.go",
			"6 tx/tx.go",
		}},
		{fixture: "rule7", want: []string{
			"7 registry/deps.go",
		}},
		{fixture: "rule8", want: []string{
			"8 app/app.go",
			"8 categories/processor/create/processor_test.go",
			"8 handler/mail/send.go",
		}},
		{fixture: "rule9", want: []string{
			"9 app/app.go",
			"9 categories/port/insert/adaptor_pg.go",
			"9 categories/port/insert/adaptor_pg_test.go",
		}},
		{fixture: "rule10", want: []string{
			"10 categories/port/list/port.go",
			"10 categories/processor/create/processor.go",
			"10 middleware/auth.go",
			"10 registry/categories/create.go",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			vs, err := Check("example.com/fx", filepath.Join("testdata", tt.fixture, "internal"))
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			got := make([]string, 0, len(vs))
			for _, v := range vs {
				got = append(got, fmt.Sprintf("%d %s", v.Rule, v.File))
			}
			slices.Sort(got)
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("violations:\n got  %q\n want %q\ndetails:", got, want)
				for _, v := range vs {
					t.Log(v)
				}
			}
		})
	}
}

func readModulePath(t *testing.T, goMod string) string {
	t.Helper()
	f, err := os.Open(filepath.Clean(goMod))
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(s.Text()), "module "); ok {
			return strings.TrimSpace(mod)
		}
	}
	t.Fatalf("no module line in %s", goMod)
	return ""
}
