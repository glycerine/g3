package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// TestAPIFixedpointReasonAfterApproximatedQuery: Z3_fixedpoint_query and
// Z3_fixedpoint_query_relations end with ctx().cleanup(), which sets the
// status to OK, so Z3_fixedpoint_get_reason_unknown answers "ok" after a
// query the bound_relation plugin can only approximate (the Int relation
// q). The fixed fork's libz3 prints result 0 / reason ok for the same
// program through the C API.
func TestAPIFixedpointReasonAfterApproximatedQuery(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetSymbol("engine", "datalog"))
	astTestMust(t, params.SetSymbol("datalog.default_relation", "bound_relation"))
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	integer, err := f.c.IntegerSort()
	astTestMust(t, err)
	bv, err := f.c.BitVectorSort(2)
	astTestMust(t, err)
	q, err := f.fp.DeclareRelation("q", []ast.ASTHandle{integer})
	astTestMust(t, err)
	r, err := f.fp.DeclareRelation("r", []ast.ASTHandle{bv})
	astTestMust(t, err)
	tt, err := f.fp.DeclareRelation("t", []ast.ASTHandle{bv})
	astTestMust(t, err)
	astTestMust(t, f.fp.SetPredicateRepresentation(r, "tr_sparse"))
	astTestMust(t, f.fp.AddTableFact(q, []uint32{3}))
	astTestMust(t, f.fp.AddTableFact(r, []uint32{1}))
	astTestMust(t, f.fp.AddTableFact(r, []uint32{2}))
	x := f.pin(f.c.BoundVariable(1, integer))
	y := f.pin(f.c.BoundVariable(0, bv))
	body := f.pin(f.m.MkAndArray([]ast.ASTHandle{f.pin(f.m.MkApp(q, []ast.ASTHandle{x})), f.pin(f.m.MkApp(r, []ast.ASTHandle{y})), f.pin(f.c.ArithmeticGT(x, f.pin(f.c.IntNumeral(2, integer))))}))
	xName, err := f.m.Symbols.Intern("x")
	astTestMust(t, err)
	yName, err := f.m.Symbols.Intern("y")
	astTestMust(t, err)
	rule := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, bv}, Names: []symbol.Symbol{xName, yName}, Body: f.pin(f.m.MkImplies(body, f.pin(f.m.MkApp(tt, []ast.ASTHandle{y}))))}))
	astTestMust(t, f.fp.AddRule(rule, "mixed"))
	for _, step := range []struct {
		declaration ast.ASTHandle
		want        lbool.LBool
	}{{tt, lbool.True}, {q, lbool.Undef}, {r, lbool.True}} {
		status, err := f.fp.QueryRelations([]ast.ASTHandle{step.declaration})
		astTestMust(t, err)
		if status != step.want {
			t.Fatal("query status", status, "want", step.want)
		}
		reason, err := f.fp.ReasonUnknown()
		astTestMust(t, err)
		if reason != "ok" {
			t.Fatalf("reason unknown after the query: %q, want \"ok\"", reason)
		}
	}
	// Z3_fixedpoint_query of the same relation takes the same cleanup.
	status, err := f.fp.Query(q)
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("expression query status", status)
	}
	reason, err := f.fp.ReasonUnknown()
	astTestMust(t, err)
	if reason != "ok" {
		t.Fatalf("reason unknown after Z3_fixedpoint_query: %q, want \"ok\"", reason)
	}
}
