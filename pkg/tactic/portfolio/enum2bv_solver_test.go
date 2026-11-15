package portfolio_test

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func enumMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type enumFixture struct {
	s        *portfolio.Enum2BVSolver
	backend  *smtcore.SMTSolver
	manager  *ast.ASTManager
	context  *cmd_context.Context
	services model.ModelEvaluatorConfig
}

func enumSolverFixture(t *testing.T) *enumFixture {
	t.Helper()
	c, err := cmd_context.NewStandaloneContext()
	enumMust(t, err)
	var budget *arena.Budget
	t.Cleanup(func() {
		enumMust(t, c.Close())
		if budget != nil && budget.Used() != 0 {
			t.Errorf("enum solver retained %d bytes", budget.Used())
		}
	})
	enumMust(t, smt2.ParseCommands(c, strings.NewReader(`(set-logic ALL)(declare-datatypes () ((E red amber green)))(declare-const x E)(assert (= x green))`), io.Discard))
	m, err := c.Manager()
	enumMust(t, err)
	budget = m.Budget()
	globals, err := c.GlobalParameters()
	enumMust(t, err)
	old, err := uparams.BindProcessGlobalParamsService(globals)
	enumMust(t, err)
	t.Cleanup(func() { _, e := uparams.BindProcessGlobalParamsService(old); enumMust(t, e) })
	coop, err := cooperate.NewCooperation(m.Budget())
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, coop.Close()) })
	services := model.ModelEvaluatorConfig{Context: globals.Context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: coop}
	backend, err := smtcore.NewSMTSolver(m, nil, "QF_BV", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals, ModelEvaluator: services})
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, backend.Close()) })
	s, err := portfolio.NewEnum2BVSolver(m, globals.Context, nil, backend, services)
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, s.Close()) })
	enumMust(t, s.SetProduceModels(true))
	return &enumFixture{s: s, backend: backend, manager: m, context: c, services: services}
}
func enumGoal(t *testing.T, f *enumFixture) (*tactic.Goal, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	g, err := tactic.NewGoal(f.manager, false, true, false)
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, g.Close()) })
	enumMust(t, f.context.AssertGoal(g))
	h, err := g.Form(0)
	enumMust(t, err)
	x, err := f.manager.GetArg(h, 0)
	enumMust(t, err)
	return g, h, x
}
func enumPin(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) {
	t.Helper()
	r, e := m.NewRef(h)
	enumMust(t, e)
	t.Cleanup(func() { enumMust(t, m.DelRef(&r)) })
}
func enumTag(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	b, e := m.MkBoolSort()
	enumMust(t, e)
	n, e := m.Symbols.Intern(name)
	enumMust(t, e)
	h, e := m.MkConst(n, b)
	enumMust(t, e)
	enumPin(t, m, h)
	return h
}
func TestEnum2BVSolverIncrementalModelsAndScopes(t *testing.T) {
	f := enumSolverFixture(t)
	m, s := f.manager, f.s
	g, h, x := enumGoal(t, f)
	enumMust(t, s.Assert(h))
	n, err := s.NumAssertions()
	enumMust(t, err)
	if n != 2 {
		t.Fatal("enum assertion must precede its side bound", n)
	}
	rewritten, err := s.Assertion(0)
	enumMust(t, err)
	op, err := m.GetDeclKind(rewritten)
	enumMust(t, err)
	if op != ast.ASTOpEq {
		t.Fatal("side bound published before rewritten assertion", op)
	}
	bound, err := s.Assertion(1)
	enumMust(t, err)
	var lower, upper ast.ASTHandle
	yes, err := m.BitVectorULEOperands(bound, &lower, &upper)
	enumMust(t, err)
	if !yes {
		t.Fatal("missing ordered enum bound")
	}

	bv, err := m.GetArg(rewritten, 0)
	enumMust(t, err)
	bvSort, err := m.GetSort(bv)
	enumMust(t, err)
	width, err := m.BitVectorSize(bvSort)
	enumMust(t, err)
	if width != 2 {
		t.Fatal("rewritten width", width)
	}
	status, err := s.CheckSat(nil)
	enumMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	enumMust(t, s.Push())
	not, err := m.MkNot(h)
	enumMust(t, err)
	enumMust(t, s.Assert(not))
	status, err = s.CheckSat(nil)
	enumMust(t, err)
	if status != lbool.False {
		t.Fatal("scoped contradiction", status)
	}
	enumMust(t, s.Pop(1))
	enumMust(t, s.Pop(0))
	status, err = s.CheckSat(nil)
	enumMust(t, err)
	if status != lbool.True {
		t.Fatal("pop did not restore satisfiability", status)
	}
	md, err := s.AcquireModel()
	enumMust(t, err)
	if md == nil {
		t.Fatal("missing model")
	}
	t.Cleanup(func() { enumMust(t, md.DecRef()) })
	d, err := m.GetDecl(x)
	enumMust(t, err)
	bvd, err := m.GetDecl(bv)
	enumMust(t, err)
	enumPin(t, m, bvd)
	enumMust(t, s.Close())
	enumMust(t, g.Close())
	value, err := md.ConstantInterpretation(d)
	enumMust(t, err)
	vd, err := m.GetDecl(value)
	enumMust(t, err)
	name, err := m.GetASTName(vd)
	enumMust(t, err)
	text, err := m.Symbols.String(name)
	enumMust(t, err)
	if text != "green" {
		t.Fatal("enum interpretation", text)
	}
	hidden, err := md.ConstantInterpretation(bvd)
	enumMust(t, err)
	if !hidden.IsNull() {
		t.Fatal("fresh BV declaration escaped model filtering")
	}
}
func TestEnum2BVSolverTrackedAssumptionsAndCores(t *testing.T) {
	f := enumSolverFixture(t)
	m, s := f.manager, f.s
	_, h, _ := enumGoal(t, f)
	first := enumTag(t, m, "enum-first")
	second := enumTag(t, m, "enum-second")
	extra := enumTag(t, m, "enum-extra")
	enumMust(t, s.AssertTracked(h, first))
	enumMust(t, s.Push())
	not, err := m.MkNot(h)
	enumMust(t, err)
	enumMust(t, s.AssertTracked(not, second))
	status, err := s.CheckSat([]ast.ASTHandle{extra})
	enumMust(t, err)
	if status != lbool.False {
		t.Fatal("tracked contradiction", status)
	}
	core, err := s.UnsatCore()
	enumMust(t, err)
	found := map[ast.ASTHandle]bool{}
	for _, h := range core {
		found[h] = true
	}
	if !found[first] || !found[second] || found[extra] {
		t.Fatal("source named core", core)
	}
	count, err := s.NumAssumptions()
	enumMust(t, err)
	if count != 2 {
		t.Fatal("temporary assumption survived", count)
	}
	enumMust(t, s.Pop(1))
	count, err = s.NumAssumptions()
	enumMust(t, err)
	if count != 1 {
		t.Fatal("scoped named assumption survived", count)
	}
	status, err = s.CheckSat(nil)
	enumMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
}
func TestEnum2BVSolverConsequencesTranslateAndInternalize(t *testing.T) {
	f := enumSolverFixture(t)
	m, s := f.manager, f.s
	_, h, x := enumGoal(t, f)
	enumMust(t, s.Assert(h))
	status, out, err := s.GetConsequences(nil, []ast.ASTHandle{x})
	enumMust(t, err)
	if status != lbool.True || len(out) != 1 {
		t.Fatal("forced enum consequence", status, out)
	}
	kind, err := m.GetDeclKind(out[0])
	enumMust(t, err)
	if kind != ast.ASTOpImplies {
		t.Fatal("consequence implication", kind)
	}
	head, err := m.GetArg(out[0], 1)
	enumMust(t, err)
	left, err := m.GetArg(head, 0)
	enumMust(t, err)
	right, err := m.GetArg(head, 1)
	enumMust(t, err)
	value, err := m.GetArg(h, 1)
	enumMust(t, err)
	if left != x || right != value {
		t.Fatal("consequence was not converted back to enum", left, right)
	}
	// A queried variable absent from assertions must still get its mapping and
	// side bound before the underlying consequence query is invoked.
	sort, err := m.GetSort(x)
	enumMust(t, err)
	name, err := m.Symbols.Intern("absent-enum")
	enumMust(t, err)
	absent, err := m.MkConst(name, sort)
	enumMust(t, err)
	enumPin(t, m, absent)
	before, err := s.NumAssertions()
	enumMust(t, err)
	status, out, err = s.GetConsequences(nil, []ast.ASTHandle{absent})
	enumMust(t, err)
	after, err := s.NumAssertions()
	enumMust(t, err)
	if status != lbool.True || len(out) != 0 || after != before+1 {
		t.Fatal("absent-variable bound/internalization", status, len(out), before, after)
	}
}

var _ solver.Solver = (*portfolio.Enum2BVSolver)(nil)

// AUDIT.md ast_rewriter_03-F2: enum2bv keeps m_enum2def / m_enum2bv as
// obj_maps, and the solver's model extension walks them in table order, so
// the model's constant order is deterministic and matches z3 (e, d, c, a,
// b, f).
func TestEnum2BVSolverModelOrderDeterministic(t *testing.T) {
	const script = `(set-logic QF_FD)
(declare-datatypes () ((Color (red) (green) (blue))))
(declare-const a Color)(declare-const b Color)(declare-const c Color)
(declare-const d Color)(declare-const e Color)(declare-const f Color)
(assert (distinct a b c))(assert (distinct d e f))
(assert (not (= a red)))(assert (not (= d green)))
(check-sat)(get-model)`
	var first string
	for i := 0; i < 20; i++ {
		c, err := cmd_context.NewStandaloneContext()
		enumMust(t, err)
		var out strings.Builder
		enumMust(t, smt2.ParseCommands(c, strings.NewReader(script), &out))
		enumMust(t, c.Close())
		got := out.String()
		if i == 0 {
			first = got
			var order []string
			for _, line := range strings.Split(got, "\n") {
				if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "(define-fun" {
					order = append(order, fields[1])
				}
			}
			if strings.Join(order, " ") != "e d c a b f" {
				t.Fatal("model constant order", order, got)
			}
		} else if got != first {
			t.Fatal("nondeterministic model", i, got, first)
		}
	}
}
