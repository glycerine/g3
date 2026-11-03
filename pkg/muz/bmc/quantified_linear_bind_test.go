package bmc_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestBMCQuantifiedLinearBindingsUseRuleOrdinal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("qlinear-binding")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{x})
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	must(rm.RegisterPredicate(predicate))
	rule, err := rm.NewRule(head, nil, nil, "binding")
	must(err)
	defer func() { must(rule.Close()) }()
	md, err := model.NewModel(m)
	must(err)
	defer func() { must(md.Close()) }()
	indexSort, err := m.BitVectorSort(4)
	must(err)
	argumentName, err := m.Symbols.Intern("qlinear-binding#0")
	must(err)
	argument, err := m.MkFuncDecl(argumentName, []ast.ASTHandle{indexSort}, integer, ast.ASTFuncDeclInfo{})
	must(err)
	zero, err := m.BitVectorNumeralRat(big.NewRat(0, 1), 4)
	must(err)
	one, err := m.BitVectorNumeralRat(big.NewRat(1, 1), 4)
	must(err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	must(err)
	nine, err := m.ArithmeticIntegerNumeralText("9")
	must(err)
	interpretation, err := model.NewFunctionInterpretation(m, 1)
	must(err)
	must(interpretation.InsertEntry([]ast.ASTHandle{zero}, nine))
	must(interpretation.InsertEntry([]ast.ASTHandle{one}, seven))
	must(md.Core().RegisterFunction(argument, interpretation))
	services, err := context.ModelEvaluatorConfig()
	must(err)
	// C++ model::eval disables completion even when the global evaluator
	// option requests it; an absent interpretation must stay symbolic here.
	must(services.Globals.SourceSetName("model_evaluator.completion", "true"))
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	emptyModel, err := model.NewModel(m)
	must(err)
	defer func() { must(emptyModel.Close()) }()
	unbound, err := compiler.QuantifiedLinearModelBindings(rule, 0, 1, 4, emptyModel.Core(), services)
	must(err)
	if len(unbound) != 1 {
		t.Fatal("unbound binding count", len(unbound))
	}
	defer func() { must(m.DelRef(&unbound[0])) }()
	missing, err := m.RefGet(unbound[0])
	must(err)
	symbolic, err := m.MkApp(argument, []ast.ASTHandle{zero})
	must(err)
	if missing != symbolic {
		t.Error("missing interpretation did not preserve the symbolic result", missing)
	}
	bindings, err := compiler.QuantifiedLinearModelBindings(rule, 0, 1, 4, md.Core(), services)
	must(err)
	if len(bindings) != 1 {
		t.Fatal("binding count", len(bindings))
	}
	defer func() { must(m.DelRef(&bindings[0])) }()
	value, err := m.RefGet(bindings[0])
	must(err)
	if value != nine {
		t.Fatal("binding was not evaluated at the selected rule ordinal", value)
	}
}

func TestBMCQuantifiedLinearProofWithRuleVariables(t *testing.T) {
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
			t.Errorf("quantified proof fixture left %d bytes", used)
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
	declare := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
		n, e := m.Symbols.Intern(text)
		must(e)
		d, e := m.MkFuncDecl(n, domain, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(d))
		return d
	}
	p := declare("qlinear-proof-p", []ast.ASTHandle{integer})
	q := declare("qlinear-proof-q", []ast.ASTHandle{integer})
	goal := declare("qlinear-proof-goal", nil)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, args)
		must(e)
		return h
	}
	add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, e := rm.NewRule(head, tails, nil, "")
		must(e)
		must(set.Add(rule))
		must(rule.Close())
	}
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	goalApp := app(goal)
	add(app(p, three))
	add(app(q, x), app(p, x))
	add(goalApp, app(q, three))
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
	status, width, md, err := compiler.SearchQuantifiedLinear(set, goal, solver)
	must(err)
	if status != lbool.True || md == nil {
		t.Fatal("quantified search", status)
	}
	defer func() { must(md.DecRef()) }()
	services, err := context.ModelEvaluatorConfig()
	must(err)
	complete, failures := false, 0
	for stop := int64(0); stop < 512 && !complete; stop++ {
		budget.FailAfter(stop)
		candidate, cause := compiler.QuantifiedLinearProof(set, goal, width, md, services)
		budget.FailAfter(-1)
		if candidate != (ast.ASTRef{}) {
			must(m.DelRef(&candidate))
		}
		if cause == nil {
			complete = true
			break
		}
		var construction memory.ConstructionFailure
		if errors.As(cause, &construction) {
			must(construction.Cleanup())
		}
		var memout memory.Failure
		if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
			t.Fatalf("proof allocation frontier %d: %v", stop, cause)
		}
		failures++
	}
	if !complete || failures == 0 {
		t.Fatal("proof allocation campaign did not reach both failure and success", failures, complete)
	}
	proof, err := compiler.QuantifiedLinearProof(set, goal, width, md, services)
	must(err)
	defer func() { must(m.DelRef(&proof)) }()
	proofHandle, err := m.RefGet(proof)
	must(err)
	fact, err := m.GetProofFact(proofHandle)
	must(err)
	if fact != goalApp {
		t.Fatal("quantified proof did not conclude the query", fact)
	}
	checker, err := proofs.NewChecker(m)
	must(err)
	valid, side, err := checker.Check(proofHandle)
	must(err)
	defer func() {
		for _, condition := range side {
			must(m.DecRef(condition))
		}
	}()
	if !valid {
		t.Fatal("quantified proof failed structural checking")
	}
	for _, condition := range side {
		must(solver.Push())
		negated, e := m.MkNot(condition)
		must(e)
		must(solver.AssertFormula(negated))
		checked, e := solver.CheckSat(nil)
		must(e)
		must(solver.Pop(1))
		if checked != lbool.False {
			t.Fatal("quantified proof side condition is not valid", checked)
		}
	}
}

func TestBMCQuantifiedLinearProofWithUninterpretedBinding(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		symbol, e := m.Symbols.Intern(name)
		must(e)
		decl, e := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(decl))
		return decl
	}
	p := declare("qlinear-unbound-P", []ast.ASTHandle{integer})
	goal := declare("qlinear-unbound-goal", nil)
	x, err := m.MkVar(0, integer)
	must(err)
	pApp, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	goalApp, err := m.MkApp(goal, nil)
	must(err)
	for _, spec := range []struct {
		head ast.ASTHandle
		tail []ast.ASTHandle
	}{{pApp, nil}, {goalApp, []ast.ASTHandle{pApp}}} {
		rule, e := rm.NewRule(spec.head, spec.tail, nil, "")
		must(e)
		must(set.Add(rule))
		must(rule.Close())
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	raw, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(raw.Close()) }()
	solver := raw.(*smtcore.SMTSolver)
	status, width, md, err := compiler.SearchQuantifiedLinear(set, goal, solver)
	must(err)
	if status != lbool.True || md == nil {
		t.Fatal("search", status)
	}
	defer func() { must(md.DecRef()) }()
	services, err := context.ModelEvaluatorConfig()
	must(err)
	proof, err := compiler.QuantifiedLinearProof(set, goal, width, md, services)
	must(err)
	defer func() { must(m.DelRef(&proof)) }()
	handle, err := m.RefGet(proof)
	must(err)
	fact, err := m.GetProofFact(handle)
	must(err)
	if fact != goalApp {
		t.Fatal("wrong proof conclusion")
	}
	checker, err := proofs.NewChecker(m)
	must(err)
	valid, side, err := checker.Check(handle)
	must(err)
	defer func() {
		for _, condition := range side {
			must(m.DecRef(condition))
		}
	}()
	if !valid {
		t.Fatal("invalid proof structure")
	}
	for _, condition := range side {
		must(solver.Push())
		negated, e := m.MkNot(condition)
		must(e)
		must(solver.AssertFormula(negated))
		checked, e := solver.CheckSat(nil)
		must(e)
		must(solver.Pop(1))
		if checked != lbool.False {
			t.Fatal("proof side condition is not valid", checked)
		}
	}
}
