package fp_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// queryRelation is context::rel_query on a nullary predicate application,
// the relation path these bytecode tests exercise.
func queryRelation(t *testing.T, engine *fp.PropositionalEngine, application ast.ASTHandle) (lbool.LBool, error) {
	t.Helper()
	declaration, err := fp.EngineManager(engine).GetDecl(application)
	if err != nil {
		return lbool.Undef, err
	}
	return engine.QueryPredicate(declaration)
}

// AUDIT.md muz_fp_04-F3: Z3_fixedpoint_query is context::query(expr*) even
// for a nullary registered predicate, so it installs m_mc and answers from
// the expression query (query!N clone), not rel_query.
func TestQueryNullaryPredicateUsesContextQuery(t *testing.T) {
	f := newQueryExpressionFixture(t)
	globals, err := f.context.GlobalParameters()
	queryMust(t, err)
	engine, err := fp.NewPropositionalEngineWithServices(f.manager, globals.Context, globals)
	queryMust(t, err)
	queryMust(t, f.engine.Close())
	f.engine = engine
	queryMust(t, f.engine.SelectEngine("pdr"))
	zero := f.pin(f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	integer := f.pin(f.manager.GetSort(zero))
	r, _ := f.predicate("nullary-context-r", integer)
	s, _ := f.predicate("nullary-context-s", integer)
	_, p := f.predicate("nullary-context-p")
	x := f.pin(f.manager.MkVar(0, integer))
	rx := f.pin(f.manager.MkApp(r, []ast.ASTHandle{x}))
	sx := f.pin(f.manager.MkApp(s, []ast.ASTHandle{x}))
	queryMust(t, f.engine.AddRule(f.pin(f.manager.MkApp(r, []ast.ASTHandle{zero})), "r0"))
	queryMust(t, f.engine.AddRule(f.pin(f.manager.MkImplies(rx, sx)), "rs"))
	positive := f.pin(f.manager.MkArithmeticGT(x, zero))
	queryMust(t, f.engine.AddRule(f.pin(f.manager.MkImplies(f.pin(f.manager.MkAnd(sx, positive)), p)), "sp"))
	status, err := f.engine.Query(p)
	queryMust(t, err)
	if status != lbool.False {
		t.Fatal("nullary query result", status)
	}
	if !fp.ContextModelConverterInstalled(f.engine) {
		t.Fatal("nullary Query did not run context::query (m_mc not installed)")
	}
	answer, err := f.engine.ExpressionAnswer()
	queryMust(t, err)
	queryMust(t, f.manager.DelRef(&answer))
}
