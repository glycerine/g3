package interp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestFullTranslationArithmeticHelpers(t *testing.T) {
	_, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("translation-x", integer)
	interpMust(t, err)
	y, err := manager.MakeVariable("translation-y", integer)
	interpMust(t, err)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	three, err := manager.MakeIntText("3")
	interpMust(t, err)
	right, err := manager.Make(OperatorPlus, three, y)
	interpMust(t, err)
	inequality, err := manager.Make(OperatorLE, x, right)
	interpMust(t, err)
	normalized, err := translation.NormalizeInequality(inequality)
	interpMust(t, err)
	normalizedRight, err := manager.Arg(normalized, 1)
	interpMust(t, err)
	value, numeral, err := manager.Numeral(normalizedRight)
	if err != nil || !numeral || value.Cmp(new(big.Rat).SetInt64(3)) != 0 {
		t.Fatal("normalized affine right side", value, numeral, err)
	}
	valid, err := translation.CheckFarkas([]ast.ASTHandle{inequality}, inequality)
	if err != nil || !valid {
		t.Fatal("Farkas identity", valid, err)
	}
	two, err := manager.MakeIntText("2")
	interpMust(t, err)
	four, err := manager.MakeIntText("4")
	interpMust(t, err)
	twoX, err := manager.Make(OperatorTimes, two, x)
	interpMust(t, err)
	fourY, err := manager.Make(OperatorTimes, four, y)
	interpMust(t, err)
	affine, err := manager.Make(OperatorPlus, twoX, fourY)
	interpMust(t, err)
	gcd, err := translation.GCDOfCoefficients(affine)
	interpMust(t, err)
	if value, numeral, e := manager.Numeral(gcd); e != nil || !numeral || value.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("coefficient gcd", value, numeral, e)
	}
	boundInequality, err := manager.Make(OperatorLE, x, three)
	interpMust(t, err)
	bound, lower, err := translation.BoundedVariable(boundInequality)
	if err != nil || bound != x || lower {
		t.Fatal("bounded variable", bound, lower, err)
	}
	coefficient, err := translation.TermCoefficient(normalized, x)
	if err != nil || coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("term coefficient", coefficient, err)
	}
	ratio, err := translation.DivideInequalities(normalized, normalized)
	interpMust(t, err)
	if value, numeral, e := manager.Numeral(ratio); e != nil || !numeral || value.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("inequality ratio", value, numeral, e)
	}
}

func TestFullTranslationTriangleEqualityTheoryLemma(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("translation-triangle-x", integer)
	interpMust(t, err)
	y, err := manager.MakeVariable("translation-triangle-y", integer)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	xFrame, err := manager.Make(OperatorLE, x, zero)
	interpMust(t, err)
	yFrame, err := manager.Make(OperatorLE, y, zero)
	interpMust(t, err)
	base, err := NewBase(manager, []ast.ASTHandle{xFrame, yFrame}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	equality, err := manager.Make(OperatorEqual, x, y)
	interpMust(t, err)
	xLeqY, err := manager.Make(OperatorLE, x, y)
	interpMust(t, err)
	yLeqX, err := manager.Make(OperatorLE, y, x)
	interpMust(t, err)
	notXLeqY, err := manager.MkNot(xLeqY)
	interpMust(t, err)
	notYLeqX, err := manager.MkNot(yLeqX)
	interpMust(t, err)
	conclusion, err := manager.Make(OperatorOr, equality, notXLeqY, notYLeqX)
	interpMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	interpMust(t, err)
	lemma := interpTheoryLemma(t, m, arithmetic, "triangle-eq", conclusion, nil)
	destination, err := NewInterpolationProof(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, destination.Close()) })
	node, err := translation.Translate(lemma, destination)
	interpMust(t, err)
	interpolant, err := destination.Interpolate(node, DownwardScopeRange(0), false)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
}

func TestFullTranslationAssignBoundsTheoryLemma(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("translation-bounds-x", integer)
	interpMust(t, err)
	y, err := manager.MakeVariable("translation-bounds-y", integer)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	le, err := manager.Make(OperatorLE, x, zero)
	interpMust(t, err)
	ge, err := manager.Make(OperatorGE, x, one)
	interpMust(t, err)
	yFrame, err := manager.Make(OperatorLE, y, zero)
	interpMust(t, err)
	base, err := NewBase(manager, []ast.ASTHandle{le, yFrame}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	clause, err := manager.Make(OperatorOr, le, ge)
	interpMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	interpMust(t, err)
	lemma := interpTheoryLemma(t, m, arithmetic, "assign-bounds", clause, nil, "-1/2")
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	result, err := translation.translateAssignBounds(lemma, clause, nil, nil, direct)
	interpMust(t, err)
	interpolant, err := direct.Interpolate(result)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
}

func TestFullTranslationGCDTheoryLemma(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("translation-gcd-x", integer)
	interpMust(t, err)
	y, err := manager.MakeVariable("translation-gcd-y", integer)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	upper, err := manager.Make(OperatorLE, x, zero)
	interpMust(t, err)
	lower, err := manager.Make(OperatorGE, x, one)
	interpMust(t, err)
	yFrame, err := manager.Make(OperatorLE, y, zero)
	interpMust(t, err)
	base, err := NewBase(manager, []ast.ASTHandle{upper, yFrame}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	upperProof, err := m.MkProofAsserted(upper)
	interpMust(t, err)
	lowerProof, err := m.MkProofAsserted(lower)
	interpMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	interpMust(t, err)
	falsehood, err := manager.Make(OperatorFalse)
	interpMust(t, err)
	lemma := interpTheoryLemma(t, m, arithmetic, "gcd-test", falsehood, []ast.ASTHandle{upperProof, lowerProof}, "1", "1")
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	first, err := direct.MakeHypothesis(upper)
	interpMust(t, err)
	second, err := direct.MakeHypothesis(lower)
	interpMust(t, err)
	result, err := translation.translateGCDTest(lemma, []ast.ASTHandle{first, second}, direct)
	interpMust(t, err)
	interpolant, err := direct.Interpolate(result)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
}

func TestFullTranslationReconstructsEqualityPropagationWithDual(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("translation-eq-propagate-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("translation-eq-propagate-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	xLeqY, err := manager.Make(OperatorLE, x, y)
	interpMust(t, err)
	xLeqY = interpPin(t, m, xLeqY)
	xGeqY, err := manager.Make(OperatorGE, x, y)
	interpMust(t, err)
	xGeqY = interpPin(t, m, xGeqY)
	base, err := NewBase(manager, []ast.ASTHandle{xLeqY, xGeqY}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	one, err := manager.MakeRealText("1")
	interpMust(t, err)
	one = interpPin(t, m, one)
	proofFact, err := m.MkProofAsserted(xLeqY)
	interpMust(t, err)
	proofFact = interpPin(t, m, proofFact)
	call := 0
	factory := solver.FactoryFunc(func(got *ast.ASTManager, _ *uparams.ParamsRef, _, _, _ bool, _ string) (solver.Solver, error) {
		status := lbool.False
		if call%5 == 4 {
			status = lbool.True
		}
		call++
		s, e := newInterpolationScriptSolver(got, status)
		if e != nil {
			return nil, e
		}
		if status == lbool.False {
			if e = s.SetProof(proofFact); e != nil {
				return nil, e
			}
		} else {
			md, modelErr := model.NewModelCore(got)
			if modelErr != nil {
				return nil, modelErr
			}
			if e = s.SetModel(md); e != nil {
				return nil, e
			}
		}
		return s, nil
	})
	evaluator := func(*model.ModelCore) (solver.ConsequenceEvaluator, error) {
		return &interpolationFixedEvaluator{manager: m, value: one}, nil
	}
	interpMust(t, base.ConfigureSolver(factory, evaluator))
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	source := make([]ast.ASTHandle, 2)
	source[0], err = m.MkProofHypothesis(xLeqY)
	interpMust(t, err)
	source[0] = interpPin(t, m, source[0])
	source[1], err = m.MkProofHypothesis(xGeqY)
	interpMust(t, err)
	source[1] = interpPin(t, m, source[1])
	translated := make([]ast.ASTHandle, 2)
	translated[0], err = direct.MakeHypothesis(xLeqY)
	interpMust(t, err)
	translated[1], err = direct.MakeHypothesis(xGeqY)
	interpMust(t, err)
	equality, err := manager.Make(OperatorEqual, x, y)
	interpMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	interpMust(t, err)
	lemma := interpTheoryLemma(t, m, arithmetic, "eq-propagate", equality, source)
	result, err := translation.translateTheoryLemma(lemma, equality, translated, []ast.ASTHandle{xLeqY, xGeqY}, direct)
	if err != nil {
		t.Fatal("equality propagation", err, "solver calls", call)
	}
	interpolant, err := direct.Interpolate(result)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
	if call != 10 {
		t.Fatal("dual reconstruction solver calls", call)
	}
	reflexivity, err := m.MkProofReflexivity(x)
	interpMust(t, err)
	reflexivity = interpPin(t, m, reflexivity)
	chain, err := m.MkProofTransitivity(reflexivity, lemma)
	interpMust(t, err)
	chain = interpPin(t, m, chain)
	combined, err := translation.combineEqualityPropagation(chain, direct, newTranslationMemo())
	interpMust(t, err)
	combinedInterpolant, err := direct.Interpolate(combined)
	interpMust(t, err)
	interpMust(t, m.DelRef(&combinedInterpolant))
	if call != 20 {
		t.Fatal("combined equality propagation solver calls", call)
	}
}

func TestFullTranslationReconstructsMysteryArithmeticFromProof(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("translation-mystery-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("translation-mystery-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	inequality, err := manager.Make(OperatorLE, x, y)
	interpMust(t, err)
	inequality = interpPin(t, m, inequality)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	source, err := m.MkProofHypothesis(inequality)
	interpMust(t, err)
	source = interpPin(t, m, source)
	normalized, err := translation.reallyNormalizeInequality(inequality)
	interpMust(t, err)
	negated, err := manager.MkNot(inequality)
	interpMust(t, err)
	normalizedNegation, err := translation.reallyNormalizeInequality(negated)
	interpMust(t, err)
	first, err := m.MkProofHypothesis(normalized)
	interpMust(t, err)
	first = interpPin(t, m, first)
	second, err := m.MkProofHypothesis(normalizedNegation)
	interpMust(t, err)
	second = interpPin(t, m, second)
	falsehood, err := manager.Make(OperatorFalse)
	interpMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	interpMust(t, err)
	certificate := interpTheoryLemma(t, m, arithmetic, "farkas", falsehood, []ast.ASTHandle{first, second}, "1", "1")
	checks := 0
	factory := solver.FactoryFunc(func(got *ast.ASTManager, _ *uparams.ParamsRef, _, _, _ bool, _ string) (solver.Solver, error) {
		checks++
		s, e := newInterpolationScriptSolver(got, lbool.False)
		if e != nil {
			return nil, e
		}
		if e = s.SetProof(certificate); e != nil {
			return nil, e
		}
		return s, nil
	})
	interpMust(t, base.ConfigureSolver(factory, nil))
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	translated, err := direct.MakeHypothesis(inequality)
	interpMust(t, err)
	mystery, err := m.MkProofTheoryLemma(arithmetic, inequality, []ast.ASTHandle{source}, nil)
	interpMust(t, err)
	mystery = interpPin(t, m, mystery)
	result, err := translation.translateTheoryLemma(mystery, inequality, []ast.ASTHandle{translated}, []ast.ASTHandle{inequality}, direct)
	interpMust(t, err)
	interpolant, err := direct.Interpolate(result)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
	if checks != 1 {
		t.Fatal("normalized proof reconstruction checks", checks)
	}
}

func interpRewritingManagerFixture(t *testing.T) (*ast.ASTManager, *Manager) {
	t.Helper()
	m := interpASTFixture(t)
	interpMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	interpMust(t, m.RegisterDeclarationPlugins())
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	interpMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	interpMust(t, err)
	globals, err := uparams.NewGlobalParamsService(m.Symbols, context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	interpMust(t, err)
	interpMust(t, globals.InitializeSource())
	t.Cleanup(func() {
		interpMust(t, globals.Close())
		interpMust(t, catalog.Close())
		interpMust(t, context.Close())
	})
	w, err := NewManagerWithParams(m, context, globals)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, w.Close()) })
	return m, w
}

func TestInterpolationManagerTheorySimplifiers(t *testing.T) {
	m, w := interpRewritingManagerFixture(t)
	intSort, err := w.IntType()
	interpMust(t, err)
	x, err := w.MakeVariable("rewrite-x", intSort)
	interpMust(t, err)
	x = interpPin(t, m, x)
	zero, err := w.MakeIntText("0")
	interpMust(t, err)
	sum, err := w.Make(OperatorPlus, x, zero)
	interpMust(t, err)
	simplified, err := w.Simplify(sum)
	interpMust(t, err)
	defer func() { interpMust(t, m.DelRef(&simplified)) }()
	if got, e := m.RefGet(simplified); e != nil || got != x {
		t.Fatal("default simplifier", got, e)
	}

	one, err := w.MakeIntText("1")
	interpMust(t, err)
	two, err := w.MakeIntText("2")
	interpMust(t, err)
	left, err := w.Make(OperatorPlus, x, one)
	interpMust(t, err)
	right, err := w.Make(OperatorPlus, two, x)
	interpMust(t, err)
	polynomial, err := w.Make(OperatorPlus, left, right)
	interpMust(t, err)
	polynomial = interpPin(t, m, polynomial)
	// AUDIT.md interp_02-F2: every iZ3 call site uses z3_simplify (empty
	// params); the result is the same as a second Simplify of the input.
	polyRef, err := w.Simplify(polynomial)
	interpMust(t, err)
	defer func() { interpMust(t, m.DelRef(&polyRef)) }()
	poly, err := m.RefGet(polyRef)
	interpMust(t, err)
	if op, e := w.Operator(poly); e != nil || op != OperatorPlus {
		t.Fatal("polynomial simplifier", op, e)
	}
	againRef, err := w.Simplify(polynomial)
	interpMust(t, err)
	defer func() { interpMust(t, m.DelRef(&againRef)) }()
	if again, e := m.RefGet(againRef); e != nil || again != poly {
		t.Fatal("simplifier is not deterministic", again, poly, e)
	}
}

func TestInterpolationManagerSimplifierConfigurationGuard(t *testing.T) {
	_, w := interpManagerFixture(t)
	truth, err := w.Make(OperatorTrue)
	interpMust(t, err)
	if _, err = w.Simplify(truth); err == nil || err.Error() != "interpolation manager has no theory rewriter" {
		t.Fatal(err)
	}
}
