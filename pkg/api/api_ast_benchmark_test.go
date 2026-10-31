package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTBenchmarkSMT2MetadataAssumptionsAndFormula(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("benchmark-x", boolean))
	formula := f.pin(f.c.Not(x))
	text, err := f.c.BenchmarkSMT2(formula, APIBenchmarkConfig{
		Name: "api-benchmark", Logic: "QF_UF", Status: "sat", Source: "Go API",
		Category: "unit", Attributes: ":notes { generated }", Assumptions: []ast.ASTHandle{x},
	})
	astTestMust(t, err)
	for _, expected := range []string{"api-benchmark", "QF_UF", ":status sat", "benchmark-x", "generated", "(not benchmark-x)"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("benchmark %q lacks %q", text, expected)
		}
	}
}

func TestAPIBenchmarkUsesGlobalSimplifyImplies(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	a := f.pin(f.c.Constant("a", boolean))
	b := f.pin(f.c.Constant("b", boolean))
	c := f.pin(f.c.Constant("c", boolean))
	inner := f.pin(f.c.Implies(b, c))
	root := f.pin(f.c.Implies(a, inner))
	globals, err := f.c.command.GlobalParameters()
	astTestMust(t, err)
	for _, tc := range []struct{ setting, want string }{
		{"true", "(=> (and a b) c)"},
		{"false", "(=> a (=> b c))"},
	} {
		astTestMust(t, globals.SourceSetName("pp.simplify_implies", tc.setting))
		text, err := f.c.BenchmarkSMT2(root, APIBenchmarkConfig{})
		astTestMust(t, err)
		if !strings.Contains(text, tc.want) {
			t.Fatal("benchmark implication setting", tc.setting, text)
		}
	}
}
