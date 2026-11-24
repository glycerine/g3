package smtcore

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestSMTAssertedMacroSwapPrecedesCanceledPostReduction(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		m, s := smtAssertedNNFFixture(t, proofs)
		b, err := m.MkBoolSort()
		astTestMust(t, err)
		p, err := m.MkConst(astTestSymbol(t, m, "macro-swap-original"), b)
		astTestMust(t, err)
		astTestMust(t, s.Assert(p))
		batch, err := macros.NewFormulaBatch(m)
		astTestMust(t, err)
		defer func() { astTestMust(t, batch.Close()) }()
		var expected []SMTKernelAssertion
		for _, name := range []string{"macro-swap-q", "macro-swap-r"} {
			q, err := m.MkConst(astTestSymbol(t, m, name), b)
			astTestMust(t, err)
			proof := ast.ASTHandle{}
			if proofs {
				proof, err = m.MkProofAsserted(q)
				astTestMust(t, err)
			}
			astTestMust(t, batch.Insert(macros.Formula{Expression: q, Proof: proof}))
			expected = append(expected, SMTKernelAssertion{Formula: q, Proof: proof})
		}
		m.Limit.Cancel()
		err = s.kernel.replacePendingMacroBatch(batch)
		m.Limit.ResetCancel()
		if err != nil && err.Error() != messages.Canceled {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s.kernel.Assertions(), expected) {
			t.Fatal("post-operation cancellation rolled back the source macro swap")
		}
	}
}

func TestSMTAssertedPropagationMarksUnchangedFalseFromRawSwap(t *testing.T) {
	m, s := smtAssertedNNFFixture(t, false)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	batch, err := macros.NewFormulaBatch(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, batch.Close()) }()
	astTestMust(t, batch.Insert(macros.Formula{Expression: falsity}))
	astTestMust(t, s.kernel.swapPendingFormulaBatch(batch))
	if s.kernel.assertedInconsistent {
		t.Fatal("raw source swap must not act as push_assertion")
	}
	s.kernel.preprocessState.PropagateValues = true
	astTestMust(t, s.kernel.propagatePendingAssertions())
	if !s.kernel.assertedInconsistent {
		t.Fatal("unchanged false row did not establish propagation inconsistency")
	}
}

func TestSMTAssertedReductionPreservesUnchangedRowsAndEagerConflict(t *testing.T) {
	fixture := func() (*ast.ASTManager, *SMTSolver) {
		m, s := smtAssertedNNFFixture(t, false)
		astTestMust(t, s.Push())
		b, err := m.MkBoolSort()
		astTestMust(t, err)
		p, err := m.MkConst(astTestSymbol(t, m, "eager-conflict-p"), b)
		astTestMust(t, err)
		np, err := m.MkNot(p)
		astTestMust(t, err)
		contradiction, err := m.MkAnd(p, np)
		astTestMust(t, err)
		batch, err := macros.NewFormulaBatch(m)
		astTestMust(t, err)
		defer func() { astTestMust(t, batch.Close()) }()
		astTestMust(t, batch.Insert(macros.Formula{Expression: contradiction}))
		r, err := m.MkConst(astTestSymbol(t, m, "eager-conflict-r"), b)
		astTestMust(t, err)
		astTestMust(t, batch.Insert(macros.Formula{Expression: r}))
		astTestMust(t, s.kernel.swapPendingFormulaBatch(batch))
		return m, s
	}
	m, s := fixture()
	start := m.Limit.Count()
	astTestMust(t, s.kernel.reducePendingAssertions())
	steps := m.Limit.Count() - start
	if steps < 2 || len(s.kernel.assertions) != 2 || !m.IsFalse(s.kernel.assertions[0].Formula) {
		t.Fatal("unchanged trailing row was discarded after the first row became false")
	}
	m, s = fixture()
	original := s.kernel.Assertions()
	scope, err := control.NewLimitScope(&m.Limit, uint32(steps-1))
	astTestMust(t, err)
	astTestMust(t, s.kernel.reducePendingAssertions())
	if !reflect.DeepEqual(s.kernel.Assertions(), original) || !s.kernel.assertedInconsistent {
		t.Fatal("canceled vector must stay unpublished while source eager inconsistency survives")
	}
	scope.Close()
	astTestMust(t, s.Pop(1))
	if s.kernel.assertedInconsistent {
		t.Fatal("scope pop did not restore the original inconsistency flag")
	}
}

func TestSMTAssertedPublicationAllocationFrontiers(t *testing.T) {
	for _, mode := range []string{"injectivity", "nnf"} {
		for _, proofs := range []bool{false, true} {
			t.Run(fmt.Sprint(mode, "/", proofs), func(t *testing.T) {
				succeeded := false
				for stop := int64(0); stop < 2048 && !succeeded; stop++ {
					t.Run(fmt.Sprint(stop), func(t *testing.T) {
						m, s, q, _, _ := smtAssertedInjectivityFixture(t, proofs)
						if mode == "nnf" {
							nq, err := m.MkNot(q)
							astTestMust(t, err)
							astTestMust(t, s.Assert(nq))
							s.kernel.preprocessState.NNFCNF = true
						}
						original := s.kernel.Assertions()
						m.Budget().FailAfter(stop)
						var err error
						if mode == "nnf" {
							err = s.kernel.normalizePendingAssertions()
						} else {
							err = s.kernel.refinePendingInjectivityAxioms()
						}
						m.Budget().FailAfter(-1)
						if err == nil {
							succeeded = true
							return
						}
						if !errors.Is(err, arena.ErrInjectedFailure) {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(original, s.kernel.Assertions()) {
							t.Fatal("stopped transformation published its local vector")
						}
					})
					if t.Failed() {
						t.Fatalf("allocation frontier %d", stop)
					}
				}
				if !succeeded {
					t.Fatal("no successful allocation frontier")
				}
			})
		}
	}
}

func TestSMTAssertedInjectivityCanceledRowsDoNotPublish(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		m, s, q, _, _ := smtAssertedInjectivityFixture(t, proofs)
		astTestMust(t, s.Assert(q))
		original := s.kernel.Assertions()
		// This simplifier has no rewriter-engine resource polls; the source
		// generic loop polls once per row. Let row one finish, then stop at
		// row two after both transformed results have been built locally.
		scope, err := control.NewLimitScope(&m.Limit, 1)
		astTestMust(t, err)
		astTestMust(t, s.kernel.refinePendingInjectivityAxioms())
		if !reflect.DeepEqual(s.kernel.Assertions(), original) {
			t.Fatal("canceled pass published transformed rows")
		}
		scope.Close()
		astTestMust(t, s.kernel.refinePendingInjectivityAxioms())
		if reflect.DeepEqual(s.kernel.Assertions(), original) {
			t.Fatal("uncanceled retry did not refine the original pending rows")
		}
	}
}

func TestSMTAssertedNNFCancellationBeforeSwapAndRetry(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		fixture := func() (*ast.ASTManager, *SMTSolver) {
			m, s, q, _, _ := smtAssertedInjectivityFixture(t, proofs)
			nq, err := m.MkNot(q)
			astTestMust(t, err)
			astTestMust(t, s.Assert(nq))
			s.kernel.preprocessState.NNFCNF = true
			return m, s
		}
		m, s := fixture()
		start := m.Limit.Count()
		astTestMust(t, s.kernel.normalizePendingAssertions())
		steps := m.Limit.Count() - start
		if steps < 2 {
			t.Fatal("missing NNF normalization/rewrite cancellation checks", steps)
		}
		m, s = fixture()
		original := s.kernel.Assertions()
		scope, err := control.NewLimitScope(&m.Limit, uint32(steps-1))
		astTestMust(t, err)
		// Exhaust the resource budget at the final row's post-rewrite poll,
		// after normalization and preceding rows have completed successfully.
		astTestMust(t, s.kernel.normalizePendingAssertions())
		if !reflect.DeepEqual(s.kernel.Assertions(), original) {
			t.Fatal("canceled NNF published partially normalized formulas")
		}
		scope.Close()
		astTestMust(t, s.kernel.normalizePendingAssertions())
		if reflect.DeepEqual(s.kernel.Assertions(), original) {
			t.Fatal("NNF retry did not normalize the original pending formulas")
		}
	}
}

func TestSMTAssertedTermITEDefinitionPreservesUnchangedConjunction(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		x := smtNewAssertedITEFixture(t, proofs, false, false)
		k := x.solver.kernel
		astTestMust(t, x.solver.Assert(x.formula))
		k.preprocessState.EliminateTermITE = true
		// Calling the pass directly before the driver's post-NNF phase
		// leaves elim_and false. The appended definition is unchanged by
		// ordinary rewriting, so source simplify_fmls preserves it verbatim.
		astTestMust(t, k.eliminatePendingTermITEs())
		if len(k.assertions) != 2 {
			t.Fatal("unchanged conjunctive definition was flattened", len(k.assertions))
		}
		op, basic, err := k.internalizer.basicOperation(k.assertions[1].Formula)
		astTestMust(t, err)
		if !basic || op != ast.ASTOpAnd {
			t.Fatal("source appended conjunction was not preserved")
		}
		if proofs {
			for _, row := range k.assertions {
				fact, err := x.m.GetProofFact(row.Proof)
				astTestMust(t, err)
				if fact != row.Formula {
					t.Fatal("preserved formula lost its justification")
				}
			}
		}
	}
}

func TestSMTAssertedBatchKeepsPrefixOwners(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	var terms [4]ast.ASTHandle
	for i := range terms {
		terms[i], err = m.MkVar(uint32(i), sort)
		astTestMust(t, err)
		astTestMust(t, m.IncRef(terms[i]))
	}
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := m.MkProofAsserted(terms[1])
	astTestMust(t, err)
	astTestMust(t, m.IncRef(proof))
	astTestMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	handles := append(terms[:], proof)
	base := make([]uint32, len(handles))
	for i, h := range handles {
		base[i], err = m.GetRefCount(h)
		astTestMust(t, err)
	}
	k := &SMTKernel{manager: m}
	t.Cleanup(func() {
		astTestMust(t, k.restoreAssertions(0))
		for _, h := range handles {
			astTestMust(t, m.DecRef(h))
		}
	})
	committed := SMTKernelAssertion{Formula: terms[0]}
	astTestMust(t, k.appendAssertion(committed.Formula, committed.Proof))
	k.assertedHead = 1
	a := SMTKernelAssertion{Formula: terms[1]}
	b := SMTKernelAssertion{Formula: terms[2]}
	c := SMTKernelAssertion{Formula: terms[3]}
	proved := SMTKernelAssertion{Formula: terms[1], Proof: proof}
	for _, pending := range [][]SMTKernelAssertion{
		{a, b}, {a, b}, {a, b, c}, {a, c}, {proved, c}, {proved, a, a}, {proved}, {},
	} {
		batch, err := macros.NewFormulaBatch(m)
		astTestMust(t, err)
		for _, row := range pending {
			astTestMust(t, batch.Insert(macros.Formula{Expression: row.Formula, Proof: row.Proof}))
		}
		astTestMust(t, k.swapPendingFormulaBatch(batch))
		astTestMust(t, batch.Close())
		want := append([]SMTKernelAssertion{committed}, pending...)
		if !reflect.DeepEqual(k.assertions, want) || k.assertedHead != 1 {
			t.Fatalf("published %v with head %d, want %v with head 1", k.assertions, k.assertedHead, want)
		}
		for i, h := range handles {
			count := base[i]
			for _, row := range want {
				if row.Formula == h || row.Proof == h {
					count++
				}
			}
			got, err := m.GetRefCount(h)
			astTestMust(t, err)
			if got != count {
				t.Fatalf("handle %v after publishing %v has %d owners, want %d", h, pending, got, count)
			}
		}
	}

	// The second row cannot acquire its stale proof. The first row is
	// shared with the old batch, and the old tail must survive the retry.
	astTestMust(t, k.appendAssertion(a.Formula, a.Proof))
	astTestMust(t, k.appendAssertion(b.Formula, b.Proof))
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	stale, err := m.MkProofHypothesis(c.Formula)
	astTestMust(t, err)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	batch := newSwapBatch(t, m)
	astTestMust(t, batch.Insert(macros.Formula{Expression: a.Formula}))
	astTestMust(t, batch.Insert(macros.Formula{Expression: c.Formula, Proof: stale}))
	astTestMust(t, m.DecRef(stale))
	want := []SMTKernelAssertion{committed, a, b}
	if err := k.swapPendingFormulaBatch(batch); !errors.Is(err, ast.ErrASTHandle) || !reflect.DeepEqual(k.assertions, want) {
		t.Fatalf("failed swap left %v, err %v", k.assertions, err)
	}
	if err := batch.Reset(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("stale batch reset returned %v", err)
	}
	astTestMust(t, batch.Insert(macros.Formula{Expression: a.Formula}))
	astTestMust(t, k.swapPendingFormulaBatch(batch))
	if !reflect.DeepEqual(k.assertions, want[:2]) {
		t.Fatalf("retry after a stale proof left %v", k.assertions)
	}
}
