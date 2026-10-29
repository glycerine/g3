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
