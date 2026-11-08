package core

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func solveEqFixture(t *testing.T) (*ast.ASTManager, *uparams.ParamsContext, *uparams.GlobalParamsService, model.ModelEvaluatorConfig) {
	t.Helper()
	m, c, g := simplifyFixture(t)
	cooperation, err := cooperate.NewCooperation(m.Budget())
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, cooperation.Close()) })
	return m, c, g, model.ModelEvaluatorConfig{Context: c, Globals: g, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation}
}

func TestSolveEquationsBooleanDefinitionsAndIndependentModel(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	runner, err := NewSolveEquationsTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a, b, _ := propagateAtoms(t, m)
	equality, err := m.MkEq(a, b)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, false, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(equality))
	simplifyMust(t, goal.Assert(b))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if !goal.IsDecidedSat() || result.ModelConverter == nil {
		t.Fatal("definitions were not eliminated with model reconstruction")
	}
	if statistics.Sums(runner.CollectStatistics)["eliminated vars"] != 2 {
		t.Fatal("eliminated-variable statistics", statistics.Sums(runner.CollectStatistics))
	}
	simplifyMust(t, runner.Cleanup())
	if statistics.Sums(runner.CollectStatistics)["eliminated vars"] != 2 {
		t.Fatal("cleanup erased elimination statistics")
	}
	simplifyMust(t, runner.ResetStatistics())
	if len(statistics.Sums(runner.CollectStatistics)) != 0 {
		t.Fatal("elimination statistics were not reset")
	}
	simplifyMust(t, runner.Close())
	var md tactic.ModelRef
	t.Cleanup(func() { simplifyMust(t, md.Close()) })
	simplifyMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &md))
	model, err := md.Get()
	simplifyMust(t, err)
	for _, term := range []ast.ASTHandle{a, b} {
		decl, err := m.GetDecl(term)
		simplifyMust(t, err)
		value, err := model.ConstantInterpretation(decl)
		simplifyMust(t, err)
		if !m.IsTrue(value) {
			t.Fatal("eliminated Boolean value was not reconstructed", value)
		}
	}
}

func TestSolveEquationsTranslationPolicyAndCancellation(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	params, err := c.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, c.Del(&params)) })
	key := "theory_solver"
	simplifyMust(t, c.SetBoolName(&params, &key, false))
	source, err := NewSolveEquationsTactic(m, c, g, &params, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, source.Close()) })
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, target.Close()) })
	translated, err := source.Translate(target)
	simplifyMust(t, err)
	simplifyMust(t, translated.IncRef())
	t.Cleanup(func() { simplifyMust(t, translated.DecRef()) })
	simplifyMust(t, c.SetBoolName(&params, &key, true))
	simplifyMust(t, source.UpdateParams(&params))
	simplifyMust(t, source.Close())
	one, err := target.ArithmeticRealNumeralText("1")
	simplifyMust(t, err)
	simplifyPin(t, target, one)
	three, err := target.ArithmeticRealNumeralText("3")
	simplifyMust(t, err)
	simplifyPin(t, target, three)
	sort, err := target.GetSort(one)
	simplifyMust(t, err)
	name, err := target.Symbols.Intern("translated-x")
	simplifyMust(t, err)
	x, err := target.MkConst(name, sort)
	simplifyMust(t, err)
	simplifyPin(t, target, x)
	sum, err := target.MkArithmeticAdd(x, one)
	simplifyMust(t, err)
	equation, err := target.MkEq(sum, three)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(target, false, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(equation))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, translated.Apply(goal, &result))
	if goal.IsDecidedSat() {
		t.Fatal("source update changed the translated theory-solver policy")
	}
	simplifyMust(t, translated.UpdateParams(&params))
	simplifyMust(t, translated.Cleanup())
	target.ResourceLimit().IncCancel()
	err = translated.Apply(goal, &result)
	target.ResourceLimit().DecCancel()
	var exception tactic.TacticException
	if !errors.As(err, &exception) || exception.Message() != "canceled" {
		t.Fatal("elimination did not honor its checkpoint cancellation", err)
	}
	simplifyMust(t, translated.Cleanup())
	simplifyMust(t, translated.Apply(goal, &result))
	if !goal.IsDecidedSat() {
		t.Fatal("translated tactic did not recover from cancellation with updated policy")
	}
}

func TestSolveEquationsCycleBreakAndCPPApply(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	runner, err := NewSolveEquationsTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a, b, _ := propagateAtoms(t, m)
	boolean, err := m.MkBoolSort()
	simplifyMust(t, err)
	name, err := m.Symbols.Intern("f")
	simplifyMust(t, err)
	f, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	fa, err := m.MkApp(f, []ast.ASTHandle{a})
	simplifyMust(t, err)
	fb, err := m.MkApp(f, []ast.ASTHandle{b})
	simplifyMust(t, err)
	left, err := m.MkEq(a, fb)
	simplifyMust(t, err)
	right, err := m.MkEq(b, fa)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, false, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(left))
	simplifyMust(t, goal.Assert(right))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	simplifyMust(t, err)
	if size != 1 || runner.eliminated != 1 {
		t.Fatal("cyclic definitions were not cut at the native variable", size, runner.eliminated)
	}
	ffa, err := m.MkApp(f, []ast.ASTHandle{fa})
	simplifyMust(t, err)
	want, err := m.MkEq(a, ffa)
	simplifyMust(t, err)
	got, err := goal.Form(0)
	simplifyMust(t, err)
	if got != want {
		t.Fatal("cycle elimination order", got, want)
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		command := exec.Command(reference, "-in")
		command.Stdin = strings.NewReader("(set-logic QF_UF)(declare-const a Bool)(declare-const b Bool)(declare-fun f (Bool) Bool)(assert (= a (f b)))(assert (= b (f a)))(apply solve-eqs)")
		native, err := command.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(native))
		}
		var output bytes.Buffer
		printer, err := ast.NewLegacySMTPrinter(m)
		simplifyMust(t, err)
		output.WriteString("(goals (goal ")
		simplifyMust(t, printer.DisplayExpr(&output, got, nil))
		output.WriteString(" :precision precise :depth 1))")
		tokens := func(s string) string {
			return strings.Join(strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(s)), " ")
		}
		if tokens(output.String()) != tokens(string(native)) {
			t.Fatalf("cycle resolution differs from C++\nGo: %s\nC++: %s", output.String(), native)
		}
	}
}

func TestSolveEquationsArithmeticCoefficientPolicy(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, coefficient := range []string{"2", "-1"} {
			t.Run(fmt.Sprint(integer, coefficient), func(t *testing.T) {
				m, c, g, services := solveEqFixture(t)
				runner, err := NewSolveEquationsTactic(m, c, g, nil, services)
				simplifyMust(t, err)
				t.Cleanup(func() { simplifyMust(t, runner.Close()) })
				numeral := func(text string) ast.ASTHandle {
					var term ast.ASTHandle
					var err error
					if integer {
						term, err = m.ArithmeticIntegerNumeralText(text)
					} else {
						term, err = m.ArithmeticRealNumeralText(text)
					}
					simplifyMust(t, err)
					simplifyPin(t, m, term)
					return term
				}
				coef, one, three := numeral(coefficient), numeral("1"), numeral("3")
				sort, err := m.GetSort(one)
				simplifyMust(t, err)
				name, err := m.Symbols.Intern("x")
				simplifyMust(t, err)
				x, err := m.MkConst(name, sort)
				simplifyMust(t, err)
				simplifyPin(t, m, x)
				product, err := m.MkArithmeticMul(coef, x)
				simplifyMust(t, err)
				sum, err := m.MkArithmeticAdd(product, one)
				simplifyMust(t, err)
				equation, err := m.MkEq(sum, three)
				simplifyMust(t, err)
				goal, err := tactic.NewGoal(m, false, true, false)
				simplifyMust(t, err)
				t.Cleanup(func() { simplifyMust(t, goal.Close()) })
				simplifyMust(t, goal.Assert(equation))
				var result tactic.TacticResult
				t.Cleanup(func() { simplifyMust(t, result.Close()) })
				simplifyMust(t, runner.Apply(goal, &result))
				if integer && coefficient == "2" {
					if goal.IsDecidedSat() || runner.eliminated != 0 {
						t.Fatal("integer non-unit coefficient was eliminated")
					}
					return
				}
				if !goal.IsDecidedSat() || runner.eliminated != 1 {
					t.Fatal("eligible arithmetic variable was not isolated", runner.eliminated)
				}
				var md tactic.ModelRef
				t.Cleanup(func() { simplifyMust(t, md.Close()) })
				simplifyMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &md))
				model, err := md.Get()
				simplifyMust(t, err)
				decl, err := m.GetDecl(x)
				simplifyMust(t, err)
				value, err := model.ConstantInterpretation(decl)
				simplifyMust(t, err)
				text, _, matched, err := m.ArithmeticNumeralText(value)
				simplifyMust(t, err)
				want := "1"
				if coefficient == "-1" {
					want = "-2"
				}
				if !matched || text != want {
					t.Fatal("isolated arithmetic model value", text, want)
				}
			})
		}
	}
}

func TestSolveEquationsProofAndTransitiveCoreSurviveCleanup(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	runner, err := NewSolveEquationsTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a, b, notB := propagateAtoms(t, m)
	equality, err := m.MkEq(a, b)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, true, true, true)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	var tags []ast.ASTHandle
	boolean, err := m.MkBoolSort()
	simplifyMust(t, err)
	for i, term := range []ast.ASTHandle{equality, a, notB} {
		name, err := m.Symbols.Intern(fmt.Sprintf("solve-tag-%d", i))
		simplifyMust(t, err)
		tag, err := m.MkConst(name, boolean)
		simplifyMust(t, err)
		simplifyPin(t, m, tag)
		tags = append(tags, tag)
		dep, err := m.DependencyLeaf(tag)
		simplifyMust(t, err)
		proof, err := m.MkProofAsserted(term)
		simplifyMust(t, err)
		simplifyMust(t, goal.AssertWithProof(term, proof, dep))
	}
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if !goal.IsDecidedUnsat() {
		t.Fatal("definition substitution did not derive the contradiction")
	}
	simplifyMust(t, runner.Cleanup())
	proof, err := goal.Proof(0)
	simplifyMust(t, err)
	fact, err := m.GetProofFact(proof)
	simplifyMust(t, err)
	if !m.IsFalse(fact) {
		t.Fatal("elimination proof does not conclude false")
	}
	dep, err := goal.Dependency(0)
	simplifyMust(t, err)
	leaves, err := m.DependencyLinearize(dep, nil)
	simplifyMust(t, err)
	if len(leaves) != 3 {
		t.Fatal("elimination lost a definition dependency", leaves)
	}
	for _, want := range tags {
		found := false
		for _, got := range leaves {
			found = found || got == want
		}
		if !found {
			t.Fatal("missing core tag", want)
		}
	}
	if result.ModelConverter != nil || result.ProofConverter != nil {
		t.Fatal("UNSAT retained a model or introduced a separate proof converter")
	}
}

func TestSolveEquationsOccurrenceLimitAndLiteralException(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	params, err := c.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, c.Del(&params)) })
	key := "solve_eqs_max_occs"
	simplifyMust(t, c.SetUIntName(&params, &key, 0))
	runner, err := NewSolveEquationsTactic(m, c, g, &params, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a, b, _ := propagateAtoms(t, m)
	equality, err := m.MkEq(a, b)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, false, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(equality))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if goal.IsDecidedSat() || runner.eliminated != 0 {
		t.Fatal("zero occurrence limit eliminated an equality variable")
	}
	simplifyMust(t, goal.Reset())
	simplifyMust(t, goal.Assert(a))
	simplifyMust(t, runner.Cleanup())
	simplifyMust(t, runner.Apply(goal, &result))
	if !goal.IsDecidedSat() || runner.eliminated != 1 {
		t.Fatal("native literal solver incorrectly used the Gaussian occurrence limit")
	}
}

func TestSolveEquationsITEOrientationsAndPolicy(t *testing.T) {
	for orientation := 0; orientation < 4; orientation++ {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprint(orientation, enabled), func(t *testing.T) {
				m, c, g, services := solveEqFixture(t)
				params, err := c.NewValue()
				simplifyMust(t, err)
				t.Cleanup(func() { simplifyMust(t, c.Del(&params)) })
				key := "ite_solver"
				simplifyMust(t, c.SetBoolName(&params, &key, enabled))
				runner, err := NewSolveEquationsTactic(m, c, g, &params, services)
				simplifyMust(t, err)
				t.Cleanup(func() { simplifyMust(t, runner.Close()) })
				condition, _, _ := propagateAtoms(t, m)
				two, err := m.ArithmeticIntegerNumeralText("2")
				simplifyMust(t, err)
				simplifyPin(t, m, two)
				three, err := m.ArithmeticIntegerNumeralText("3")
				simplifyMust(t, err)
				simplifyPin(t, m, three)
				sort, err := m.GetSort(two)
				simplifyMust(t, err)
				name, err := m.Symbols.Intern("x")
				simplifyMust(t, err)
				x, err := m.MkConst(name, sort)
				simplifyMust(t, err)
				simplifyPin(t, m, x)
				leftArgs := [2]ast.ASTHandle{x, two}
				rightArgs := [2]ast.ASTHandle{x, three}
				if orientation&1 != 0 {
					leftArgs[0], leftArgs[1] = leftArgs[1], leftArgs[0]
				}
				if orientation&2 != 0 {
					rightArgs[0], rightArgs[1] = rightArgs[1], rightArgs[0]
				}
				left, err := m.MkEq(leftArgs[0], leftArgs[1])
				simplifyMust(t, err)
				right, err := m.MkEq(rightArgs[0], rightArgs[1])
				simplifyMust(t, err)
				formula, err := m.MkIte(condition, left, right)
				simplifyMust(t, err)
				goal, err := tactic.NewGoal(m, false, true, false)
				simplifyMust(t, err)
				t.Cleanup(func() { simplifyMust(t, goal.Close()) })
				simplifyMust(t, goal.Assert(formula))
				var result tactic.TacticResult
				t.Cleanup(func() { simplifyMust(t, result.Close()) })
				simplifyMust(t, runner.Apply(goal, &result))
				if !enabled {
					if goal.IsDecidedSat() || runner.eliminated != 0 {
						t.Fatal("disabled ITE solver eliminated a variable")
					}
					return
				}
				if !goal.IsDecidedSat() || runner.eliminated != 1 {
					t.Fatal("eligible ITE definition was not eliminated")
				}
				var md tactic.ModelRef
				t.Cleanup(func() { simplifyMust(t, md.Close()) })
				simplifyMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &md))
				model, err := md.Get()
				simplifyMust(t, err)
				decl, err := m.GetDecl(x)
				simplifyMust(t, err)
				value, err := model.ConstantInterpretation(decl)
				simplifyMust(t, err)
				text, _, found, err := m.ArithmeticNumeralText(value)
				simplifyMust(t, err)
				if !found || text != "3" {
					t.Fatal("ITE model did not reconstruct the default-false condition branch", text)
				}
			})
		}
	}
}
