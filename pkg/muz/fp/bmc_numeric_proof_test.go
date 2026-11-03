package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp linear/qlinear/nonlinear proof assembly.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBMCNumericProofIndependentValidation(t *testing.T) {
	for _, name := range []string{"bmc/nonlinear", "qbmc/nonlinear", "bmc/linear", "qbmc/linear", "bmc/recursive", "qbmc/recursive"} {
		t.Run(name, func(t *testing.T) {
			runBMCNumericProofIndependentValidation(t, name)
		})
	}
}

// TestQBMCRecursiveRuleSelection isolates the intermittent level-zero rule
// selection failure while exercising the same setup and proof checks as the
// numeric integration test. Repeat with go test -run '^TestQBMCRecursiveRuleSelection$' -count=100.
func TestQBMCRecursiveRuleSelection(t *testing.T) {
	runBMCNumericProofIndependentValidation(t, "qbmc/recursive")
}

func runBMCNumericProofIndependentValidation(t *testing.T, name string) {
	m, g := newPDRLifecycleServices(t)
	for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
		fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
	}
	fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	fragmentMust(t, e.SelectEngine(strings.Split(name, "/")[0]))
	var pins []ast.ASTRef
	defer func() {
		for i := len(pins) - 1; i >= 0; i-- {
			fragmentMust(t, m.DelRef(&pins[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		fragmentMust(t, err)
		r, err := m.NewRef(h)
		fragmentMust(t, err)
		pins = append(pins, r)
		return h
	}
	boolean := own(m.MkBoolSort())
	one := own(m.ArithmeticIntegerNumeralText("1"))
	two := own(m.ArithmeticIntegerNumeralText("2"))
	three := own(m.ArithmeticIntegerNumeralText("3"))
	four := own(m.ArithmeticIntegerNumeralText("4"))
	five := own(m.ArithmeticIntegerNumeralText("5"))
	integer, err := m.GetSort(one)
	fragmentMust(t, err)
	x := own(m.MkVar(0, integer))
	y := own(m.MkVar(1, integer))
	decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		fragmentMust(t, err)
		d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
		fragmentMust(t, e.RegisterPredicate(d))
		return d
	}
	goal := decl("numeric-goal", nil)
	left := decl("numeric-left", []ast.ASTHandle{integer})
	right := decl("numeric-right", []ast.ASTHandle{integer})
	leafDomain := []ast.ASTHandle{integer, integer}
	if strings.HasSuffix(name, "/recursive") {
		leafDomain = leafDomain[:1]
	}
	leaf := decl("numeric-leaf", leafDomain)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, args)) }
	and := func(args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkAndArray(args)) }
	var originals []ast.ASTHandle
	add := func(body, head ast.ASTHandle, n int, name string) {
		formula := own(m.MkImplies(body, head))
		sorts := make([]ast.ASTHandle, n)
		names := make([]symbol.Symbol, n)
		for i := range sorts {
			sorts[i] = integer
			names[i] = symbol.Numerical(uint32(i))
		}
		if n != 0 {
			formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
		}
		originals = append(originals, formula)
		fragmentMust(t, e.AddRule(formula, name))
	}
	if strings.HasSuffix(name, "/recursive") {
		zero := own(m.ArithmeticIntegerNumeralText("0"))
		add(own(m.MkTrue()), app(leaf, zero), 0, "recursive-base")
		add(and(app(leaf, x), own(m.MkArithmeticLT(x, three))), app(leaf, own(m.MkArithmeticAdd(x, one))), 1, "recursive-successor")
		add(app(leaf, three), app(goal), 0, "recursive-goal")
	} else {
		// Numeric bounds and shared rule variables exercise actual model bindings.
		add(and(own(m.MkArithmeticGE(x, two)), own(m.MkArithmeticLE(x, five))), app(leaf, x, y), 2, "numeric-leaf-range")
		add(and(app(leaf, x, y), own(m.MkArithmeticLE(x, three))), app(left, x), 2, "numeric-left-step")
		if strings.HasSuffix(name, "/linear") {
			add(and(app(left, x), own(m.MkEq(x, three))), app(goal), 1, "numeric-goal-step")
		} else {
			add(and(app(leaf, x, y), own(m.MkArithmeticGE(x, four))), app(right, x), 2, "numeric-right-step")
			add(and(app(left, x), app(right, y), own(m.MkEq(y, own(m.MkArithmeticAdd(x, one))))), app(goal), 2, "numeric-goal-step")
		}
	}
	solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, solver.Close()) }()
	status, err := e.QueryPredicate(goal)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("numeric BMC query", status)
	}
	answer, err := e.Answer(goal)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&answer)) }()
	proof, err := m.RefGet(answer)
	fragmentMust(t, err)
	fact, err := m.GetProofFact(proof)
	fragmentMust(t, err)
	want := app(e.queryClonePredicate)
	if fact != want {
		t.Fatal("proof does not conclude requested query", fact, want)
	}
	checker, err := proofs.NewChecker(m)
	fragmentMust(t, err)
	valid, conditions, err := checker.Check(proof)
	fragmentMust(t, err)
	defer func() {
		for _, c := range conditions {
			fragmentMust(t, m.DecRef(c))
		}
	}()
	if !valid {
		t.Fatal("numeric proof failed structural checker")
	}
	for i, c := range conditions {
		fragmentMust(t, solver.Push())
		fragmentMust(t, solver.Assert(own(m.MkNot(c))))
		status, err := solver.CheckSat(nil)
		fragmentMust(t, err)
		if status != lbool.False {
			t.Fatal("invalid independent proof condition", i, status)
		}
		fragmentMust(t, solver.Pop(1))
	}
	// The checker treats asserted leaves as axioms. Independently
	// justify each against only the original program and query bridge.
	for _, formula := range originals {
		fragmentMust(t, solver.Assert(formula))
	}
	fragmentMust(t, solver.Assert(own(m.MkImplies(app(goal), want))))
	status, err = solver.CheckSat(nil)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("original proof premises inconsistent", status)
	}
	seen := map[ast.ASTHandle]bool{}
	stack := []ast.ASTHandle{proof}
	leaves := 0
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		isProof, err := m.IsProof(node)
		fragmentMust(t, err)
		if !isProof {
			continue
		}
		kind, err := m.GetDeclKind(node)
		fragmentMust(t, err)
		if kind == ast.ASTPrAsserted {
			leaves++
			asserted, err := m.GetProofFact(node)
			fragmentMust(t, err)
			fragmentMust(t, solver.Push())
			fragmentMust(t, solver.Assert(own(m.MkNot(asserted))))
			status, err := solver.CheckSat(nil)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("proof assertion not justified by original clauses", status)
			}
			fragmentMust(t, solver.Pop(1))
		}
		count, err := m.GetNumArgs(node)
		fragmentMust(t, err)
		for i := uint32(0); i < count; i++ {
			child, err := m.GetArg(node, i)
			fragmentMust(t, err)
			stack = append(stack, child)
		}
	}
	if leaves == 0 {
		t.Fatal("proof has no original clause premises")
	}

}
