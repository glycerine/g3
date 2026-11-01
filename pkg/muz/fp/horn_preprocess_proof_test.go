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

func TestQBMCConvertsNonlinearSlicedProof(t *testing.T) {
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
	status, err := engine.queryBMCPredicateWithSliceProof(goal, true)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("nonlinear sliced status", status)
	}
	foundSliced := false
	for _, r := range engine.bmcTrace {
		decl, e := r.HeadDeclaration()
		fragmentMust(t, e)
		if decl != p && decl != q && decl != goal {
			foundSliced = true
		}
	}
	if !foundSliced {
		t.Fatal("nonlinear search did not use sliced predicates")
	}
	proof, err := manager.RefGet(engine.bmcProof)
	fragmentMust(t, err)
	fact, err := manager.GetProofFact(proof)
	fragmentMust(t, err)
	if fact != goalApp {
		t.Fatal("nonlinear converted proof has the wrong conclusion")
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
		t.Fatal("nonlinear converted proof is invalid")
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
			t.Fatal("invalid nonlinear converted side condition", checked)
		}
	}
	if !onlyErrorKind(errors.Join(errRetryUnslicedProof, errRetryUnslicedProof), errRetryUnslicedProof) {
		t.Fatal("retry signal was lost")
	}
	if onlyErrorKind(errors.Join(errRetryUnslicedProof, errors.New("cleanup failed")), errRetryUnslicedProof) {
		t.Fatal("retry would hide a cleanup error")
	}
}

func TestQBMCRetriesWhenSliceResolutionMergesTails(t *testing.T) {
	testRetryWhenSliceResolutionMergesTails(t, "qbmc")
}

func TestPDRConvertsSharedStateBranches(t *testing.T) {
	testRetryWhenSliceResolutionMergesTails(t, "pdr")
}

func testRetryWhenSliceResolutionMergesTails(t *testing.T, backend string) {
	manager, globals := newPDRLifecycleServices(t)
	for _, setting := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, globals.SourceSetName(setting, "false"))
	}
	fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine(backend))
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
	p := declare("qbmc-merge-P", []ast.ASTHandle{integer})
	q := declare("qbmc-merge-Q", []ast.ASTHandle{integer})
	r := declare("qbmc-merge-R", []ast.ASTHandle{integer})
	goal := declare("qbmc-merge-goal", nil)
	x, err := manager.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := manager.MkVar(1, integer)
	fragmentMust(t, err)
	app := func(decl ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, e := manager.MkApp(decl, args)
		fragmentMust(t, e)
		ref, e := manager.NewRef(h)
		fragmentMust(t, e)
		deferFn := ref
		t.Cleanup(func() { fragmentMust(t, manager.DelRef(&deferFn)) })
		return h
	}
	p0, q0, ry, goalApp := app(p, zero), app(q, zero), app(r, y), app(goal)
	px, qx := app(p, x), app(q, x)
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
	}{{p0, nil}, {q0, nil}, {ry, nil}, {goalApp, []ast.ASTHandle{px, qx, q0, ry}}} {
		rule, e := engine.rules.NewRule(spec.head, spec.tails, nil, "")
		fragmentMust(t, e)
		fragmentMust(t, engine.set.Add(rule))
		fragmentMust(t, rule.Close())
	}
	if backend == "pdr" {
		_, err = engine.queryPDRWithSliceProof(goal, true)
	} else {
		_, err = engine.queryBMCPredicateWithSliceProof(goal, true)
	}
	if backend == "pdr" {
		fragmentMust(t, err)
	} else if !onlyErrorKind(err, errRetryUnslicedProof) {
		t.Fatal("expected retry after merged tails", err)
	}
	var status lbool.LBool
	if backend == "pdr" {
		status, err = engine.queryPDR(goal)
	} else {
		status, err = engine.queryBMCPredicate(goal)
	}
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("unsliced retry status", status)
	}
	var proofRef ast.ASTRef
	if backend == "pdr" {
		proofRef, err = manager.CopyRef(engine.pdrProof)
	} else {
		proofRef, err = manager.CopyRef(engine.bmcProof)
	}
	fragmentMust(t, err)
	defer func() { fragmentMust(t, manager.DelRef(&proofRef)) }()
	proof, err := manager.RefGet(proofRef)
	fragmentMust(t, err)
	fact, err := manager.GetProofFact(proof)
	fragmentMust(t, err)
	if fact != goalApp {
		t.Fatal("retry proof conclusion differs")
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
		t.Fatal("retry proof is invalid")
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
			t.Fatal("invalid retry proof side condition", checked)
		}
	}
	if backend == "pdr" {
		if len(engine.pdrPredicates) == 0 {
			t.Fatal("PDR did not use sliced predicates")
		}
	} else {
		for _, rule := range engine.bmcTrace {
			decl, e := rule.HeadDeclaration()
			fragmentMust(t, e)
			if decl != p && decl != q && decl != r && decl != goal {
				t.Fatal("retry retained a sliced predicate")
			}
		}
	}
}

func TestPDRConvertsSlicedProofToOriginalRules(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	for _, setting := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, globals.SourceSetName(setting, "false"))
	}
	fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine("pdr"))
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
	p, goal := declare("pdr-sliced-proof-P", []ast.ASTHandle{integer}), declare("pdr-sliced-proof-goal", nil)
	empty := declare("pdr-sliced-proof-empty", nil)
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
	status, err := engine.QueryPredicate(goal)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("PDR status", status)
	}
	if _, found := engine.pdrPredicates[p]; !found {
		t.Fatal("PDR did not use a sliced predicate")
	}
	proof, err := manager.RefGet(engine.pdrProof)
	fragmentMust(t, err)
	fact, err := manager.GetProofFact(proof)
	fragmentMust(t, err)
	if fact != goalApp {
		t.Fatal("converted proof has the wrong conclusion")
	}
	for _, get := range []func(ast.ASTHandle) (ast.ASTRef, error){engine.Proof, engine.Answer} {
		ref, e := get(goal)
		fragmentMust(t, e)
		handle, e := manager.RefGet(ref)
		fragmentMust(t, e)
		if handle != proof {
			t.Fatal("public proof does not use converted snapshot")
		}
		fragmentMust(t, manager.DelRef(&ref))
		if ref, e = get(p); e == nil {
			if ref != (ast.ASTRef{}) {
				fragmentMust(t, manager.DelRef(&ref))
			}
			t.Fatal("proof accepted a different query predicate")
		}
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
		t.Fatal("converted PDR proof is invalid")
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
			t.Fatal("invalid converted PDR proof side condition", checked)
		}
	}
	owned, err := manager.CopyRef(engine.pdrProof)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, manager.DelRef(&owned)) }()
	status, err = engine.QueryPredicate(goal)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("repeated PDR status", status)
	}
	old, err := manager.RefGet(owned)
	fragmentMust(t, err)
	if _, err = manager.GetProofFact(old); err != nil {
		t.Fatal("previous proof did not survive query replacement", err)
	}
	status, err = engine.QueryPredicate(empty)
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("empty PDR query", status)
	}
	if engine.pdrProof != (ast.ASTRef{}) {
		t.Fatal("unreachable query kept a stale proof")
	}
	if _, err = manager.GetProofFact(old); err != nil {
		t.Fatal("owned proof did not survive unreachable query", err)
	}
	if ref, e := engine.Proof(empty); e == nil {
		if ref != (ast.ASTRef{}) {
			fragmentMust(t, manager.DelRef(&ref))
		}
		t.Fatal("unreachable query returned a reachable proof")
	}
}

func TestSpacerSlicesWithProofTraceEnabled(t *testing.T) {
	for _, mode := range []string{"parameter", "forced"} {
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
			fragmentMust(t, engine.SelectEngine("spacer"))
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
			p := declare("spacer-trace-slice-P", []ast.ASTHandle{integer})
			goal := declare("spacer-trace-slice-goal", nil)
			missing := declare("spacer-trace-slice-missing", nil)
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
			status, err := engine.QueryPredicate(goal)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("reachable query", status)
			}
			if _, found := engine.spacerPredicates[p]; !found {
				t.Fatal("proof tracing prevented Spacer slicing")
			}
			answer, err := engine.Answer(goal)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, manager.DelRef(&answer)) }()
			handle, err := manager.RefGet(answer)
			fragmentMust(t, err)
			if handle.IsNull() {
				t.Fatal("missing reachable answer")
			}
			proof, err := engine.spacer.Proof()
			fragmentMust(t, err)
			if !proof.IsNull() {
				t.Fatal("Spacer unexpectedly returned an overall proof")
			}
			status, err = engine.QueryPredicate(missing)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("unreachable query", status)
			}
			model, err := engine.Model(missing)
			fragmentMust(t, err)
			fragmentMust(t, model.Close())
			if _, err = manager.RefGet(answer); err != nil {
				t.Fatal("previous answer lost its ownership", err)
			}
		})
	}
}
