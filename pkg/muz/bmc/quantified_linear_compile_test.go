package bmc_test

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestBMCQuantifiedLinearCompilationLevels(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	m, err := context.Manager()
	must(err)
	budget := m.Budget()
	defer func() {
		must(context.Close())
		if used := budget.Used(); used != 0 {
			t.Errorf("quantified compilation fixture left %d bytes", used)
		}
	}()
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	decl := func(name string) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(d))
		return d
	}
	p, q, r := decl("qlinear-p"), decl("qlinear-q"), decl("qlinear-r")
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	app := func(d, value ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, []ast.ASTHandle{value})
		must(e)
		return h
	}
	add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, e := rm.NewRule(head, tails, nil, "")
		must(e)
		must(set.Add(rule))
		must(rule.Close())
	}
	add(app(p, one))
	add(app(q, x), app(p, x))
	same, err := m.MkEq(x, y)
	must(err)
	positive, err := m.MkArithmeticGT(z, zero)
	must(err)
	add(app(r, x), app(p, y), same, positive)
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("rules did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	if invalid, e := compiler.CompileQuantifiedLinear(set, 0); e == nil || len(invalid) != 0 {
		t.Fatal("zero-width quantified index accepted", e)
	}
	formulas, err := compiler.CompileQuantifiedLinear(set, 4)
	must(err)
	defer func() {
		for i := range formulas {
			must(m.DelRef(&formulas[i]))
		}
	}()
	if len(formulas) != 6 {
		t.Fatal("one rule and one predicate implication per group", len(formulas))
	}
	solver, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(solver.Close()) }()
	sub, err := rewriter.NewVarSubstitution(m, true)
	must(err)
	defer func() { must(sub.Close()) }()
	var levels []ast.ASTHandle
	for _, value := range []int64{0, 1} {
		n, e := m.BitVectorNumeralRat(big.NewRat(value, 1), 4)
		must(e)
		ref, e := m.NewRef(n)
		must(e)
		defer func() { must(m.DelRef(&ref)) }()
		levels = append(levels, n)
	}
	for _, formula := range formulas {
		h, e := m.RefGet(formula)
		must(e)
		body, e := m.GetQuantifierBody(h)
		must(e)
		for _, level := range levels {
			ref, e := m.NewRef(ast.ASTHandle{})
			must(e)
			must(sub.Substitute(body, []ast.ASTHandle{level}, ref))
			inst, e := m.RefGet(ref)
			must(e)
			must(solver.Assert(inst))
			must(m.DelRef(&ref))
		}
	}
	indexSort, err := m.BitVectorSort(4)
	must(err)
	for _, tc := range []struct {
		name         string
		level, value ast.ASTHandle
		want         lbool.LBool
	}{
		{"qlinear-q", levels[0], one, lbool.False},
		{"qlinear-q", levels[1], zero, lbool.False},
		{"qlinear-q", levels[1], one, lbool.True},
		{"qlinear-r", levels[0], one, lbool.False},
		{"qlinear-r", levels[1], zero, lbool.False},
		{"qlinear-r", levels[1], one, lbool.True},
	} {
		qName, e := m.Symbols.Intern(tc.name + "#")
		must(e)
		qIndexed, e := m.MkFuncDecl(qName, []ast.ASTHandle{indexSort}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		argName, e := m.Symbols.Intern(tc.name + "#0")
		must(e)
		qArg, e := m.MkFuncDecl(argName, []ast.ASTHandle{indexSort}, integer, ast.ASTFuncDeclInfo{})
		must(e)
		must(solver.Push())
		query, e := m.MkApp(qIndexed, []ast.ASTHandle{tc.level})
		must(e)
		must(solver.Assert(query))
		argument, e := m.MkApp(qArg, []ast.ASTHandle{tc.level})
		must(e)
		eq, e := m.MkEq(argument, tc.value)
		must(e)
		must(solver.Assert(eq))
		status, e := solver.CheckSat(nil)
		must(e)
		if status != tc.want {
			t.Fatalf("quantified level result: got %v, want %v", status, tc.want)
		}
		must(solver.Pop(1))
	}
}

func TestBMCQuantifiedLinearSearchPreservesSolverScope(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	m, err := context.Manager()
	must(err)
	budget := m.Budget()
	defer func() {
		must(context.Close())
		if used := budget.Used(); used != 0 {
			t.Errorf("quantified search fixture left %d bytes", used)
		}
	}()
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("qlinear-search-fact")
	must(err)
	predicate, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	fact, err := m.MkApp(predicate, nil)
	must(err)
	goalName, err := m.Symbols.Intern("qlinear-search-goal")
	must(err)
	goal, err := m.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	goalApp, err := m.MkApp(goal, nil)
	must(err)
	middleName, err := m.Symbols.Intern("qlinear-search-middle")
	must(err)
	middle, err := m.MkFuncDecl(middleName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	middleApp, err := m.MkApp(middle, nil)
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	must(rm.RegisterPredicate(predicate))
	must(rm.RegisterPredicate(goal))
	must(rm.RegisterPredicate(middle))
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	rule, err := rm.NewRule(fact, nil, nil, "fact")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	falsity, err := m.MkFalse()
	must(err)
	impossible, err := rm.NewRule(goalApp, []ast.ASTHandle{fact, falsity}, nil, "impossible")
	must(err)
	must(set.Add(impossible))
	must(impossible.Close())
	transition, err := rm.NewRule(middleApp, []ast.ASTHandle{fact}, nil, "middle")
	must(err)
	must(set.Add(transition))
	must(transition.Close())
	step, err := rm.NewRule(goalApp, []ast.ASTHandle{middleApp}, nil, "step")
	must(err)
	must(set.Add(step))
	must(step.Close())
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	raw, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(raw.Close()) }()
	solver, ok := raw.(*smtcore.SMTSolver)
	if !ok {
		t.Fatal("expected native SMT solver")
	}
	guardName, err := m.Symbols.Intern("qlinear-existing-guard")
	must(err)
	guard, err := m.MkConst(guardName, boolean)
	must(err)
	guardRef, err := m.NewRef(guard)
	must(err)
	defer func() { must(m.DelRef(&guardRef)) }()
	must(solver.AssertFormula(guard))
	nonlinear, err := base.NewRuleSet(rm)
	must(err)
	badRule, err := rm.NewRule(goalApp, []ast.ASTHandle{fact, fact}, nil, "nonlinear")
	must(err)
	must(nonlinear.Add(badRule))
	must(badRule.Close())
	_, _, badModel, badErr := compiler.SearchQuantifiedLinear(nonlinear, goal, solver)
	must(nonlinear.Close())
	if badErr == nil || badModel != nil {
		t.Fatal("nonlinear rules were accepted by quantified linear search", badErr)
	}
	m.ResourceLimit().Cancel()
	_, _, canceledModel, cancelErr := compiler.SearchQuantifiedLinear(set, goal, solver)
	m.ResourceLimit().ResetCancel()
	if cancelErr == nil || canceledModel != nil {
		t.Fatal("canceled quantified search returned a model", cancelErr)
	}
	// The source has a second checkpoint after compiling/asserting a width and
	// before adding q(T). One unit lets the outer checkpoint pass and requires
	// the post-compilation checkpoint to restore the solver scope.
	must(m.ResourceLimit().Push(1))
	limitedStatus, limitedWidth, limitedModel, limitedErr := compiler.SearchQuantifiedLinear(set, goal, solver)
	m.ResourceLimit().Pop()
	if limitedErr == nil || limitedStatus != lbool.Undef || limitedWidth != 4 || limitedModel != nil {
		t.Fatal("post-compilation quantified checkpoint was skipped", limitedStatus, limitedWidth, limitedModel, limitedErr)
	}
	status, width, md, err := compiler.SearchQuantifiedLinear(set, goal, solver)
	must(err)
	if status != lbool.True || width != 4 || md == nil {
		t.Fatal("quantified search", status, width, md)
	}
	defer func() { must(md.DecRef()) }()
	indexSort, err := m.BitVectorSort(width)
	must(err)
	indexName, err := m.Symbols.Intern("T")
	must(err)
	index, err := m.MkConst(indexName, indexSort)
	must(err)
	indexDecl, err := m.GetDecl(index)
	must(err)
	value, err := md.ConstantInterpretation(indexDecl)
	must(err)
	if value.IsNull() {
		t.Fatal("winning model lost its index after solver pop")
	}
	level, bits, numeral, err := m.BitVectorNumeralBigInt(value)
	must(err)
	if !numeral || bits != width || level.Cmp(big.NewInt(2)) < 0 {
		t.Fatal("recursive query chose an index below its derivation depth", level, bits)
	}
	services, err := context.ModelEvaluatorConfig()
	must(err)
	completed, failures := false, 0
	for stop := int64(0); stop < 512 && !completed; stop++ {
		budget.FailAfter(stop)
		candidate, cause := compiler.QuantifiedLinearTrace(set, goal, width, md, services)
		budget.FailAfter(-1)
		for i := len(candidate) - 1; i >= 0; i-- {
			must(candidate[i].Close())
		}
		if cause == nil {
			completed = true
			break
		}
		var construction memory.ConstructionFailure
		if errors.As(cause, &construction) {
			must(construction.Cleanup())
		}
		var memout memory.Failure
		if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
			t.Fatalf("trace allocation frontier %d: %v", stop, cause)
		}
		failures++
	}
	if !completed || failures == 0 {
		t.Fatal("trace allocation campaign did not reach both failure and success", failures, completed)
	}
	emptyModel, err := model.NewModelCore(m)
	must(err)
	invalidTrace, invalidErr := compiler.QuantifiedLinearTrace(set, goal, width, emptyModel, services)
	must(emptyModel.Close())
	if invalidErr == nil || len(invalidTrace) != 0 {
		t.Fatal("uninterpreted model was accepted for trace reconstruction", invalidErr)
	}
	trace, err := compiler.QuantifiedLinearTrace(set, goal, width, md, services)
	must(err)
	if len(trace) != 3 {
		t.Fatal("quantified rule trace length", len(trace))
	}
	for i, want := range []string{"step", "middle", "fact"} {
		got, e := trace[i].Name()
		must(e)
		if got != want {
			t.Fatal("quantified rule trace", i, got, want)
		}
	}
	for i := len(trace) - 1; i >= 0; i-- {
		must(trace[i].Close())
	}
	proof, err := compiler.QuantifiedLinearProof(set, goal, width, md, services)
	must(err)
	defer func() { must(m.DelRef(&proof)) }()
	proofHandle, err := m.RefGet(proof)
	must(err)
	conclusion, err := m.GetProofFact(proofHandle)
	must(err)
	if conclusion != goalApp {
		t.Fatal("quantified proof conclusion", conclusion, goalApp)
	}
	negated, err := m.MkNot(guard)
	must(err)
	status, err = solver.CheckSat([]ast.ASTHandle{negated})
	must(err)
	if status != lbool.False {
		t.Fatal("quantified search damaged the original solver scope", status)
	}
}

func TestBMCQuantifiedLinearCompilationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			context, err := cmd_context.NewStandaloneContext()
			must(err)
			m, err := context.Manager()
			must(err)
			budget := m.Budget()
			t.Cleanup(func() {
				must(context.Close())
				if used := budget.Used(); used != 0 {
					t.Errorf("compilation left %d bytes", used)
				}
			})
			must(m.RegisterDeclarationPlugins())
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("qlinear-budget")
			must(err)
			decl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			head, err := m.MkApp(decl, nil)
			must(err)
			rm, err := base.NewRuleManager(m)
			must(err)
			t.Cleanup(func() { must(rm.Close()) })
			must(rm.RegisterPredicate(decl))
			set, err := base.NewRuleSet(rm)
			must(err)
			t.Cleanup(func() { must(set.Close()) })
			rule, err := rm.NewRule(head, nil, nil, "fact")
			must(err)
			must(set.Add(rule))
			must(rule.Close())
			compiler, err := bmc.NewCompiler(m)
			must(err)
			t.Cleanup(func() { must(compiler.Close()) })
			budget.FailAfter(stop)
			formulas, err := compiler.CompileQuantifiedLinear(set, 4)
			budget.FailAfter(-1)
			for i := range formulas {
				must(m.DelRef(&formulas[i]))
			}
			if err == nil {
				success = true
				return
			}
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				must(failure.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal(err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("quantified compilation allocation frontier", stop)
		}
	}
	if !success || failures == 0 {
		t.Fatal("did not observe failure and success", failures, success)
	}
}
