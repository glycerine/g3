package base_test

// Source: src/muz/base/hnf.cpp imp::reset/get_fresh_predicates and the
// rule_manager-owned hnf member in dl_rule.h/dl_rule.cpp.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/symbol"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestHornNormalizerOwnsFreshPredicatesAfterResultCloses(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		name := "no-proof"
		if withProof {
			name = "proof"
		}
		t.Run(name, func(t *testing.T) {
			f := newRuleSetFixture(t, "P", "Q", "R", "H")
			m := f.manager
			disjunction, err := m.MkOr(f.app["P"], f.app["Q"])
			ruleMust(t, err)
			body, err := m.MkAnd(disjunction, f.app["R"])
			ruleMust(t, err)
			formula, err := m.MkImplies(body, f.app["H"])
			ruleMust(t, err)
			pin, err := m.NewRef(formula)
			ruleMust(t, err)
			defer func() { ruleMust(t, m.DelRef(&pin)) }()
			normalizer, err := base.NewHornNormalizer(m, "lifecycle")
			ruleMust(t, err)
			t.Cleanup(func() { ruleMust(t, normalizer.Close()) })
			var result *base.HornNormalization
			if withProof {
				ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				p, err := m.MkProofAsserted(formula)
				ruleMust(t, err)
				ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
				result, err = normalizer.NormalizeWithProof(formula, p)
			} else {
				result, err = normalizer.Normalize(formula)
			}
			ruleMust(t, err)
			if len(result.FreshPredicates) != 1 {
				t.Fatal("expected one fresh predicate")
			}
			declaration, err := m.RefGet(result.FreshPredicates[0])
			ruleMust(t, err)
			ruleMust(t, result.Close())
			if arity, err := m.GetArity(declaration); err != nil || arity != 0 {
				t.Fatal("normalizer lost its source-owned fresh declaration", arity, err)
			}
		})
	}
}

func TestHornNormalizerResetNameAndOwningSnapshots(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R", "H")
	m := f.manager
	disjunction, err := m.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	body, err := m.MkAnd(disjunction, f.app["R"])
	ruleMust(t, err)
	formula, err := m.MkImplies(body, f.app["H"])
	ruleMust(t, err)
	pin, err := m.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&pin)) }()
	normalizer, err := base.NewHornNormalizer(m, "first")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, normalizer.Close()) })
	first, err := normalizer.Normalize(formula)
	ruleMust(t, err)
	snapshots, err := normalizer.FreshPredicates()
	ruleMust(t, err)
	if len(snapshots) != 1 {
		t.Fatal("source fresh declaration snapshot", len(snapshots))
	}
	old, err := m.RefGet(snapshots[0])
	ruleMust(t, err)
	ruleMust(t, first.Close())
	// Source's already-Horn shortcut precedes work/cache reset.
	horn, err := m.MkImplies(f.app["P"], f.app["H"])
	ruleMust(t, err)
	unchanged, err := normalizer.Normalize(horn)
	ruleMust(t, err)
	ruleMust(t, unchanged.Close())
	still, err := normalizer.FreshPredicates()
	ruleMust(t, err)
	if len(still) != 1 {
		t.Fatal("already-Horn invocation cleared the source cache")
	}
	actual, err := m.RefGet(still[0])
	ruleMust(t, err)
	if actual != old {
		t.Fatal("already-Horn invocation replaced the source fresh predicate")
	}
	ruleMust(t, m.DelRef(&still[0]))
	ruleMust(t, normalizer.Reset())
	empty, err := normalizer.FreshPredicates()
	ruleMust(t, err)
	if len(empty) != 0 {
		t.Fatal("reset left cached fresh predicates")
	}
	if _, err := m.GetArity(old); err != nil {
		t.Fatal("reset invalidated an owning fresh-predicate snapshot", err)
	}
	ruleMust(t, m.DelRef(&snapshots[0]))
	if _, err := m.GetArity(old); err == nil {
		t.Fatal("reset kept an unowned old fresh declaration alive")
	}
	prefix, err := m.Symbols.Intern("second")
	ruleMust(t, err)
	ruleMust(t, normalizer.SetName(prefix))
	second, err := normalizer.Normalize(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, second.Close()) }()
	secondDecl, err := m.RefGet(second.FreshPredicates[0])
	ruleMust(t, err)
	name, err := m.GetASTName(secondDecl)
	ruleMust(t, err)
	text, err := m.Symbols.String(name)
	ruleMust(t, err)
	if !strings.HasPrefix(text, "second") {
		t.Fatal("SetName did not select the new definition prefix", text)
	}
	ruleMust(t, normalizer.Reset())
	ruleMust(t, normalizer.SetName(symbol.Symbol{}))
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	original, err := m.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	third, err := normalizer.NormalizeWithProof(formula, original)
	ruleMust(t, err)
	defer func() { ruleMust(t, third.Close()) }()
	thirdDecl, err := m.RefGet(third.FreshPredicates[0])
	ruleMust(t, err)
	name, err = m.GetASTName(thirdDecl)
	ruleMust(t, err)
	text, err = m.Symbols.String(name)
	ruleMust(t, err)
	if !strings.HasPrefix(text, "P") {
		t.Fatal("null name did not restore source P prefix", text)
	}
	ruleMust(t, normalizer.Close())
	// Output formula/proof ownership is independent of the destroyed normalizer.
	for _, result := range []*base.HornNormalization{second, third} {
		for i, ref := range result.Formulas {
			formula, err := m.RefGet(ref)
			ruleMust(t, err)
			_, err = m.GetKind(formula)
			ruleMust(t, err)
			if len(result.Proofs) != 0 {
				p, err := m.RefGet(result.Proofs[i])
				ruleMust(t, err)
				fact, err := m.GetProofFact(p)
				ruleMust(t, err)
				if fact != formula {
					t.Fatal("closed normalizer changed output proof fact")
				}
			}
		}
	}
	if _, err = normalizer.Normalize(formula); !errors.Is(err, base.ErrHornNormalizationOwner) {
		t.Fatal("closed normalizer accepted invocation", err)
	}
}

func TestHornNormalizerResetPreservesCurrentBodyAfterBoundedWorklist(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		name := "no-proof"
		if withProof {
			name = "proof"
		}
		t.Run(name, func(t *testing.T) {
			f := newRuleSetFixture(t, "P", "Q", "R", "H")
			m := f.manager
			disjunction, err := m.MkOr(f.app["P"], f.app["Q"])
			ruleMust(t, err)
			body, err := m.MkAnd(disjunction, f.app["R"])
			ruleMust(t, err)
			formula, err := m.MkImplies(body, f.app["H"])
			ruleMust(t, err)
			pin, err := m.NewRef(formula)
			ruleMust(t, err)
			defer func() { ruleMust(t, m.DelRef(&pin)) }()
			normalizer, err := base.NewHornNormalizer(m, "bounded-owner")
			ruleMust(t, err)
			t.Cleanup(func() { ruleMust(t, normalizer.Close()) })
			var proof ast.ASTHandle
			if withProof {
				ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				proof, err = m.MkProofAsserted(formula)
				ruleMust(t, err)
				ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
			}
			var result *base.HornNormalization
			func() {
				ruleMust(t, m.Limit.Push(1))
				defer m.Limit.Pop()
				if withProof {
					result, err = normalizer.NormalizeWithProof(formula, proof)
				} else {
					result, err = normalizer.Normalize(formula)
				}
				ruleMust(t, err)
			}()
			if len(result.Formulas) != 1 || len(result.FreshPredicates) != 1 {
				t.Fatal("expected one processed main clause before cancellation", len(result.Formulas), len(result.FreshPredicates))
			}
			old, err := m.RefGet(result.FreshPredicates[0])
			ruleMust(t, err)
			ruleMust(t, result.Close())
			ruleMust(t, normalizer.Reset())
			cached, err := normalizer.FreshPredicates()
			ruleMust(t, err)
			if len(cached) != 0 {
				t.Fatal("reset did not clear fresh declaration vector")
			}
			if _, err = m.GetArity(old); err != nil {
				t.Fatal("reset destroyed the source current-body root", err)
			}
			nextFormula, err := m.MkImplies(disjunction, f.app["H"])
			ruleMust(t, err)
			next, err := normalizer.Normalize(nextFormula)
			ruleMust(t, err)
			defer func() { ruleMust(t, next.Close()) }()
			if _, err = m.GetArity(old); err == nil {
				t.Fatal("next processing step did not release the previous body root")
			}
		})
	}
}

// C++ m_todo/m_proofs survive cancellation and the already-Horn fast path.
// Reset releases them even when no output clause was produced.
func TestHornNormalizerOwnsCanceledWorkUntilReset(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		for _, split := range []bool{false, true} {
			t.Run(fmt.Sprintf("proof=%v/split=%v", withProof, split), func(t *testing.T) {
				f := newRuleSetFixture(t, "P", "Q", "H")
				m := f.manager
				for _, app := range f.app {
					ref, err := m.NewRef(app)
					ruleMust(t, err)
					t.Cleanup(func() { ruleMust(t, m.DelRef(&ref)) })
				}
				disjunction, err := m.MkOr(f.app["P"], f.app["Q"])
				ruleMust(t, err)
				formula, err := m.MkImplies(disjunction, f.app["H"])
				ruleMust(t, err)
				input, err := m.NewRef(formula)
				ruleMust(t, err)

				pending := []ast.ASTHandle{formula}
				var pins []ast.ASTRef
				if split {
					pending = nil
					for _, name := range []string{"P", "Q"} {
						clause, err := m.MkImplies(f.app[name], f.app["H"])
						ruleMust(t, err)
						ref, err := m.NewRef(clause)
						ruleMust(t, err)
						pins = append(pins, ref)
						pending = append(pending, clause)
					}
				}
				var proof ast.ASTHandle
				if withProof {
					ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					proof, err = m.MkProofAsserted(formula)
					ruleMust(t, err)
					ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
					if !split {
						pending = append(pending, proof)
					}
				}
				n, err := base.NewHornNormalizer(m, "pending")
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, n.Close()) })
				var result *base.HornNormalization
				func() {
					if split {
						ruleMust(t, m.Limit.Push(1))
						defer m.Limit.Pop()
					} else {
						m.Limit.Cancel()
						defer m.Limit.ResetCancel()
					}
					if withProof {
						result, err = n.NormalizeWithProof(formula, proof)
					} else {
						result, err = n.Normalize(formula)
					}
					ruleMust(t, err)
				}()
				if len(result.Formulas) != 0 {
					t.Fatal("canceled work unexpectedly produced output")
				}
				ruleMust(t, result.Close())
				for i := range pins {
					ruleMust(t, m.DelRef(&pins[i]))
				}
				ruleMust(t, m.DelRef(&input))
				// Already-Horn input leaves the canceled queue intact.
				horn, err := m.MkImplies(f.app["H"], f.app["P"])
				ruleMust(t, err)
				fast, err := n.Normalize(horn)
				ruleMust(t, err)
				for _, h := range pending {
					if _, err := m.GetKind(h); err != nil {
						t.Fatal("cancellation lost source queued expression/proof", err)
					}
				}
				ruleMust(t, fast.Close())
				ruleMust(t, n.Reset())
				for _, h := range pending {
					if _, err := m.GetKind(h); err == nil {
						t.Fatal("reset left queued expression/proof alive")
					}
				}
			})
		}
	}
}
