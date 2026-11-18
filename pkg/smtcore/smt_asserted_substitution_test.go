package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func smtHoldSubstitutionInput(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) {
	t.Helper()
	r, err := m.NewRef(h)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&r)) })
}

func TestSMTAssertedCommittedBooleanBindingRewritesNextAssertion(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, negative := range []bool{false, true} {
			m, s := smtAssertedNNFFixture(t, proofs)
			smtDisableAutoConfig(t, s)
			a := smtRelevancyBool(t, m, "committed-boolean-a")
			notA, err := m.MkNot(a)
			astTestMust(t, err)
			smtHoldSubstitutionInput(t, m, notA)
			first, opposite := a, notA
			if negative {
				first, opposite = notA, a
			}
			astTestMust(t, s.Assert(first))
			astTestMust(t, s.kernel.internalizePending())
			// No subsequent reduction/check is allowed before this observation.
			astTestMust(t, s.Assert(opposite))
			k := s.kernel
			if !k.assertedInconsistent || !m.IsFalse(k.assertions[len(k.assertions)-1].Formula) {
				t.Fatal("committed binding did not affect assertion-time rewriting", proofs, negative)
			}
			if proofs {
				p, err := k.AssertionInconsistencyProof()
				astTestMust(t, err)
				fact, err := m.GetProofFact(p)
				astTestMust(t, err)
				if !m.IsFalse(fact) {
					t.Fatal("committed Boolean substitution lost its composed proof")
				}
			}
		}
	}
}

func TestSMTAssertedCommittedNumeralBindingNestedRollback(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		m, s := smtAssertedNNFFixture(t, proofs)
		smtDisableAutoConfig(t, s)
		seven, err := m.ArithmeticIntegerNumeralText("7")
		astTestMust(t, err)
		sort, err := m.GetSort(seven)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "committed-numeral-x"), sort)
		astTestMust(t, err)
		smtHoldSubstitutionInput(t, m, x)
		eight, err := m.ArithmeticIntegerNumeralText("8")
		astTestMust(t, err)
		x7, err := m.MkEq(x, seven)
		astTestMust(t, err)
		x8, err := m.MkEq(x, eight)
		astTestMust(t, err)
		smtHoldSubstitutionInput(t, m, x8)
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(x7))
		astTestMust(t, s.kernel.internalizePending())
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(x8))
		if !s.kernel.assertedInconsistent {
			t.Fatal("numeral binding was not retained through nested push")
		}
		astTestMust(t, s.Pop(1))
		astTestMust(t, s.Assert(x8))
		if !s.kernel.assertedInconsistent {
			t.Fatal("inner pop erased the outer committed binding")
		}
		astTestMust(t, s.Pop(1))
		astTestMust(t, s.Assert(x8))
		if s.kernel.assertedInconsistent || len(s.kernel.assertions) != 1 || m.IsFalse(s.kernel.assertions[0].Formula) {
			t.Fatal("outer pop retained a stale committed numeral binding")
		}
		level, err := m.ScopedExprSubstitutionScopeLevel(s.kernel.assertedScopedSubst)
		astTestMust(t, err)
		if level != s.kernel.ScopeLevel() {
			t.Fatal("substitution scopes diverged from assertion scopes", level, s.kernel.ScopeLevel())
		}
	}
}

func TestSMTAssertedTemporaryPropagationBindingsDoNotEscape(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		m, s := smtAssertedNNFFixture(t, proofs)
		a := smtRelevancyBool(t, m, "temporary-substitution-a")
		b := smtRelevancyBool(t, m, "temporary-substitution-b")
		notA, err := m.MkNot(a)
		astTestMust(t, err)
		clause, err := m.MkOr(notA, b)
		astTestMust(t, err)
		astTestMust(t, s.Assert(a))
		astTestMust(t, s.Assert(clause))
		astTestMust(t, s.kernel.propagatePendingAssertions())
		if len(s.kernel.assertions) != 2 || s.kernel.assertions[1].Formula != b {
			t.Fatal("temporary propagation did not simplify the pending suffix")
		}
		empty, err := m.ExprSubstitutionEmpty(s.kernel.assertedSubstitution)
		astTestMust(t, err)
		if !empty || s.kernel.assertedHead != 0 {
			t.Fatal("uncommitted direction bindings escaped their temporary scope")
		}
		notB, err := m.MkNot(b)
		astTestMust(t, err)
		astTestMust(t, s.Assert(notB))
		if s.kernel.assertedInconsistent {
			t.Fatal("a later assertion saw an uncommitted propagation binding")
		}
	}
}

func TestSMTAssertedEmptyPropagationRestoresBorrowedSubstitution(t *testing.T) {
	m, s := smtAssertedNNFFixture(t, false)
	a := smtRelevancyBool(t, m, "empty-propagation-binding-a")
	astTestMust(t, s.Assert(a))
	astTestMust(t, s.kernel.internalizePending())
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	smtHoldSubstitutionInput(t, m, notA)
	astTestMust(t, s.kernel.preprocessor.Reset())
	astTestMust(t, s.kernel.propagatePendingAssertions())
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	astTestMust(t, s.kernel.preprocessor.Rewrite(notA, out))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if !m.IsFalse(got) {
		t.Fatal("empty source propagation skipped its cache flush/reattachment")
	}
	level, err := m.ScopedExprSubstitutionScopeLevel(s.kernel.assertedScopedSubst)
	astTestMust(t, err)
	if level != 0 {
		t.Fatal("empty propagation leaked a temporary scope", level)
	}
}

func TestSMTAssertedSubstitutionAllocationFrontiers(t *testing.T) {
	for _, mode := range []string{"constructor", "commit", "propagate"} {
		for _, proofs := range []bool{false, true} {
			t.Run(fmt.Sprint(mode, "/", proofs), func(t *testing.T) {
				succeeded := false
				for stop := int64(0); stop < 2048 && !succeeded; stop++ {
					t.Run(fmt.Sprint(stop), func(t *testing.T) {
						var m *ast.ASTManager
						var k *SMTKernel
						if mode == "constructor" {
							m = astTestManager(t)
							if proofs {
								astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
							}
						} else {
							var s *SMTSolver
							m, s = smtAssertedNNFFixture(t, proofs)
							k = s.kernel
							a := smtRelevancyBool(t, m, "frontier-substitution-a")
							astTestMust(t, s.Assert(a))
							if mode == "propagate" {
								b := smtRelevancyBool(t, m, "frontier-substitution-b")
								notA, err := m.MkNot(a)
								astTestMust(t, err)
								clause, err := m.MkOr(notA, b)
								astTestMust(t, err)
								astTestMust(t, s.Assert(clause))
							}
						}
						m.Budget().FailAfter(stop)
						var err error
						switch mode {
						case "constructor":
							k, err = NewSMTKernel(m, nil)
						case "commit":
							err = k.commitAssertedTo(len(k.assertions))
						case "propagate":
							err = k.propagatePendingAssertions()
						}
						m.Budget().FailAfter(-1)
						if mode == "constructor" && k != nil {
							astTestMust(t, k.Close())
						}
						if err == nil {
							succeeded = true
						} else if !errors.Is(err, arena.ErrInjectedFailure) {
							var failure umemory.Failure
							if !errors.As(err, &failure) || failure.Kind != umemory.FailureMemout {
								t.Fatal(err)
							}
						}
					})
					if t.Failed() {
						t.Fatalf("allocation frontier %d", stop)
					}
				}
				if !succeeded {
					t.Fatal("no successful substitution allocation frontier")
				}
			})
		}
	}
}

// AUDIT.md smtcore_01-F10: update_substitution only traces "incompatible"
// for a ground equality neither side of which is greater, then falls
// through and records n -> true.
func TestSMTAssertedSubstitutionIncompatibleEqualityFallsThrough(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	a := smtRelevancyBool(t, m, "incompatible-eq-a")
	eq, err := m.MkEq(a, a)
	astTestMust(t, err)
	smtHoldSubstitutionInput(t, m, eq)
	astTestMust(t, k.addAssertionSubstitution(SMTKernelAssertion{Formula: eq}, map[ast.ASTHandle]uint32{}))
	var def, proof ast.ASTHandle
	found, err := m.ExprSubstitutionFindProof(k.assertedSubstitution, eq, &def, &proof)
	astTestMust(t, err)
	if !found || !m.IsTrue(def) {
		t.Fatal("incompatible equality was not recorded as n -> true", found, def)
	}
}
