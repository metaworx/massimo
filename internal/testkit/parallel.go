// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package testkit

// The rule of parallel tests (AP TestSpeed, D11-A): in a package held to
// it, every test calls t.Parallel() before anything else, but those on
// the package's list of serial tests, each with its reason, which Go
// runs one after the other before the parallel ones begin; a test that
// swaps a package variable says so through Swapping. docs/testing.md,
// "Parallel tests".

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// SwapVar is the environment variable Swapping sets for the test.
const SwapVar = "MASSIMO_TEST_SWAP"

// Swapping marks the test as one that swaps a package variable, such as
// a hook, until its end: every swap in a package held to the rule of
// parallel tests, or the helper that makes it, calls it first. It sets
// SwapVar through t.Setenv, which panics in a parallel test, and after
// which t.Parallel panics: a swap in a parallel test fails at once,
// where it would otherwise race with the tests beside it.
func Swapping(t testing.TB) {
	t.Helper()
	t.Setenv(SwapVar, "1")
}

// ParallelOrListed holds the package whose test files are in dir to the
// rule of parallel tests, and fails t for each place that breaks it:
// every test calls t.Parallel() as its first statement, or is on serial
// and does not call it; serial names only tests of the package, each
// with its reason; and Parallel() is called only in a test's own body or
// a subtest's function literal, never in a helper, where it would make
// a test parallel that the list holds serial. Every *_test.go file is
// read, whatever system it builds for, so that a test only Windows runs
// is held to the rule on Linux too. A package calls it from its own
// TestParallelOrListed, with dir ".".
func ParallelOrListed(t testing.TB, dir string, serial map[string]string) {
	t.Helper()
	problems, err := parallelProblems(dir, serial)
	if err != nil {
		t.Fatalf("testkit: %v", err)
	}
	for _, p := range problems {
		t.Errorf("%s", p)
	}
}

// parallelProblems returns where the test files in dir break the rule of
// parallel tests, in the order of the files and then of the list's
// names.
func parallelProblems(dir string, serial map[string]string) ([]string, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, fmt.Errorf("the test files in %s: %w", dir, err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no test files in %s", dir)
	}
	sort.Strings(names)
	fset := token.NewFileSet()
	seen := map[string]bool{}
	var problems []string
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, at := range strayParallel(fset, f) {
			problems = append(problems, fmt.Sprintf("%s: Parallel() outside a test's body and a subtest's function literal", at))
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || !isTestFunc(fn) {
				continue
			}
			test, at := fn.Name.Name, fset.Position(fn.Pos())
			seen[test] = true
			param := fn.Type.Params.List[0].Names[0].Name
			reason, listed := serial[test]
			switch {
			case listed && callsParallel(fn.Body, param):
				problems = append(problems, fmt.Sprintf("%s: %s is on the serial list (%s) and calls %s.Parallel()", at, test, reason, param))
			case !listed && !parallelFirst(fn.Body, param):
				problems = append(problems, fmt.Sprintf("%s: %s neither calls %s.Parallel() first nor is on the serial list with its reason", at, test, param))
			}
		}
	}
	listed := make([]string, 0, len(serial))
	for test := range serial {
		listed = append(listed, test)
	}
	sort.Strings(listed)
	for _, test := range listed {
		switch {
		case !seen[test]:
			problems = append(problems, fmt.Sprintf("the serial list names %s, which is no test of the package", test))
		case strings.TrimSpace(serial[test]) == "":
			problems = append(problems, fmt.Sprintf("the serial list names %s without a reason", test))
		}
	}
	return problems, nil
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
