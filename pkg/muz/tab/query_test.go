package tab_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/tab"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestQueryPredicateBuildsProofAndUnreachableAnswer(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	coop, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(coop.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: globals.Context, Globals: globals, Cooperation: coop})
	must(err)
	defer func() { must(lite.Close()) }()
	querySolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(querySolver.Close()) }()
	subSolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(subSolver.Close()) }()
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	rules, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(rules.Close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	sort, err := m.GetSort(one)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	makeDecl := func(name string) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(d))
		return d
	}
	p, q := makeDecl("query-p"), makeDecl("query-q")
	app := func(d, a ast.ASTHandle) ast.ASTHandle { h, e := m.MkApp(d, []ast.ASTHandle{a}); must(e); return h }
	x, err := m.MkVar(0, sort)
	must(err)
	rule, err := rm.NewRule(app(p, x), []ast.ASTHandle{app(q, x)}, nil, "step")
	must(err)
	must(rules.Add(rule))
	must(rule.Close())
	rule, err = rm.NewRule(app(q, one), nil, nil, "fact")
	must(err)
	must(rules.Add(rule))
	must(rule.Close())
	for _, tc := range []struct {
		value ast.ASTHandle
		want  lbool.LBool
	}{{one, lbool.True}, {two, lbool.False}} {
		status, answer, e := tab.QueryPredicate(rules, app(p, tc.value), rw, lite, querySolver, subSolver, "first")
		must(e)
		if status != tc.want {
			t.Fatal("query status", status, tc.want)
		}
		h, e := m.RefGet(answer)
		must(e)
		if status == lbool.True {
			kind, e := m.GetDeclKind(h)
			must(e)
			if kind != ast.ASTPrHyperResolve {
				t.Fatal("expected hyper-resolution proof", kind)
			}
			fact, e := m.GetProofFact(h)
			must(e)
			if !m.IsFalse(fact) {
				t.Fatal("proof does not conclude false")
			}
		} else if !m.IsTrue(h) {
			t.Fatal("unreachable query answer is not true")
		}
		must(m.DelRef(&answer))
	}
	negative, err := rm.NewRule(app(p, one), []ast.ASTHandle{app(q, one)}, []bool{true}, "negative")
	must(err)
	must(rules.Add(negative))
	must(negative.Close())
	_, answer, err := tab.QueryPredicate(rules, app(p, one), rw, lite, querySolver, subSolver, "first")
	if err == nil || answer != (ast.ASTRef{}) {
		t.Fatal("tabulation accepted a negated predicate", err)
	}
}
