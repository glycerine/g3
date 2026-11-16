package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// TestAPIFixedpointMagicSetsExpressionQueryRows: context::query flushes the
// added rules (flush_add_rules) before rel_context::query builds the query
// rule and runs mk_magic_sets, so the adorned rule joins r before q (the
// extensional tails pop from the back) and the answer lists #b01 first.
// The query clone used to flush after the transform, removing two magic
// rules and re-adding the source rules. Expected answers are the fixed
// fork's through the C API (Z3_fixedpoint_query of exists y. t(y)).
func TestAPIFixedpointMagicSetsExpressionQueryRows(t *testing.T) {
	for _, sparse := range []bool{true, false} {
		f := newAPIFixedpointFixture(t)
		params, err := f.c.NewParameterSet()
		astTestMust(t, err)
		astTestMust(t, params.SetSymbol("engine", "datalog"))
		astTestMust(t, params.SetSymbol("datalog.default_relation", "bound_relation"))
		astTestMust(t, params.SetBool("datalog.magic_sets_for_queries", true))
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
		if sparse {
			astTestMust(t, f.fp.SetPredicateRepresentation(r, "tr_sparse"))
		}
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
		query := f.pin(f.c.Exists([]ast.ASTHandle{bv}, []string{"y"}, f.pin(f.m.MkApp(tt, []ast.ASTHandle{f.pin(f.c.BoundVariable(0, bv))}))))
		status, err := f.fp.QueryExpression(query)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("magic-sets expression query status", status)
		}
		answer, err := f.fp.LastAnswer()
		astTestMust(t, err)
		handle, err := f.m.RefGet(answer)
		astTestMust(t, err)
		text, err := f.c.ASTString(handle, APIASTPrintSMTFull)
		astTestMust(t, err)
		astTestMust(t, f.m.DelRef(&answer))
		if want := "(or (= (:var 0) #b01) (= (:var 0) #b10))"; text != want {
			t.Fatalf("sparse=%v: answer %s, want %s", sparse, text, want)
		}
	}
}
