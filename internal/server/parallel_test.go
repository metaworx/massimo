// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The server's tests run in parallel (AP TestSpeed, Block 7, D11-A):
// every test calls t.Parallel() before anything else, but those on
// serialTests, which Go runs one after the other before the parallel
// ones begin; a test that swaps a package variable says so through
// swapping. docs/testing.md, "Parallel tests".

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// serialTests are the tests that do not run in parallel, and why: each
// changes something every test of the package shares, binds an address
// a test beside it could bind too, or checks a time that tests beside
// it would stretch.
var serialTests = map[string]string{
	// A package variable swapped while the test runs.
	"TestFullName":               "swaps lookupCNAME",
	"TestGetSystemInterfaces":    "swaps hostInterfaces",
	"TestProviderIssuerAtSaving": "swaps oidcClient (fakeDiscovery)",
	"TestProvidersAPI":           "swaps oidcClient (fakeDiscovery)",
	"TestRebootBlocked":          "swaps rebootStuck",
	"TestWallReset":              "sets wallSilence to 0",

	// A fixed port: a Cube added through the API as the only one gets
	// the proxy on the Cube's port, :62910, with the discovery port
	// :23272 where the server has no address of its own for it.
	"TestConcurrentAddCube":          "listens on :62910 and :23272",
	"TestDeleteAndAddCube":           "listens on :62910 and :23272",
	"TestRemovedCubeIsListedAsFound": "listens on :62910",

	// A time the test checks as the product's, which the load of the
	// tests beside it would stretch.
	"TestBackupAutomatic":                   "the backup at backups.auto_max within 5 s",
	"TestStepObserverDoesNotWaitOnTheStore": "the observer's three calls within 500 ms",
}

// swapping marks the test as one that swaps a package variable, such
// as a hook, until its end: every swap in the package's tests, or the
// helper that makes it, calls it first. It sets an environment variable
// through t.Setenv, which panics in a parallel test, and after which
// t.Parallel panics: a swap in a parallel test fails at once, where it
// would otherwise race with the tests beside it.
func swapping(t *testing.T) {
	t.Helper()
	t.Setenv("MASSIMO_TEST_SWAP", "1")
}

// Every test of the package calls t.Parallel() as its first statement,
// or is on serialTests and does not call it; serialTests names only
// tests of the package, each with its reason; and Parallel() is called
// only in a test's own body or a subtest's function literal, never in a
// helper, where it would make a test parallel that the list holds
// serial.
func TestParallelOrListed(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*_test.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("the package's test files: %v %v", names, err)
	}
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range strayParallel(fset, f) {
			t.Errorf("%s: Parallel() outside a test's body and a subtest's function literal", at)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || !isTestFunc(fn) {
				continue
			}
			test, at := fn.Name.Name, fset.Position(fn.Pos())
			seen[test] = true
			param := fn.Type.Params.List[0].Names[0].Name
			reason, listed := serialTests[test]
			switch {
			case listed && callsParallel(fn.Body, param):
				t.Errorf("%s: %s is on serialTests (%s) and calls %s.Parallel()", at, test, reason, param)
			case !listed && !parallelFirst(fn.Body, param):
				t.Errorf("%s: %s neither calls %s.Parallel() first nor is on serialTests with its reason", at, test, param)
			}
		}
	}
	for test, reason := range serialTests {
		switch {
		case !seen[test]:
			t.Errorf("serialTests names %s, which is no test of the package", test)
		case strings.TrimSpace(reason) == "":
			t.Errorf("serialTests names %s without a reason", test)
		}
	}
}

// isTestFunc reports whether fn is a test go test runs: TestXxx, Xxx
// not starting with a lower-case letter, of one *testing.T.
func isTestFunc(fn *ast.FuncDecl) bool {
	name, ok := strings.CutPrefix(fn.Name.Name, "Test")
	if fn.Recv != nil || !ok || fn.Body == nil {
		return false
	}
	if r, _ := utf8.DecodeRuneInString(name); unicode.IsLower(r) {
		return false
	}
	ps := fn.Type.Params.List
	if len(ps) != 1 || len(ps[0].Names) != 1 {
		return false
	}
	star, ok := ps[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "testing" && sel.Sel.Name == "T"
}

// parallelFirst reports whether body's first statement is
// param.Parallel().
func parallelFirst(body *ast.BlockStmt, param string) bool {
	if len(body.List) == 0 {
		return false
	}
	s, ok := body.List[0].(*ast.ExprStmt)
	return ok && isParallelCall(s.X, param)
}

// callsParallel reports whether body calls param.Parallel() anywhere,
// leaving out the function literals that take a parameter of the same
// name, such as a subtest's.
func callsParallel(body *ast.BlockStmt, param string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			for _, f := range n.Type.Params.List {
				for _, id := range f.Names {
					if id.Name == param {
						return false
					}
				}
			}
		case *ast.CallExpr:
			if isParallelCall(n, param) {
				found = true
			}
		}
		return !found
	})
	return found
}

// strayParallel returns where f calls Parallel() in a function that is
// neither a test nor a subtest's function literal (the second argument
// of a Run).
func strayParallel(fset *token.FileSet, f *ast.File) []token.Position {
	subtests := map[*ast.FuncLit]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && len(call.Args) == 2 {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Run" {
				if lit, ok := call.Args[1].(*ast.FuncLit); ok {
					subtests[lit] = true
				}
			}
		}
		return true
	})
	var stray []token.Position
	var stack []ast.Node // the nodes from the file down to the one inspected
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if _, ok := parallelOf(n); !ok {
			return true
		}
		for i := len(stack) - 1; i >= 0; i-- {
			switch fn := stack[i].(type) {
			case *ast.FuncLit:
				if !subtests[fn] {
					stray = append(stray, fset.Position(n.Pos()))
				}
				return true
			case *ast.FuncDecl:
				if !isTestFunc(fn) {
					stray = append(stray, fset.Position(n.Pos()))
				}
				return true
			}
		}
		return true
	})
	return stray
}

// isParallelCall reports whether e is param.Parallel().
func isParallelCall(e ast.Expr, param string) bool {
	x, ok := parallelOf(e)
	id, isIdent := x.(*ast.Ident)
	return ok && isIdent && id.Name == param
}

// parallelOf returns x where n is x.Parallel(), a call without
// arguments, and whether it is.
func parallelOf(n ast.Node) (ast.Expr, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Parallel" {
		return nil, false
	}
	return sel.X, true
}
