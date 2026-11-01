package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestHornPreprocessingCarriesRequestedRuleProofs(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	fragmentMust(t, globals.SourceSetName("fixedpoint.xform.slice", "false"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine("qbmc"))
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		s, e := manager.Symbols.Intern(name)
		fragmentMust(t, e)
		d, e := manager.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, e)
		fragmentMust(t, engine.RegisterPredicate(d))
		h, e := manager.MkApp(d, nil)
		fragmentMust(t, e)
		return d, h
	}
	_, fact := declare("proof-preprocess-fact")
	goal, goalApp := declare("proof-preprocess-goal")
	fragmentMust(t, engine.AddRule(fact, "fact"))
	step, err := manager.MkImplies(fact, goalApp)
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(step, "step"))
	fragmentMust(t, engine.validateFragment())
	prepared, err := engine.preprocessHornRules(goal, true, true)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, prepared.Close()) }()
	group, err := prepared.rules.PredicateRules(goal)
	fragmentMust(t, err)
	defer func() {
		for i := range group {
			fragmentMust(t, group[i].Close())
		}
	}()
	if len(group) == 0 {
		t.Fatal("preprocessing lost the goal rule")
	}
	for _, rule := range group {
		proof, found, e := rule.Proof()
		fragmentMust(t, e)
		if !found {
			t.Fatal("preprocessed goal rule lost its proof")
		}
		formula, e := rule.ToFormula()
		fragmentMust(t, e)
		value, e := manager.RefGet(formula)
		fragmentMust(t, e)
		fact, e := manager.GetProofFact(proof)
		fragmentMust(t, e)
		fragmentMust(t, manager.DelRef(&formula))
		if value != fact {
			t.Fatal("preprocessed proof concludes a different rule")
		}
	}
}

func TestHornPreprocessingKeepsOriginalPredicatesForProofTraces(t *testing.T) {
	for _, mode := range []string{"disabled", "parameter", "forced"} {
		t.Run(mode, func(t *testing.T) {
			manager, globals := newPDRLifecycleServices(t)
			for _, setting := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
				fragmentMust(t, globals.SourceSetName(setting, "false"))
			}
			if mode == "parameter" {
				fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
			}
			engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, engine.Close()) }()
			engine.forceProofTrace = mode == "forced"
			fragmentMust(t, engine.SelectEngine("qbmc"))
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			zero, err := manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
			fragmentMust(t, err)
			integer, err := manager.GetSort(zero)
			fragmentMust(t, err)
			declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				symbol, e := manager.Symbols.Intern(name)
				fragmentMust(t, e)
				decl, e := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, e)
				fragmentMust(t, engine.RegisterPredicate(decl))
				return decl
			}
			p := declare("proof-slice-P", []ast.ASTHandle{integer})
			goal := declare("proof-slice-goal", nil)
			variable, err := manager.MkVar(0, integer)
			fragmentMust(t, err)
			pApp, err := manager.MkApp(p, []ast.ASTHandle{variable})
			fragmentMust(t, err)
			goalApp, err := manager.MkApp(goal, nil)
			fragmentMust(t, err)
			for _, spec := range []struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
			}{{pApp, nil}, {goalApp, []ast.ASTHandle{pApp}}} {
				rule, e := engine.rules.NewRule(spec.head, spec.tails, nil, "")
				fragmentMust(t, e)
				fragmentMust(t, engine.set.Add(rule))
				fragmentMust(t, rule.Close())
			}
			fragmentMust(t, engine.validateFragment())
			prepared, err := engine.preprocessHornRules(goal, true, true)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, prepared.Close()) }()
			_, sliced := prepared.slicedPredicates[p]
			if sliced != (mode == "disabled") {
				t.Fatalf("predicate sliced = %v, mode %s", sliced, mode)
			}
			if mode != "disabled" {
				count, e := prepared.rules.Len()
				fragmentMust(t, e)
				if count != 2 {
					t.Fatalf("got %d rules", count)
				}
				for i := 0; i < count; i++ {
					rule, e := prepared.rules.Rule(i)
					fragmentMust(t, e)
					_, found, e := rule.Proof()
					fragmentMust(t, e)
					if !found {
						t.Fatal("rule lacks requested proof")
					}
					fragmentMust(t, rule.Close())
				}
			}
		})
	}
}

func TestQBMCConvertsSlicedProofToOriginalRules(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	for _, setting := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, globals.SourceSetName(setting, "false"))
	}
	fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine("qbmc"))
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	zero, err := manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	fragmentMust(t, err)
	integer, err := manager.GetSort(zero)
	fragmentMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		s, e := manager.Symbols.Intern(name)
		fragmentMust(t, e)
		d, e := manager.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, e)
		fragmentMust(t, engine.RegisterPredicate(d))
		return d
	}
	p, goal := declare("qbmc-sliced-proof-P", []ast.ASTHandle{integer}), declare("qbmc-sliced-proof-goal", nil)
	x, err := manager.MkVar(0, integer)
	fragmentMust(t, err)
	pApp, err := manager.MkApp(p, []ast.ASTHandle{x})
	fragmentMust(t, err)
	goalApp, err := manager.MkApp(goal, nil)
	fragmentMust(t, err)
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
	}{{pApp, nil}, {goalApp, []ast.ASTHandle{pApp}}} {
		r, e := engine.rules.NewRule(spec.head, spec.tails, nil, "")
		fragmentMust(t, e)
		fragmentMust(t, engine.set.Add(r))
		fragmentMust(t, r.Close())
	}
	status, err := engine.queryBMCPredicateWithSliceProof(goal, true)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("query status", status)
	}
	foundSliced := false
	for _, r := range engine.bmcTrace {
		decl, e := r.HeadDeclaration()
		fragmentMust(t, e)
		if decl != goal && decl != p {
			foundSliced = true
		}
	}
	if !foundSliced {
		t.Fatal("query did not use a sliced predicate")
	}
	proof, err := manager.RefGet(engine.bmcProof)
	fragmentMust(t, err)
	fact, err := manager.GetProofFact(proof)
	fragmentMust(t, err)
	if fact != goalApp {
		t.Fatal("converted proof has the wrong conclusion")
	}
	checker, err := proofs.NewChecker(manager)
	fragmentMust(t, err)
	valid, side, err := checker.Check(proof)
	fragmentMust(t, err)
	defer func() {
		for _, condition := range side {
			fragmentMust(t, manager.DecRef(condition))
		}
	}()
	if !valid {
		t.Fatal("converted QBMC proof is invalid")
	}
	solver, err := smtcore.NewSMTSolver(manager, nil, "", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, solver.Close()) }()
	for _, condition := range side {
		fragmentMust(t, solver.Push())
		negated, e := manager.MkNot(condition)
		fragmentMust(t, e)
		fragmentMust(t, solver.AssertFormula(negated))
		checked, e := solver.CheckSat(nil)
		fragmentMust(t, e)
		fragmentMust(t, solver.Pop(1))
		if checked != lbool.False {
			t.Fatal("invalid converted QBMC proof side condition", checked)
		}
	}
}

func TestQBMCFallsBackFromUnsupportedSlicedProof(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	for _, setting := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, globals.SourceSetName(setting, "false"))
	}
	fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine("qbmc"))
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	zero, err := manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	fragmentMust(t, err)
	integer, err := manager.GetSort(zero)
	fragmentMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		s, e := manager.Symbols.Intern(name)
		fragmentMust(t, e)
		d, e := manager.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, e)
		fragmentMust(t, engine.RegisterPredicate(d))
		return d
	}
	p := declare("qbmc-fallback-P", []ast.ASTHandle{integer})
	q := declare("qbmc-fallback-Q", []ast.ASTHandle{integer})
	goal := declare("qbmc-fallback-goal", nil)
	x, err := manager.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := manager.MkVar(1, integer)
	fragmentMust(t, err)
	pApp, err := manager.MkApp(p, []ast.ASTHandle{x})
	fragmentMust(t, err)
	qApp, err := manager.MkApp(q, []ast.ASTHandle{y})
	fragmentMust(t, err)
	goalApp, err := manager.MkApp(goal, nil)
	fragmentMust(t, err)
	for _, term := range []ast.ASTHandle{pApp, qApp, goalApp} {
		ref, e := manager.NewRef(term)
		fragmentMust(t, e)
		defer func() { fragmentMust(t, manager.DelRef(&ref)) }()
	}
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
	}{{pApp, nil}, {qApp, nil}, {goalApp, []ast.ASTHandle{pApp, qApp}}} {
		r, e := engine.rules.NewRule(spec.head, spec.tails, nil, "")
		fragmentMust(t, e)
		fragmentMust(t, engine.set.Add(r))
		fragmentMust(t, r.Close())
	}
	_, err = engine.queryBMCPredicateWithSliceProof(goal, true)
	if !onlyErrorKind(err, errRetryUnslicedProof) {
		t.Fatal("expected unsliced retry", err)
	}
	status, err := engine.queryBMCPredicate(goal)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("fallback status", status)
	}
	for _, r := range engine.bmcTrace {
		decl, e := r.HeadDeclaration()
		fragmentMust(t, e)
		if decl != p && decl != q && decl != goal {
			t.Fatal("fallback trace contains a sliced predicate")
		}
	}
	proof, err := manager.RefGet(engine.bmcProof)
	fragmentMust(t, err)
	fact, err := manager.GetProofFact(proof)
	fragmentMust(t, err)
	if fact != goalApp {
		t.Fatal("nonlinear fallback proof has the wrong conclusion")
	}
	checker, err := proofs.NewChecker(manager)
	fragmentMust(t, err)
	valid, side, err := checker.Check(proof)
	fragmentMust(t, err)
	defer func() {
		for _, condition := range side {
			fragmentMust(t, manager.DecRef(condition))
		}
	}()
	if !valid {
		t.Fatal("nonlinear fallback proof is invalid")
	}
	solver, err := smtcore.NewSMTSolver(manager, nil, "", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, solver.Close()) }()
	for _, condition := range side {
		fragmentMust(t, solver.Push())
		negated, e := manager.MkNot(condition)
		fragmentMust(t, e)
		fragmentMust(t, solver.AssertFormula(negated))
		checked, e := solver.CheckSat(nil)
		fragmentMust(t, e)
		fragmentMust(t, solver.Pop(1))
		if checked != lbool.False {
			t.Fatal("invalid nonlinear fallback side condition", checked)
		}
	}
	if !onlyErrorKind(errors.Join(errRetryUnslicedProof, errRetryUnslicedProof), errRetryUnslicedProof) {
		t.Fatal("retry signal was lost")
	}
	if onlyErrorKind(errors.Join(errRetryUnslicedProof, errors.New("cleanup failed")), errRetryUnslicedProof) {
		t.Fatal("retry would hide a cleanup error")
	}
}
