package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestQueryExpressionScopesCompoundRuleAndPreservesProof(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	cooperation, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(cooperation.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: globals.Context, Globals: globals, Cooperation: cooperation})
	must(err)
	defer func() { must(lite.Close()) }()
	querySolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(querySolver.Close()) }()
	subsumptionSolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(subsumptionSolver.Close()) }()
	ruleManager, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(ruleManager.Close()) }()
	rules, err := base.NewRuleSet(ruleManager)
	must(err)
	defer func() { must(rules.Close()) }()

	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	integer, err := m.GetSort(one)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declarations := make([]ast.ASTHandle, 3)
	for i, text := range []string{"compound-p", "compound-q", "compound-missing"} {
		name, e := m.Symbols.Intern(text)
		must(e)
		declarations[i], e = m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(ruleManager.RegisterPredicate(declarations[i]))
	}
	apply := func(declaration, argument ast.ASTHandle) ast.ASTHandle {
		application, e := m.MkApp(declaration, []ast.ASTHandle{argument})
		must(e)
		return application
	}
	for _, declaration := range declarations[:2] {
		rule, e := ruleManager.NewRule(apply(declaration, one), nil, nil, "fact")
		must(e)
		must(rules.Add(rule))
		must(rule.Close())
	}
	originalCount, err := rules.Len()
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	xRef, err := m.NewRef(x)
	must(err)
	defer func() { must(m.DelRef(&xRef)) }()
	sparse, err := m.MkVar(2, integer)
	must(err)
	sparseRef, err := m.NewRef(sparse)
	must(err)
	defer func() { must(m.DelRef(&sparseRef)) }()

	for _, tc := range []struct {
		name   string
		right  ast.ASTHandle
		value  ast.ASTHandle
		status lbool.LBool
	}{
		{name: "reachable", right: declarations[1], value: x, status: lbool.True},
		{name: "sparse-variable", right: declarations[1], value: sparse, status: lbool.True},
		{name: "unreachable", right: declarations[2], value: x, status: lbool.False},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, e := m.MkAnd(apply(declarations[0], tc.value), apply(tc.right, tc.value))
			must(e)
			status, answer, e := QueryExpression(rules, query, rw, lite, querySolver, subsumptionSolver, "first")
			must(e)
			if status != tc.status {
				t.Fatal("compound query status", status, tc.status)
			}
			formula, e := m.RefGet(answer)
			must(e)
			if status == lbool.True {
				proof, e := m.IsProof(formula)
				must(e)
				if !proof {
					t.Fatal("reachable compound query did not publish a proof")
				}
				fact, e := m.GetProofFact(formula)
				must(e)
				if !m.IsFalse(fact) {
					t.Fatal("compound proof has wrong conclusion")
				}
			} else if !m.IsTrue(formula) {
				t.Fatal("unreachable compound answer is not true")
			}
			must(m.DelRef(&answer))
			count, e := rules.Len()
			must(e)
			if count != originalCount {
				t.Fatal("compound query changed source program", originalCount, count)
			}
		})
	}
}
