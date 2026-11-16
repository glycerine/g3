package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// TestAPIFixedpointBoundFavouriteTableRepresentation: AUDIT.md FUZZ-F5
// (relation plugins). With the bound_relation favourite, an Int predicate
// with an API fact (rel_context::add_fact into its bound relation) and a
// bit-vector predicate with an explicit tr_sparse representation feed a
// rule into a bit-vector head. g3 refused the Int fact as an unsupported
// bound fragment; the program runs on the relation plugins as in z3. The
// expected answers are the fixed fork's C API answers
// (Z3_fixedpoint_set_predicate_representation, Z3_fixedpoint_add_fact,
// Z3_fixedpoint_query_relations): the Int predicate's bound relation is
// not precise, so its own query is l_undef.
func TestAPIFixedpointBoundFavouriteTableRepresentation(t *testing.T) {
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
	out, err := f.fp.DeclareRelation("t", []ast.ASTHandle{bv})
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
	rule := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, bv}, Names: []symbol.Symbol{xName, yName}, Body: f.pin(f.m.MkImplies(body, f.pin(f.m.MkApp(out, []ast.ASTHandle{y}))))}))
	astTestMust(t, f.fp.AddRule(rule, "mixed"))
	for _, c := range []struct {
		decl   ast.ASTHandle
		status lbool.LBool
		want   string
	}{
		{out, lbool.True, "(or (= (:var 0) #b10) (= (:var 0) #b01))"},
		{q, lbool.Undef, ""},
		{r, lbool.True, "(or (= (:var 0) #b01) (= (:var 0) #b10))"},
	} {
		status, err := f.fp.QueryRelations([]ast.ASTHandle{c.decl})
		astTestMust(t, err)
		if status != c.status {
			t.Fatal("status", c.want, status)
		}
		if status != lbool.True {
			continue
		}
		answer, err := f.fp.LastAnswer()
		astTestMust(t, err)
		h, err := f.m.RefGet(answer)
		astTestMust(t, err)
		text, err := f.c.ASTString(h, APIASTPrintSMTFull)
		astTestMust(t, err)
		astTestMust(t, f.m.DelRef(&answer))
		if text != c.want {
			t.Fatalf("answer:\n%s\nwant\n%s", text, c.want)
		}
	}
}
