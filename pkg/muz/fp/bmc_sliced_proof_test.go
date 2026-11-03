package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp bmc::query applies slicing before
// selecting the linear, quantified-linear, or nonlinear compiler.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBMCSlicedProofUsesOriginalRules(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		for _, shape := range []struct {
			name      string
			nonlinear bool
			trace     bool
		}{
			{"linear", false, true},
			{"nonlinear", true, true},
			{"nonlinear-post-query-proof", true, false},
		} {
			for _, slicing := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/slice=%v", backend, shape.name, slicing), func(t *testing.T) {
					testBMCSlicedProofUsesOriginalRules(t, backend, shape.nonlinear, shape.trace, slicing)
				})
			}
		}
	}
}

func testBMCSlicedProofUsesOriginalRules(t *testing.T, backend string, nonlinear, trace, slicing bool) {
	m, g := newPDRLifecycleServices(t)
	for _, key := range []string{"inline_eager", "inline_linear", "subsumption_checker", "coalesce_rules"} {
		fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
	}
	fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
	fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", fmt.Sprint(slicing)))
	fragmentMust(t, g.SourceSetName("fixedpoint.generate_proof_trace", fmt.Sprint(trace)))
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	fragmentMust(t, e.SelectEngine(backend))
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
	two := own(m.ArithmeticIntegerNumeralText("2"))
	three := own(m.ArithmeticIntegerNumeralText("3"))
	integer, err := m.GetSort(two)
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
	p := decl("sliced-proof-P", []ast.ASTHandle{integer, integer})
	q := decl("sliced-proof-Q", []ast.ASTHandle{integer, integer})
	goal := decl("sliced-proof-Goal", nil)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, args)) }
	var originals []ast.ASTHandle
	add := func(body, head ast.ASTHandle, variables int, name string) {
		formula := own(m.MkImplies(body, head))
		if variables != 0 {
			sorts := make([]ast.ASTHandle, variables)
			names := make([]symbol.Symbol, variables)
			for i := range sorts {
				sorts[i] = integer
				names[i] = symbol.Numerical(uint32(i))
			}
			formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
		}
		originals = append(originals, formula)
		fragmentMust(t, e.AddRule(formula, name))
	}
	// The first coordinate requires a numeric binding; the unused second
	// coordinate must be removed by slicing while proofs still use P and Q.
	add(own(m.MkEq(x, two)), app(p, x, y), 2, "P-fact")
	body := app(p, two, x)
	variables := 1
	if nonlinear {
		add(own(m.MkEq(x, three)), app(q, x, y), 2, "Q-fact")
		body = own(m.MkAnd(body, app(q, three, y)))
		variables = 2
	}
	add(body, app(goal), variables, "Goal-step")
	status, err := e.QueryPredicate(goal)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("sliced query status", status)
	}
	if e.queryClone == nil || e.forceProofTrace || e.queryClone.forceProofTrace {
		t.Fatal("query snapshot missing or temporary proof setting escaped")
	}
	sliced := 0
	for _, rule := range e.bmcTrace {
		d, err := rule.HeadDeclaration()
		fragmentMust(t, err)
		if d != p && d != q && d != goal && d != e.queryClonePredicate {
			arity, err := m.GetArity(d)
			fragmentMust(t, err)
			if arity >= 2 {
				t.Fatal("sliced predicate kept the unused argument")
			}
			sliced++
		}
	}
	wantSliced := 0
	if slicing {
		wantSliced = 1
		if nonlinear {
			wantSliced = 2
		}
	}
	if sliced != wantSliced {
		t.Fatalf("trace has %d sliced rules, want %d", sliced, wantSliced)
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
		t.Fatal("converted proof does not conclude the requested query")
	}
	checker, err := proofs.NewChecker(m)
	fragmentMust(t, err)
	valid, conditions, err := checker.Check(proof)
	fragmentMust(t, err)
	defer func() {
		for _, condition := range conditions {
			fragmentMust(t, m.DecRef(condition))
		}
	}()
	if !valid {
		t.Fatal("converted proof failed structural validation")
	}
	solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, solver.Close()) }()
	verify := func(formula ast.ASTHandle) {
		fragmentMust(t, solver.Push())
		fragmentMust(t, solver.Assert(own(m.MkNot(formula))))
		status, err := solver.CheckSat(nil)
		fragmentMust(t, err)
		if status != lbool.False {
			t.Fatal("proof condition does not follow from independent premises", status)
		}
		fragmentMust(t, solver.Pop(1))
	}
	for _, condition := range conditions {
		verify(condition)
	}
	// Asserted leaves are axioms to the proof checker. Validate them against
	// only the original program and the public query bridge, without slices.
	for _, formula := range originals {
		fragmentMust(t, solver.Assert(formula))
	}
	fragmentMust(t, solver.Assert(own(m.MkImplies(app(goal), want))))
	status, err = solver.CheckSat(nil)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("original proof premises are inconsistent", status)
	}
	seen := make(map[ast.ASTHandle]bool)
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
			verify(asserted)
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
		t.Fatal("converted proof has no original rule premises")
	}
}

func TestBMCConvertsSliceProofWhenResolutionMergesTails(t *testing.T) {
	for _, reordered := range []bool{false, true} {
		t.Run(fmt.Sprintf("reordered=%v", reordered), func(t *testing.T) {
			testSliceProofWhenResolutionMergesTails(t, "bmc", reordered)
		})
	}
}
