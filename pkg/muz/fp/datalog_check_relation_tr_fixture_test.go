package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// TestDatalogCheckRelationTrSparseFixture runs a checked tr_* query
// (datalog.check_relation tr_sparse; check_relation's equivalence checks
// run the SMT kernel on quantified formulas) through the muz/fp engine
// fixture, as vector plugins4_check_tr_sparse_mixed does through cmd/g3:
// r(true); r(false) :- r(b), not p(x, b). The answer is the fixed fork's: (or (= (:var 0) true) (= (:var 0) false)).
// The fixture's AST manager now has the parameter services every g3
// context gives it (as z3's ast_manager reads gparams); without them
// check_equiv's kernel had no preprocessing and no quantifier support and
// failed with "AST source precondition" on the quantified formulas.
func TestDatalogCheckRelationTrSparseFixture(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "tr_sparse"))
	b, err := m.MkBoolSort()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	declare := func(text string, sorts ...ast.ASTHandle) ast.ASTHandle {
		name, err := m.Symbols.Intern(text)
		must(err)
		d, err := m.MkFuncDecl(name, sorts, b, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := declare("p", integer, b)
	r := declare("r", b)
	yes, err := m.MkTrue()
	must(err)
	no, err := m.MkFalse()
	must(err)
	fact, err := m.MkApp(r, []ast.ASTHandle{yes})
	must(err)
	must(e.addRuleEager(fact, ""))
	x, err := m.MkVar(1, integer)
	must(err)
	bv, err := m.MkVar(0, b)
	must(err)
	rb, err := m.MkApp(r, []ast.ASTHandle{bv})
	must(err)
	pxb, err := m.MkApp(p, []ast.ASTHandle{x, bv})
	must(err)
	notp, err := m.MkNot(pxb)
	must(err)
	body, err := m.MkAndArray([]ast.ASTHandle{rb, notp})
	must(err)
	head, err := m.MkApp(r, []ast.ASTHandle{no})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, ""))
	status, err := e.QueryPredicate(r)
	must(err)
	if status != lbool.True {
		t.Fatal("status", status)
	}
	answer, err := e.Answer(r)
	must(err)
	got, err := m.RefGet(answer)
	must(err)
	v, err := m.MkVar(0, b)
	must(err)
	isTrue, err := m.MkEq(v, yes)
	must(err)
	isFalse, err := m.MkEq(v, no)
	must(err)
	want, err := m.MkOrArray([]ast.ASTHandle{isTrue, isFalse})
	must(err)
	same := got == want
	must(m.DelRef(&answer))
	if !same {
		t.Fatal("answer is not (or (= (:var 0) true) (= (:var 0) false))")
	}
}
