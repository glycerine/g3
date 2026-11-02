package duality_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Check both the proof DAG and every logical side condition produced by the
// source proof checker. Structural acceptance alone does not prove validity.
func validateDualityProof(t *testing.T, m *ast.ASTManager, config smtcore.SMTSolverConfig, proof ast.ASTRef) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	h, err := m.RefGet(proof)
	must(err)
	if h.IsNull() {
		t.Fatal("missing counterexample proof")
	}
	fact, err := m.GetProofFact(h)
	must(err)
	if !m.IsFalse(fact) {
		t.Fatal("query proof must conclude false")
	}
	checker, err := proofs.NewChecker(m)
	must(err)
	valid, side, err := checker.Check(h)
	must(err)
	defer func() { must(checker.ReleaseSideConditions(side)) }()
	if !valid || len(side) == 0 {
		t.Fatal("invalid or empty hyper-resolution proof", valid, len(side))
	}
	native, err := smtcore.NewSMTSolver(m, nil, "", config)
	must(err)
	defer func() { must(native.Close()) }()
	for i, condition := range side {
		must(native.Push())
		negated, err := m.MkNot(condition)
		must(err)
		must(native.Assert(negated))
		status, err := native.CheckSat(nil)
		must(err)
		must(native.Pop(1))
		if status != lbool.False {
			t.Fatal("proof side condition is not valid", i, status)
		}
	}
}

func TestDualityProofQuantifiedNonlinearAndRepeatedPremises(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := cmd_context.NewStandaloneContext()
	must(err)
	defer owner.Close()
	m, err := owner.Manager()
	must(err)
	config, err := owner.SolverTranslationConfig()
	must(err)
	backend, err := duality.NewBackend(m, config, false)
	must(err)
	defer backend.Close()
	var refs []ast.ASTRef
	defer func() {
		for i := range refs {
			must(m.DelRef(&refs[i]))
		}
	}()
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, err := m.NewRef(h)
		must(err)
		refs = append(refs, ref)
		return h
	}
	boolean := pin(m.MkBoolSort())
	x, y := pin(m.MkVar(1, boolean)), pin(m.MkVar(0, boolean))
	xn, err := m.Symbols.Intern("proof-x")
	must(err)
	yn, err := m.Symbols.Intern("proof-y")
	must(err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		must(err)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = boolean
		}
		return pin(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
	}
	p, goal, dead := declare("proof-P", 2), declare("proof-Goal", 2), declare("proof-dead", 0)
	pxy := pin(m.MkApp(p, []ast.ASTHandle{x, y}))
	pyx := pin(m.MkApp(p, []ast.ASTHandle{y, x}))
	gxy := pin(m.MkApp(goal, []ast.ASTHandle{x, y}))
	notY := pin(m.MkNot(y))
	body := pin(m.MkAndArray([]ast.ASTHandle{pxy, x, pyx, notY, pxy}))
	quantify := func(h ast.ASTHandle) ast.ASTHandle {
		return pin(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean, boolean}, Names: []usymbol.Symbol{xn, yn}, Body: h}))
	}
	rules := []ast.ASTHandle{
		pin(m.MkApp(dead, nil)), // pruning this first rule must not shift proof clauses
		quantify(pxy),
		quantify(pin(m.MkImplies(body, gxy))),
	}
	truth, falsity := pin(m.MkTrue()), pin(m.MkFalse())
	query := pin(m.MkApp(goal, []ast.ASTHandle{truth, falsity}))
	prepared, err := duality.PrepareQuery(m, rules, []uint32{math.MaxUint32, math.MaxUint32, math.MaxUint32}, query, []ast.ASTHandle{p, goal, dead}, math.MaxUint32)
	must(err)
	defer prepared.Close()
	status, err := backend.Query(prepared)
	must(err)
	must(prepared.Close())
	if status != lbool.True {
		t.Fatal("independently reachable Boolean Horn query", status)
	}
	proof, err := backend.GetProof()
	must(err)
	defer func() { must(m.DelRef(&proof)) }()
	validateDualityProof(t, m, config, proof)

	// QE-lite may eliminate the predecessor from y = !x. The original
	// quantified rule still needs x=true to justify W(false) from W(true).
	w := declare("proof-W", 1)
	wx := pin(m.MkApp(w, []ast.ASTHandle{x}))
	wy := pin(m.MkApp(w, []ast.ASTHandle{y}))
	notX := pin(m.MkNot(x))
	transition := pin(m.MkEq(y, notX))
	stepBody := pin(m.MkAnd(wx, transition))
	witnessRules := []ast.ASTHandle{
		pin(m.MkApp(w, []ast.ASTHandle{truth})),
		quantify(pin(m.MkImplies(stepBody, wy))),
	}
	witnessQuery := pin(m.MkApp(w, []ast.ASTHandle{falsity}))
	preparedWitness, err := duality.PrepareQuery(m, witnessRules, []uint32{math.MaxUint32, math.MaxUint32}, witnessQuery, []ast.ASTHandle{w}, math.MaxUint32)
	must(err)
	defer preparedWitness.Close()
	status, err = backend.Query(preparedWitness)
	must(err)
	must(preparedWitness.Close())
	if status != lbool.True {
		t.Fatal("independently reachable Boolean predecessor query", status)
	}
	witnessProof, err := backend.GetProof()
	must(err)
	defer func() { must(m.DelRef(&witnessProof)) }()
	validateDualityProof(t, m, config, witnessProof)
	completed := false
	before := m.Stats().LiveReferences
	for fault := int64(0); fault < 1024; fault++ {
		m.Budget().FailAfter(fault)
		trial, failure := backend.GetProof()
		m.Budget().FailAfter(-1)
		if failure == nil {
			must(m.DelRef(&trial))
			completed = true
		} else {
			if trial != (ast.ASTRef{}) {
				t.Fatal("failed witness extraction published a partial proof", fault, failure)
			}
			// A retry owns cleanup of any stopped construction before starting
			// a fresh witness solve; the original counterexample stays usable.
			retry, err := backend.GetProof()
			must(err)
			must(m.DelRef(&retry))
		}
		if m.Stats().LiveReferences != before {
			t.Fatal("witness extraction leaked references", fault, failure, before, m.Stats().LiveReferences)
		}
		if completed {
			t.Log("witness extraction allocation prefixes", fault)
			break
		}
	}
	if !completed {
		t.Fatal("witness extraction did not reach allocation success")
	}

	// Normalized integer transition heads keep y while projecting x; the
	// second step requires recovering x=1, rather than the default x=0.
	zero := pin(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := pin(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	two := pin(m.ArithmeticNumeralRat(big.NewRat(2, 1), true))
	integer := pin(m.GetSort(zero))
	old, next := pin(m.MkVar(1, integer)), pin(m.MkVar(0, integer))
	rn, err := m.Symbols.Intern("proof-Reach")
	must(err)
	reach := pin(m.MkFuncDecl(rn, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
	rOld := pin(m.MkApp(reach, []ast.ASTHandle{old}))
	rNext := pin(m.MkApp(reach, []ast.ASTHandle{next}))
	plusOne := pin(m.MkArithmeticAdd(old, one))
	update := pin(m.MkEq(next, plusOne))
	intBody := pin(m.MkAnd(update, rOld))
	intRule := pin(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, integer}, Names: []usymbol.Symbol{xn, yn}, Body: pin(m.MkImplies(intBody, rNext))}))
	intRules := []ast.ASTHandle{pin(m.MkApp(reach, []ast.ASTHandle{zero})), intRule}
	intQuery := pin(m.MkApp(reach, []ast.ASTHandle{two}))
	preparedInt, err := duality.PrepareQuery(m, intRules, []uint32{math.MaxUint32, math.MaxUint32}, intQuery, []ast.ASTHandle{reach}, math.MaxUint32)
	must(err)
	defer preparedInt.Close()
	status, err = backend.Query(preparedInt)
	must(err)
	must(preparedInt.Close())
	if status != lbool.True {
		t.Fatal("independently reachable integer predecessor query", status)
	}
	intProof, err := backend.GetProof()
	must(err)
	defer func() { must(m.DelRef(&intProof)) }()
	validateDualityProof(t, m, config, intProof)
	must(backend.Close())
	validateDualityProof(t, m, config, proof)
	validateDualityProof(t, m, config, witnessProof)
	validateDualityProof(t, m, config, intProof)
}
