package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// TestAPIFixedpointInlinerLinearFacts: AUDIT.md FUZZ-F5. With
// xform.inline_eager=false and xform.inline_linear_branch, middle has an
// API fact (a relation fact) and a rule from seed; left and right read
// middle. z3 4.7.1's mk_rule_inliner collects the predicates with facts
// (count_pred_occurrences) only when inlining eagerly, so inline_linear
// inlined middle away and lost its fact (left = right = {false}). The
// expected answer is the fixed fork's (FORK_BUGFIXES.md,
// tst_z3_4_7_1_inliner_linear_facts): {true, false} for both.
func TestAPIFixedpointInlinerLinearFacts(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetSymbol("engine", "datalog"))
	astTestMust(t, params.SetBool("xform.inline_eager", false))
	astTestMust(t, params.SetBool("xform.inline_linear_branch", true))
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	var d [4]ast.ASTHandle
	for i, name := range []string{"seed", "middle", "left", "right"} {
		d[i], err = f.fp.DeclareRelation(name, []ast.ASTHandle{boolean})
		astTestMust(t, err)
	}
	astTestMust(t, f.fp.AddTableFact(d[0], []uint32{0}))
	astTestMust(t, f.fp.AddTableFact(d[1], []uint32{1}))
	x := f.pin(f.c.BoundVariable(0, boolean))
	xName, err := f.m.Symbols.Intern("x")
	astTestMust(t, err)
	for _, pair := range [][2]int{{0, 1}, {1, 2}, {1, 3}} {
		body := f.pin(f.m.MkImplies(f.pin(f.m.MkApp(d[pair[0]], []ast.ASTHandle{x})), f.pin(f.m.MkApp(d[pair[1]], []ast.ASTHandle{x}))))
		rule := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{xName}, Body: body}))
		astTestMust(t, f.fp.AddRule(rule, ""))
	}
	status, err := f.fp.QueryRelations([]ast.ASTHandle{d[2], d[3]})
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("status", status)
	}
	answer, err := f.fp.LastAnswer()
	astTestMust(t, err)
	h, err := f.m.RefGet(answer)
	astTestMust(t, err)
	text, err := f.c.ASTString(h, APIASTPrintSMTFull)
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&answer))
	want := "(and (or (= (:var 0) true) (= (:var 0) false))\n     (or (= (:var 0) true) (= (:var 0) false)))"
	if text != want {
		t.Fatalf("answer:\n%s\nwant\n%s", text, want)
	}
}
