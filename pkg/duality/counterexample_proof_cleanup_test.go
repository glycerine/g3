package duality

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type proofModeCleanupWriter struct {
	callback func()
	run      bool
}

func (w *proofModeCleanupWriter) Write(p []byte) (int, error) {
	if !w.run {
		w.run = true
		w.callback()
	}
	return len(p), nil
}

func TestDualityProofExtractionCleanupRemainsGraphOwnedAcrossRefusals(t *testing.T) {
	ctx, manager, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	term, err := ctx.BoolConst("proof-extraction-cleanup")
	must(err)
	handle, err := term.Raw()
	must(err)
	keeper, err := manager.NewRef(handle)
	must(err)
	defer func() { must(manager.DelRef(&keeper)) }()
	owned, err := manager.NewRef(handle)
	must(err)
	call := &proofWitnessCall{manager: manager, refs: []ast.ASTRef{owned}, terms: []*Expr{term}}

	staleTerm := term.value.ref
	must(manager.DelRef(&term.value.ref))
	term.value.ref = staleTerm
	staleRef := call.refs[0]
	must(manager.DelRef(&call.refs[0]))
	call.refs[0] = staleRef
	if err = call.close(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("term cleanup refusal was not preserved", err)
	}
	if len(call.terms) != 1 || len(call.refs) != 1 {
		t.Fatal("term refusal surrendered later proof owners", len(call.terms), len(call.refs))
	}
	graph.pendingProofWitnesses = append(graph.pendingProofWitnesses, call)

	term.value.ref, err = manager.NewRef(handle)
	must(err)
	if err = graph.cleanupProofWitnesses(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("reference cleanup refusal was not preserved", err)
	}
	if len(graph.pendingProofWitnesses) != 1 || len(call.terms) != 0 || len(call.refs) != 1 {
		t.Fatal("reference refusal lost pending extraction owner", len(graph.pendingProofWitnesses), len(call.terms), len(call.refs))
	}
	call.refs[0], err = manager.NewRef(handle)
	must(err)
	must(graph.cleanupProofWitnesses())
	if len(graph.pendingProofWitnesses) != 0 || len(call.refs) != 0 || len(call.terms) != 0 {
		t.Fatal("proof extraction cleanup retry did not retire all owners")
	}
}

func TestDualityProofWitnessRetiresAlreadyConsumedTerm(t *testing.T) {
	ctx, manager, _ := dualityNativeSolverFixture(t)
	term, err := ctx.BoolConst("proof-consumed-term")
	if err != nil {
		t.Fatal(err)
	}
	// Consume the native owner before its wrapper observes the receipt. Close
	// then reports the zero-ref error while marking the wrapper closed.
	if err = manager.DelRef(&term.value.ref); err != nil || term.value.ref != (ast.ASTRef{}) || term.value.closed {
		t.Fatal("native term owner was not consumed independently", err, term.value.ref, term.value.closed)
	}
	call := &proofWitnessCall{manager: manager, terms: []*Expr{term}}
	if err = call.close(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed term retry did not expose its cleanup result", err)
	}
	if !term.value.closed || len(call.terms) != 0 {
		t.Fatal("consumed term remained on the retry frontier", len(call.terms))
	}
	if err = call.close(); err != nil {
		t.Fatal("retired term poisoned later cleanup retries", err)
	}
}

func TestDualityProofModeCleanupRemainsGraphOwnedAheadOfOutput(t *testing.T) {
	ctx, manager, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(manager.ToggleProofMode(ast.ASTProofDisabled))
	scope, err := ast.NewScopedProof(manager)
	must(err)
	term, err := ctx.BoolConst("proof-mode-output")
	must(err)
	defer func() { must(term.Close()) }()
	handle, err := term.Raw()
	must(err)
	output, err := manager.NewRef(handle)
	must(err)
	innerTerm, err := ctx.BoolConst("proof-mode-inner-refusal")
	must(err)
	must(manager.DelRef(&innerTerm.value.ref))
	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	scopeCall := &proofWitnessCall{manager: manager, proofScope: scope, refs: []ast.ASTRef{output}}
	innerCall := &proofWitnessCall{manager: manager, terms: []*Expr{innerTerm}}

	var scopeErr error
	writer := &proofModeCleanupWriter{callback: func() {
		// Recursive extraction cleanup is queued before the outer defer runs.
		graph.pendingProofWitnesses = append(graph.pendingProofWitnesses, innerCall)
		scopeErr = scopeCall.close()
		if scopeErr != nil {
			graph.insertPendingProofWitness(0, scopeCall)
		}
	}}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget(), BorrowedWriter: writer})
	must(err)
	must(manager.DisplayBoundedLowLevel(stream, handle, 1))
	must(stream.Close())
	if !writer.run || !errors.Is(scopeErr, ast.ErrASTPending) {
		t.Fatal("proof scope cleanup did not refuse under active AST formatting", writer.run, scopeErr)
	}
	if scopeCall.proofScope == nil || len(scopeCall.refs) != 1 || len(graph.pendingProofWitnesses) != 2 || graph.pendingProofWitnesses[0] != scopeCall || graph.pendingProofWitnesses[1] != innerCall {
		t.Fatal("refused scope did not move ahead of recursive proof cleanup")
	}
	if err = graph.cleanupProofWitnesses(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("inner native refusal was not reached after scope restoration", err)
	}
	if scopeCall.proofScope != nil || len(scopeCall.refs) != 0 || len(graph.pendingProofWitnesses) != 1 || graph.pendingProofWitnesses[0] != innerCall || len(innerCall.terms) != 0 {
		t.Fatal("scope was not retired before the refusing inner owner")
	}
	mode, err := manager.ProofMode()
	must(err)
	if mode != ast.ASTProofDisabled {
		t.Fatal("proof mode remained enabled after cleanup retry", mode)
	}
	if _, err = manager.RefGet(output); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("output was not released after proof mode restoration", err)
	}
	must(graph.cleanupProofWitnesses())
	if len(graph.pendingProofWitnesses) != 0 {
		t.Fatal("retired inner refusal poisoned the next cleanup retry")
	}
}

var _ io.Writer = (*proofModeCleanupWriter)(nil)
