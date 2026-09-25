package main

import (
	"go/parser"
	"go/token"
	"testing"
)

// TestRunCreate_DoesNotDependOnConfigLoad enforces, at the source level,
// that the invites CLI never drags in internal/config. The CLI is an
// admin-only tool: bringing config.Load() back would reintroduce the bug
// from #173, M14 — a tool that fails to run because CLERK_JWKS_URL is
// unset (a variable it has no business reading).
//
// The test parses main.go and create.go and fails if any file in the
// package imports internal/config. Static check rather than runtime:
// no DB, no flakiness, no extra deps.
func TestRunCreate_DoesNotDependOnConfigLoad(t *testing.T) {
	const forbidden = `"github.com/fgjcarlos/ghamusinos/internal/config"`

	fset := token.NewFileSet()
	for _, path := range []string{"main.go", "create.go"} {
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q): %v", path, err)
		}
		for _, imp := range f.Imports {
			if imp.Path.Value == forbidden {
				t.Errorf("%s importa internal/config — el CLI admin no debe depender de config.Load() (issue #173, M14)", path)
			}
		}
	}
}
