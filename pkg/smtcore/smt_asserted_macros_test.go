package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtAssertedMacroFixture(t *testing.T, proofs, quasi bool) (*ast.ASTManager, *SMTSolver, model.ModelEvaluatorConfig) {
	t.Helper()
	return smtAssertedMacroFixtureWithConfig(t, proofs, quasi, SMTSolverConfig{})
}

func smtAssertedMacroFixtureWithConfig(t *testing.T, proofs, quasi bool, solverConfig SMTSolverConfig) (*ast.ASTManager, *SMTSolver, model.ModelEvaluatorConfig) {
	t.Helper()
	m, config := smtModelGeneratorFixture(t)
	if proofs {
		astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	}
	local, err := config.Context.NewValue()
	astTestMust(t, err)
	macroFinder, propagate := "macro_finder", "propagate_values"
	astTestMust(t, config.Context.SetBoolName(&local, &macroFinder, true))
	astTestMust(t, config.Context.SetBoolName(&local, &propagate, false))
	quasiMacros := "quasi_macros"
	astTestMust(t, config.Context.SetBoolName(&local, &quasiMacros, quasi))
	solverConfig.Params, solverConfig.ModelEvaluator = config.Context, config
	solver, err := NewSMTSolver(m, &local, "AUFLIA", solverConfig)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()); astTestMust(t, config.Context.Del(&local)) })
	astTestMust(t, solver.SetProduceModels(true))
	return m, solver, config
}

func TestSMTAssertedMacrosExpandAcrossScopesAndPublishModels(t *testing.T) {
	t.Run("Full", func(t *testing.T) { checkAssertedMacroScopes(t, SMTSolverConfig{}) })
	t.Run("FirstVerificationPush", func(t *testing.T) { checkAssertedMacroScopes(t, SMTSolverConfig{SimplifyInitialPushOnce: true}) })
	t.Run("PrivateRefutation", func(t *testing.T) { checkAssertedMacroScopes(t, SMTSolverConfig{SkipRepeatedPreprocessing: true}) })
}

func checkAssertedMacroScopes(t *testing.T, cfg SMTSolverConfig) {
	t.Helper()
	m, solver, config := smtAssertedMacroFixtureWithConfig(t, false, false, cfg)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integer, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	declare := func(name string) ast.ASTHandle {
		f, err := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		ref, err := m.NewRef(f)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
		return f
	}
	f, g := declare("asserted-macro-f"), declare("asserted-macro-g")
	fApp, err := m.MkApp(f, []ast.ASTHandle{x})
	astTestMust(t, err)
	define := func(function, definition ast.ASTHandle, name string) {
		head, err := m.MkApp(function, []ast.ASTHandle{x})
		astTestMust(t, err)
		body, err := m.MkEq(head, definition)
		astTestMust(t, err)
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name)}, Body: body})
		astTestMust(t, err)
		astTestMust(t, solver.Assert(q))
	}
	xPlusOne, err := m.MkArithmeticAdd(x, one)
	astTestMust(t, err)
	define(f, xPlusOne, "asserted-macro-x")
	if cfg.SimplifyInitialPushOnce {
		// Commit the definition in a fresh base, then test the ordinary
		// nested-scope and model paths using that same solver.
		astTestMust(t, solver.Push())
		if solver.kernel.skipRepeatedPreprocessing || solver.kernel.reduceIfSharingChanged || solver.kernel.assertedMacros.NumMacros() != 1 {
			t.Fatal("initial push lost the macro or leaked the shortened schedule")
		}
		if !ast.ASTSourceDebug && solver.kernel.simplifyInitialPushOnce {
			t.Fatal("initial push did not consume the first-batch option")
		}
		astTestMust(t, solver.Pop(1))
	}

	check := func(wantMacros int) {
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True || solver.kernel.assertedMacros.NumMacros() != wantMacros || !solver.kernel.internalizer.quantifiers.Empty() {
			t.Fatal("macro definition reached quantifier search or was lost", status, solver.kernel.assertedMacros.NumMacros())
		}
	}
	evaluate := func(function ast.ASTHandle, expected string) {
		generated, err := solver.AcquireModel()
		astTestMust(t, err)
		if generated == nil {
			t.Fatal("missing generated macro model")
		}
		defer func() { astTestMust(t, generated.DecRef()) }()
		seven, err := m.ArithmeticIntegerNumeralText("7")
		astTestMust(t, err)
		application, err := m.MkApp(function, []ast.ASTHandle{seven})
		astTestMust(t, err)
		out, err := m.NewRef(ast.ASTHandle{})
		astTestMust(t, err)
		defer func() { astTestMust(t, m.DelRef(&out)) }()
		evaluator, err := model.NewModelEvaluator(generated, config)
		astTestMust(t, err)
		defer func() { astTestMust(t, evaluator.Close()) }()
		astTestMust(t, evaluator.SetModelCompletion(true))
		astTestMust(t, evaluator.Evaluate(application, out))
		value, err := m.RefGet(out)
		astTestMust(t, err)
		text, integer, matched, err := m.ArithmeticNumeralText(value)
		astTestMust(t, err)
		if !matched || !integer || text != expected {
			t.Fatal("macro interpretation missing from generated model", text, integer, matched, expected)
		}
	}
	check(1)
	evaluate(f, "8")
	astTestMust(t, solver.Push())
	fPlusOne, err := m.MkArithmeticAdd(fApp, one)
	astTestMust(t, err)
	define(g, fPlusOne, "asserted-macro-inner-x")
	check(2)
	evaluate(f, "8")
	evaluate(g, "9")
	astTestMust(t, solver.Pop(1))
	check(1)
	evaluate(f, "8")
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	interpretation, err := generated.FunctionInterpretation(g)
	astTestMust(t, err)
	astTestMust(t, generated.DecRef())
	if interpretation != nil {
		t.Fatal("popped macro survived in the published model")
	}
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	// Previously extracted definitions remain active when discovery and
	// ordinary preprocessing are disabled for subsequent assertions.
	params, err := config.Context.NewValue()
	astTestMust(t, err)
	for _, name := range []string{"macro_finder", "preprocess"} {
		astTestMust(t, config.Context.SetBoolName(&params, &name, false))
	}
	astTestMust(t, solver.UpdateParams(&params))
	astTestMust(t, config.Context.Del(&params))
	four, err := m.ArithmeticIntegerNumeralText("4")
	astTestMust(t, err)
	application, err := m.MkApp(f, []ast.ASTHandle{three})
	astTestMust(t, err)
	equal, err := m.MkEq(application, four)
	astTestMust(t, err)
	unequal, err := m.MkNot(equal)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(unequal))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("later ground assertion did not expand an existing macro", status)
	}
}

func TestSMTAssertedMacrosCommittedDeclarationsAndScopeRollback(t *testing.T) {
	m, solver, _ := smtAssertedMacroFixture(t, false, false)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "committed-macro-f"), []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	ref, err := m.NewRef(f)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	head, err := m.MkApp(f, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := m.MkEq(head, x)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "committed-macro-x")}, Body: body})
	astTestMust(t, err)
	qRef, err := m.NewRef(q)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&qRef)) })
	astTestMust(t, solver.Push())
	ground, err := m.MkApp(f, []ast.ASTHandle{zero})
	astTestMust(t, err)
	eq, err := m.MkEq(ground, zero)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(eq))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("ground formula failed before macro discovery", status)
	}
	astTestMust(t, solver.Assert(q))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || solver.kernel.assertedMacros.NumMacros() != 0 || solver.kernel.internalizer.quantifiers.Empty() {
		t.Fatal("committed declaration was incorrectly extracted as a macro", status, solver.kernel.assertedMacros.NumMacros())
	}
	astTestMust(t, solver.Pop(1))
	astTestMust(t, solver.Assert(q))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || solver.kernel.assertedMacros.NumMacros() != 1 || !solver.kernel.internalizer.quantifiers.Empty() {
		t.Fatal("popped declaration remained forbidden for macro discovery", status, solver.kernel.assertedMacros.NumMacros())
	}
}

func TestSMTAssertedMacroExpansionPreservesUnsatProof(t *testing.T) {
	m, solver, _ := smtAssertedMacroFixture(t, true, false)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "proof-macro-p", boolean, boolean)
	head, err := m.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	body, err := m.MkIff(head, x)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "proof-macro-x")}, Body: body})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(q))
	no, err := m.MkFalse()
	astTestMust(t, err)
	contradiction, err := m.MkApp(p, []ast.ASTHandle{no})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(contradiction))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False || solver.kernel.assertedMacros.NumMacros() != 1 {
		t.Fatal("macro expansion did not expose the contradiction", status)
	}
	proof, err := solver.Proof()
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("macro-expanded contradiction lost its proof")
	}
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if !m.IsFalse(fact) {
		t.Fatal("expanded proof does not establish false", fact)
	}
}

func TestSMTAssertedMacroPublishesPermutedArguments(t *testing.T) {
	m, solver, config := smtAssertedMacroFixture(t, false, false)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	integer, err := m.GetSort(three)
	astTestMust(t, err)
	x, err := m.MkVar(1, integer)
	astTestMust(t, err)
	y, err := m.MkVar(0, integer)
	astTestMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "permuted-macro-f"), []ast.ASTHandle{integer, integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	head, err := m.MkApp(f, []ast.ASTHandle{y, x})
	astTestMust(t, err)
	difference, err := m.MkArithmeticSub(x, y)
	astTestMust(t, err)
	body, err := m.MkEq(head, difference)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "permuted-x"), astTestSymbol(t, m, "permuted-y")}, Body: body})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(q))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True || solver.kernel.assertedMacros.NumMacros() != 1 {
		t.Fatal("permuted macro was not extracted", status)
	}
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	defer func() { astTestMust(t, generated.DecRef()) }()
	evaluator, err := model.NewModelEvaluator(generated, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, evaluator.Close()) }()
	application, err := m.MkApp(f, []ast.ASTHandle{three, seven})
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	astTestMust(t, evaluator.Evaluate(application, out))
	value, err := m.RefGet(out)
	astTestMust(t, err)
	text, _, matched, err := m.ArithmeticNumeralText(value)
	astTestMust(t, err)
	if !matched || text != "4" {
		t.Fatal("macro interpretation did not normalize its argument permutation", text, matched)
	}
}
