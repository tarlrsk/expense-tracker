// Package archtest checks the import rules of ADR-0032 on an internal/ tree, and who may use the
// auth transaction (ADR-0034). It parses source files only (no type checking) and is used by
// tests.
package archtest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Modules in dependency order: a module may import only modules before it.
var Modules = []string{"account", "categories", "transactions", "categorization", "entry", "imports", "dashboard", "budgets"}

// HealthModule is outside the order: it imports no module and no module imports it.
const HealthModule = "health"

// NonModules are the known packages under internal/ that are not modules.
var NonModules = []string{"app", "registry", "handler", "external", "tx", "db", "apperr", "middleware", "config", "archtest", "authz"}

// Violation is one broken rule.
type Violation struct {
	Rule int    // rule number, 1 to 10
	File string // path relative to the internal/ directory, slash-separated
	Msg  string
}

func (v Violation) String() string {
	return fmt.Sprintf("rule %d: %s: %s", v.Rule, v.File, v.Msg)
}

type file struct {
	rel     string // file path relative to internal/
	dir     string // package directory relative to internal/
	test    bool
	ast     *ast.File
	imports []string // internal imports, relative to internal/
}

type constructor struct {
	dir  string // adaptor package directory relative to internal/
	name string
}

// Check walks internalDir (the internal/ directory of the Go module modulePath)
// and returns every violation of the ADR-0032 rules. Directories named testdata
// are skipped.
func Check(modulePath, internalDir string) ([]Violation, error) {
	prefix := modulePath + "/internal/"
	var vs []Violation

	vs = append(vs, checkTopLevel(internalDir)...)

	var files []file
	fset := token.NewFileSet()
	err := filepath.WalkDir(internalDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != internalDir && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(internalDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		fi := file{rel: rel, dir: path.Dir(rel), test: strings.HasSuffix(d.Name(), "_test.go"), ast: f}
		for _, imp := range f.Imports {
			ip, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: import %s: %w", rel, imp.Path.Value, err)
			}
			if t, ok := strings.CutPrefix(ip, prefix); ok {
				fi.imports = append(fi.imports, t)
			}
		}
		files = append(files, fi)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", internalDir, err)
	}

	for _, f := range files {
		if f.dir == "." {
			vs = append(vs, Violation{3, f.rel, "file directly under internal/; put it in a package"})
		}
		for _, target := range f.imports {
			if target == f.dir {
				continue // external test package importing its own package
			}
			vs = append(vs, checkImport(f, target)...)
		}
	}

	vs = append(vs, checkConstructors(files, prefix)...)
	vs = append(vs, checkAuthUse(files, prefix)...)
	return vs, nil
}

func checkTopLevel(internalDir string) []Violation {
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		return []Violation{{3, ".", fmt.Sprintf("read internal/: %v", err)}}
	}
	var vs []Violation
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !isModule(e.Name()) && !slices.Contains(NonModules, e.Name()) {
			vs = append(vs, Violation{3, e.Name(), "unknown package, add it to the architecture test"})
		}
	}
	return vs
}

func checkImport(f file, target string) []Violation {
	src, dst := strings.Split(f.dir, "/"), strings.Split(target, "/")
	srcTop, dstTop := src[0], dst[0]
	var vs []Violation
	add := func(rule int, format string, args ...any) {
		vs = append(vs, Violation{rule, f.rel, fmt.Sprintf(format, args...)})
	}

	// Rule 1: only <module>/port/..., app and db itself import db.
	mayImportDB := srcTop == "db" || srcTop == "app" || (isModule(srcTop) && layer(src) == "port")
	if dstTop == "db" && !mayImportDB {
		add(1, "imports internal/%s; only <module>/port/..., app and db may", target)
	}

	// Rule 2: module order; health is outside it.
	switch {
	case srcTop == HealthModule && isModule(dstTop) && dstTop != HealthModule:
		add(2, "health imports module %s; health imports no module", dstTop)
	case dstTop == HealthModule && isModule(srcTop) && srcTop != HealthModule:
		add(2, "module %s imports health; no module imports health", srcTop)
	case isOrdered(srcTop) && isOrdered(dstTop) && order(dstTop) > order(srcTop):
		add(2, "module %s imports later module %s", srcTop, dstTop)
	}

	// Rule 4: processors import no processor and no outside service.
	if isModule(srcTop) && layer(src) == "processor" {
		if isModule(dstTop) && layer(dst) == "processor" {
			add(4, "processor imports processor %s", target)
		}
		if dstTop == "external" {
			add(4, "processor imports outside service %s", target)
		}
	}

	// Rule 5: orchestrators import no database port.
	if isModule(srcTop) && layer(src) == "orchestrator" && isModule(dstTop) && layer(dst) == "port" {
		add(5, "orchestrator imports database port %s", target)
	}

	// Rule 6: business and base packages import no HTTP or wiring package.
	if (isModule(srcTop) || slices.Contains([]string{"external", "tx", "apperr", "config", "authz"}, srcTop)) &&
		slices.Contains([]string{"handler", "registry", "app", "middleware"}, dstTop) {
		add(6, "imports internal/%s; %s must not depend on handler, registry, app or middleware", target, srcTop)
	}

	// Rule 7: the registry root never imports registry/<module>.
	if f.dir == "registry" && dstTop == "registry" && len(dst) > 1 {
		add(7, "registry imports internal/%s; app calls each module's Register", target)
	}
	return vs
}

// checkConstructors applies rule 8: adaptor constructors are called only from registry.
func checkConstructors(files []file, prefix string) []Violation {
	ctors := map[constructor]bool{}
	for _, f := range files {
		base := path.Base(f.rel)
		if f.test || !strings.HasPrefix(base, "adaptor_") || !isAdaptorDir(f.dir) {
			continue
		}
		for _, decl := range f.ast.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.IsExported() && strings.HasPrefix(fn.Name.Name, "New") {
				ctors[constructor{f.dir, fn.Name.Name}] = true
			}
		}
	}
	if len(ctors) == 0 {
		return nil
	}

	var vs []Violation
	for _, f := range files {
		names := importNames(f.ast, prefix)
		if len(names) == 0 {
			continue
		}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			dir, ok := names[id.Name]
			if !ok || !ctors[constructor{dir, sel.Sel.Name}] {
				return true
			}
			if strings.HasPrefix(f.dir, "registry/") || (f.test && f.dir == dir) {
				return true
			}
			vs = append(vs, Violation{8, f.rel, fmt.Sprintf("calls adaptor constructor %s.%s; only registry may", dir, sel.Sel.Name)})
			return true
		})
	}
	return vs
}

// checkAuthUse applies rules 9 and 10 (ADR-0032, ADR-0034), on test files too:
//   - rule 9: only packages under account/port/ (and db itself) refer to db.AuthConn;
//   - rule 10: only account/..., app, db, tx and registry/account refer to tx.Auth or to any
//     identifier named WithAuthTx (a call, a method or an interface method).
//
// Without type checking, WithAuthTx is matched by name whatever its receiver.
func checkAuthUse(files []file, prefix string) []Violation {
	var vs []Violation
	for _, f := range files {
		names := importNames(f.ast, prefix)
		mayAuthConn := within(f.dir, "db") || within(f.dir, "account/port")
		mayAuthTx := within(f.dir, "account") || within(f.dir, "app") || within(f.dir, "db") ||
			within(f.dir, "tx") || within(f.dir, "registry/account")
		ast.Inspect(f.ast, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				id, ok := n.X.(*ast.Ident)
				if !ok {
					return true
				}
				switch dir := names[id.Name]; {
				case dir == "db" && n.Sel.Name == "AuthConn" && !mayAuthConn:
					vs = append(vs, Violation{9, f.rel, "uses db.AuthConn; only account/port/... may"})
				case dir == "tx" && n.Sel.Name == "Auth" && !mayAuthTx:
					vs = append(vs, Violation{10, f.rel, "uses tx.Auth; only account, app, db, tx and registry/account may"})
				}
			case *ast.Ident:
				if n.Name == "WithAuthTx" && !mayAuthTx {
					vs = append(vs, Violation{10, f.rel, "uses WithAuthTx; only account, app, db, tx and registry/account may"})
				}
			}
			return true
		})
	}
	return vs
}

// within reports whether dir is pkg or a package below it.
func within(dir, pkg string) bool { return dir == pkg || strings.HasPrefix(dir, pkg+"/") }

// importNames maps each local import name of f to the internal package
// directory it refers to. Without an alias the name is the last path element.
func importNames(f *ast.File, prefix string) map[string]string {
	names := map[string]string{}
	for _, imp := range f.Imports {
		ip, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		dir, ok := strings.CutPrefix(ip, prefix)
		if !ok {
			continue
		}
		name := path.Base(dir)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		names[name] = dir
	}
	return names
}

func isModule(name string) bool { return name == HealthModule || slices.Contains(Modules, name) }

func isOrdered(name string) bool { return slices.Contains(Modules, name) }

func order(name string) int { return slices.Index(Modules, name) }

// layer returns the layer folder of a module package path (processor, port, ...).
func layer(parts []string) string {
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func isAdaptorDir(dir string) bool {
	parts := strings.Split(dir, "/")
	return parts[0] == "external" || (isModule(parts[0]) && layer(parts) == "port")
}
