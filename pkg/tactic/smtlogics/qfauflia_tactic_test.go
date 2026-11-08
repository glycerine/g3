package smtlogics_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func qfaufliaMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func qfaufliaPin(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) {
	t.Helper()
	ref, err := m.NewRef(h)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, m.DelRef(&ref)) })
}

func TestQFAUFLIATacticModelConverterSurvivesStrategyClose(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, c.Close()) })
	qfaufliaMust(t, c.SetLogic("QF_AUFLIA"))
	m, err := c.Manager()
	qfaufliaMust(t, err)
	runner, err := c.NewBuiltinTactic("qfauflia")
	qfaufliaMust(t, err)
	qfaufliaMust(t, runner.IncRef())
	closed := false
	t.Cleanup(func() {
		if !closed {
			qfaufliaMust(t, runner.DecRef())
		}
	})
	sort, err := c.LookupSort("Int")
	qfaufliaMust(t, err)
	name, err := m.Symbols.Intern("x")
	qfaufliaMust(t, err)
	x, err := m.MkConst(name, sort)
	qfaufliaMust(t, err)
	qfaufliaPin(t, m, x)
	three, err := m.ArithmeticIntegerNumeralText("3")
	qfaufliaMust(t, err)
	qfaufliaPin(t, m, three)
	eq, err := m.MkEq(x, three)
	qfaufliaMust(t, err)
	qfaufliaPin(t, m, eq)
	goal, err := tactic.NewGoal(m, false, true, false)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
	qfaufliaMust(t, goal.Assert(eq))
	var result tactic.TacticResult
	t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
	qfaufliaMust(t, runner.Apply(goal, &result))
	if !goal.IsDecidedSat() || result.ModelConverter == nil {
		t.Fatal("strategy failed to solve the definition with a model converter")
	}
	qfaufliaMust(t, runner.Cleanup())
	qfaufliaMust(t, runner.DecRef())
	closed = true
	var md tactic.ModelRef
	t.Cleanup(func() { qfaufliaMust(t, md.Close()) })
	qfaufliaMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &md))
	model, err := md.Get()
	qfaufliaMust(t, err)
	decl, err := m.GetDecl(x)
	qfaufliaMust(t, err)
	value, err := model.ConstantInterpretation(decl)
	qfaufliaMust(t, err)
	if value != three {
		t.Fatal("strategy result lost its definition after close", value)
	}
}

func TestQFAUFLIATacticTranslationAndProofLifetime(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, c.Close()) })
	qfaufliaMust(t, c.SetBoolOption("proof", true))
	qfaufliaMust(t, c.SetLogic("QF_AUFLIA"))
	m, err := c.Manager()
	qfaufliaMust(t, err)
	source, err := c.NewBuiltinTactic("qfauflia")
	qfaufliaMust(t, err)
	qfaufliaMust(t, source.IncRef())
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget(), ProofMode: ast.ASTProofEnabled})
	qfaufliaMust(t, err)
	// The source copy-constructs the target (ast_manager(m, ...)), which
	// keeps the source family ids.
	qfaufliaMust(t, target.CopyFamiliesPlugins(m))
	t.Cleanup(func() { qfaufliaMust(t, target.Close()) })
	translated, err := source.Translate(target)
	qfaufliaMust(t, err)
	qfaufliaMust(t, translated.IncRef())
	qfaufliaMust(t, source.DecRef())
	t.Cleanup(func() { qfaufliaMust(t, translated.DecRef()) })
	zero, err := target.ArithmeticIntegerNumeralText("0")
	qfaufliaMust(t, err)
	qfaufliaPin(t, target, zero)
	one, err := target.ArithmeticIntegerNumeralText("1")
	qfaufliaMust(t, err)
	qfaufliaPin(t, target, one)
	sort, err := target.GetSort(zero)
	qfaufliaMust(t, err)
	name, err := target.Symbols.Intern("proof-x")
	qfaufliaMust(t, err)
	x, err := target.MkConst(name, sort)
	qfaufliaMust(t, err)
	qfaufliaPin(t, target, x)
	eq, err := target.MkEq(x, zero)
	qfaufliaMust(t, err)
	qfaufliaPin(t, target, eq)
	family, err := target.GetFamilyID(zero)
	qfaufliaMust(t, err)
	lower, err := target.MkFamilyApp(family, 3, nil, []ast.ASTHandle{x, one})
	qfaufliaMust(t, err)
	qfaufliaPin(t, target, lower)
	goal, err := tactic.NewGoal(target, true, false, false)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
	for _, term := range []ast.ASTHandle{eq, lower} {
		pr, e := target.MkProofAsserted(term)
		qfaufliaMust(t, e)
		qfaufliaMust(t, goal.AssertWithProof(term, pr, ast.ASTDependency{}))
	}
	var model tactic.ModelRef
	t.Cleanup(func() { qfaufliaMust(t, model.Close()) })
	var labels tactic.ModelLabels
	t.Cleanup(func() { labels.Close() })
	proof, err := target.NewRef(ast.ASTHandle{})
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, target.DelRef(&proof)) })
	var dependency tactic.DependencyRef
	t.Cleanup(func() { qfaufliaMust(t, dependency.Close()) })
	status, reason, err := tactic.CheckSat(translated, goal, &model, &labels, proof, &dependency)
	qfaufliaMust(t, err)
	if status != lbool.False {
		t.Fatal("translated strategy failed", status, reason)
	}
	qfaufliaMust(t, translated.Cleanup())
	pr, err := target.RefGet(proof)
	qfaufliaMust(t, err)
	fact, err := target.GetProofFact(pr)
	qfaufliaMust(t, err)
	if !target.IsFalse(fact) {
		t.Fatal("strategy cleanup invalidated the contradiction proof")
	}
}
