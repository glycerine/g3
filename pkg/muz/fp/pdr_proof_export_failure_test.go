package fp

// Source: pdr::model_search::get_proof_trace owns each constructed proof in
// proof_ref/trail storage before publishing it through context::get_proof.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRProofExportReferenceAllocationFailureAndRetry(t *testing.T) {
	failures := 0
	for available := 0; available <= 16; available++ {
		succeeded := false
		for stop := int64(0); stop < 128; stop++ {
			t.Run(fmt.Sprintf("references=%d/allowance=%d", available, stop), func(t *testing.T) {
				failed := testPDRProofExportFailure(t, available, stop, false)
				if failed {
					failures++
				} else {
					succeeded = true
				}
			})
			if t.Failed() {
				return
			}
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("public proof export did not reach a successful allocation allowance", available)
		}
	}
	if failures == 0 {
		t.Fatal("public proof export did not exercise an allocation refusal")
	}
}

func TestPDRSymbolicProofExportReferenceAllocationFailureAndRetry(t *testing.T) {
	failures := 0
	for available := 0; available <= 32; available++ {
		succeeded := false
		for stop := int64(0); stop < 128; stop++ {
			t.Run(fmt.Sprintf("references=%d/allowance=%d", available, stop), func(t *testing.T) {
				failed := testPDRProofExportFailure(t, available, stop, true)
				if failed {
					failures++
				} else {
					succeeded = true
				}
			})
			if t.Failed() {
				return
			}
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("public proof export did not reach a successful allocation allowance", available)
		}
	}
	if failures == 0 {
		t.Fatal("public proof export did not exercise an allocation refusal")
	}
}

func testPDRProofExportFailure(t *testing.T, available int, stop int64, symbolic bool) bool {
	t.Helper()
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("public proof export fixture leaked storage", budget.Used())
		}
	})
	manager, globals := newPDRLifecycleServices(t)
	budget = manager.Budget()
	for _, setting := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker", "fixedpoint.pdr.validate_result"} {
		must(globals.SourceSetName(setting, "false"))
	}
	must(globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	must(err)
	t.Cleanup(func() { must(engine.Close()) })
	must(engine.SelectEngine("pdr"))
	boolean, err := manager.MkBoolSort()
	must(err)
	declare := func(name string, domain, arguments []ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
		symbol, cause := manager.Symbols.Intern(name)
		must(cause)
		declaration, cause := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(engine.RegisterPredicate(declaration))
		application, cause := manager.MkApp(declaration, arguments)
		must(cause)
		return declaration, application
	}
	var domain, arguments []ast.ASTHandle
	if symbolic {
		// The two rule-variable banks require a fresh shifted variable during
		// hyper-resolution, beyond the nullary proof-constructor frontier.
		zero, cause := manager.ArithmeticIntegerNumeralText("0")
		must(cause)
		zeroOwner, cause := manager.NewRef(zero)
		must(cause)
		t.Cleanup(func() { must(manager.DelRef(&zeroOwner)) })
		integer, cause := manager.GetSort(zero)
		must(cause)
		variable, cause := manager.MkVar(0, integer)
		must(cause)
		domain, arguments = []ast.ASTHandle{integer}, []ast.ASTHandle{variable}
	}
	_, premise := declare("proof-export-P", domain, arguments)
	goal, conclusion := declare("proof-export-Q", nil, nil)
	must(engine.addRuleEager(premise, "proof-export-fact"))
	edge, err := manager.MkImplies(premise, conclusion)
	must(err)
	must(engine.addRuleEager(edge, "proof-export-edge"))
	status, err := engine.QueryPredicate(goal)
	must(err)
	if status != lbool.True || engine.pdrProof != (ast.ASTRef{}) {
		t.Fatal("missing uncached, unsliced reachable query", status)
	}

	// Exhaust reference pages through the public owner API, then leave a
	// varying prefix for the formula/proof trail before its next physical page.
	// A fresh fixture at every allowance keeps the first exported proof cold.
	var pressure []ast.ASTRef
	t.Cleanup(func() {
		budget.FailAfter(-1)
		must(closeASTRefs(manager, pressure))
	})
	for {
		budget.FailAfter(0)
		for {
			reference, cause := manager.NewRef(ast.ASTHandle{})
			if cause != nil {
				budget.FailAfter(-1)
				if reference != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("reference-page allocation frontier", reference, cause)
				}
				break
			}
			pressure = append(pressure, reference)
		}
		if len(pressure) >= available {
			break
		}
		reference, cause := manager.NewRef(ast.ASTHandle{})
		must(cause)
		pressure = append(pressure, reference)
	}
	for i := 0; i < available; i++ {
		last := len(pressure) - 1
		must(manager.DelRef(&pressure[last]))
		pressure = pressure[:last]
	}
	before := manager.Stats()
	mode, err := manager.ProofMode()
	must(err)
	checkOwners := func(label string) {
		t.Helper()
		after := manager.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
			t.Fatal(label, "changed live AST ownership", before, after)
		}
		current, cause := manager.ProofMode()
		must(cause)
		if current != mode {
			t.Fatal(label, "changed the caller's proof mode", mode, current)
		}
		last, cause := engine.LastResult()
		must(cause)
		if last != lbool.True {
			t.Fatal(label, "invalidated the successful query", last)
		}
	}
	budget.FailAfter(stop)
	proof, cause := engine.Proof(goal)
	budget.FailAfter(-1)
	if cause != nil {
		if proof != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
			t.Fatal("failed proof export published a result or changed its cause", proof, cause)
		}
		var pending memory.ConstructionFailure
		if errors.As(cause, &pending) {
			must(pending.Cleanup())
		}
	} else {
		must(manager.DelRef(&proof))
	}
	checkOwners("allocation campaign")

	// Retry the same public query snapshot and validate the returned derivation.
	retry, err := engine.Proof(goal)
	must(err)
	proofHandle, err := manager.RefGet(retry)
	must(err)
	hyper, err := manager.IsProofKind(proofHandle, ast.ASTPrHyperResolve)
	must(err)
	fact, err := manager.GetProofFact(proofHandle)
	must(err)
	if !hyper || fact != conclusion {
		t.Fatal("retry did not prove the original query by hyper-resolution")
	}
	must(manager.DelRef(&retry))
	checkOwners("completed retry")
	if available == 9 && cause == nil {
		// Validate one fully owned export independently after checking the
		// export's exact live-owner accounting. The caller keeps this proof
		// across a subsequent query and final engine destruction.
		held, err := engine.Proof(goal)
		must(err)
		heldHandle, err := manager.RefGet(held)
		must(err)
		beforeValidation := manager.Stats()
		checker, err := proofs.NewChecker(manager)
		must(err)
		valid, side, err := checker.Check(heldHandle)
		must(err)
		if !valid {
			t.Fatal("retry proof failed independent structural checking")
		}
		solver, err := smtcore.NewSMTSolver(manager, nil, "", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
		must(err)
		for _, condition := range side {
			must(solver.Push())
			negation, err := manager.NewRef(ast.ASTHandle{})
			must(err)
			negated, err := manager.MkNot(condition)
			must(err)
			must(manager.SetRef(negation, negated))
			must(solver.Assert(negated))
			checked, err := solver.CheckSat(nil)
			must(err)
			must(solver.Pop(1))
			must(manager.DelRef(&negation))
			if checked != lbool.False {
				t.Fatal("retry proof has an invalid side condition", checked)
			}
		}
		must(solver.Close())
		must(checker.ReleaseSideConditions(side))
		afterValidation := manager.Stats()
		if afterValidation.LiveNodes != beforeValidation.LiveNodes || afterValidation.LiveReferences != beforeValidation.LiveReferences || afterValidation.LiveInfos != beforeValidation.LiveInfos || afterValidation.LiveParameters != beforeValidation.LiveParameters {
			t.Fatal("independent verifier changed live AST ownership", beforeValidation, afterValidation)
		}
		status, err = engine.QueryPredicate(goal)
		must(err)
		if status != lbool.True {
			t.Fatal("repeated query changed the reachable result", status)
		}
		must(engine.Close())
		must(engine.Close())
		current, err := manager.RefGet(held)
		must(err)
		hyper, err = manager.IsProofKind(current, ast.ASTPrHyperResolve)
		must(err)
		fact, err = manager.GetProofFact(current)
		must(err)
		if current != heldHandle || !hyper || fact != conclusion {
			t.Fatal("query replacement or engine closure invalidated the caller's proof")
		}
		must(manager.DelRef(&held))
	}
	return cause != nil
}
