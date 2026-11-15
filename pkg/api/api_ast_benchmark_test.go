package api

import (
	"strconv"
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
		// display_smt2 passes the literal true as smt_printer's
		// simplify_implies; the setting only reaches its indent parameter.
		{"false", "(=> (and a b) c)"},
	} {
		astTestMust(t, globals.SourceSetName("pp.simplify_implies", tc.setting))
		text, err := f.c.BenchmarkSMT2(root, APIBenchmarkConfig{})
		astTestMust(t, err)
		if !strings.Contains(text, tc.want) {
			t.Fatal("benchmark implication setting", tc.setting, text)
		}
	}
}

// AUDIT.md ast_07-F5: decl_collector::order_deps emits the sorts in
// top_sort's obj_map table order. The expected text is what
// Z3_benchmark_to_smtlib_string(ctx, "b", "", "", "", 0, 0, r) prints with
// build/libz3.so for the same objects.
func TestAPIBenchmarkSortOrderMatchesObjMap(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	eqs := make([]ast.ASTHandle, 6)
	for i := range eqs {
		s := f.pin(f.c.UninterpretedSort("S" + strconv.Itoa(i)))
		x := f.pin(f.c.Constant("x"+strconv.Itoa(i), s))
		eqs[i] = f.pin(f.c.Equal(x, x))
	}
	root := f.pin(f.c.And(eqs...))
	text, err := f.c.BenchmarkSMT2(root, APIBenchmarkConfig{Name: "b"})
	astTestMust(t, err)
	want := "(declare-sort S2)\n (declare-sort S0)\n (declare-sort S1)\n (declare-sort S5)\n (declare-sort S4)\n (declare-sort S3)\n (declare-fun x5 () S5)\n(declare-fun x4 () S4)\n(declare-fun x3 () S3)\n(declare-fun x2 () S2)\n(declare-fun x1 () S1)\n(declare-fun x0 () S0)\n(assert\n (and (= x0 x0) (= x1 x1) (= x2 x2) (= x3 x3) (= x4 x4) (= x5 x5)))\n(check-sat)\n"
	if !strings.HasSuffix(text, want) {
		t.Fatalf("benchmark sort order:\n%s\nwant suffix:\n%s", text, want)
	}
}
