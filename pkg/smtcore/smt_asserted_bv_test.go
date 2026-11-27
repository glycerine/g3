package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTAssertedBit2IntGuardsAndProofs(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, mode := range []string{"enabled", "disabled", "no-preprocessing"} {
			t.Run(fmt.Sprint(proofs, "/", mode), func(t *testing.T) {
				m, solver := smtAssertedNNFFixture(t, proofs)
				s, err := m.BitVectorSort(4)
				astTestMust(t, err)
				values := make([]ast.ASTHandle, 2)
				for i := range values {
					values[i], err = m.MkConst(astTestSymbol(t, m, fmt.Sprint("b2i-", i)), s)
					astTestMust(t, err)
					values[i], err = m.MkBitVectorToInt(values[i])
					astTestMust(t, err)
				}
				formula, err := m.MkEq(values[0], values[1])
				astTestMust(t, err)
				astTestMust(t, solver.Assert(formula))
				k := solver.kernel
				k.preprocessState.SimplifyBitToInt = mode != "disabled"
				k.preprocess = mode != "no-preprocessing"
				before := k.assertions[0].Formula
				astTestMust(t, k.applyPendingBit2Int())
				row := k.assertions[0]
				if changed := row.Formula != before; changed != (mode == "enabled") {
					t.Fatal("bit2int change", changed, mode)
				}
				if mode == "enabled" {
					a, err := m.GetArg(row.Formula, 0)
					astTestMust(t, err)
					sort, err := m.GetSort(a)
					astTestMust(t, err)
					if sort != s {
						t.Fatal("integer equality did not become bit-vector equality")
					}
				}
				if proofs {
					fact, err := m.GetProofFact(row.Proof)
					astTestMust(t, err)
					if fact != row.Formula {
						t.Fatal("bit2int assertion proof mismatch")
					}
				}
			})
		}
	}
}

func TestSMTAssertedBVSharingScopesAndProofs(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, mode := range []string{"enabled", "disabled", "no-preprocessing", "before-first-scope"} {
			t.Run(fmt.Sprint(proofs, "/", mode), func(t *testing.T) {
				m, solver := smtAssertedNNFFixture(t, proofs)
				// Keep the sharing circuits through the source post-operation
				// reduction rather than flattening them back into n-ary terms.
				params, err := solver.kernel.paramContext.NewValue()
				astTestMust(t, err)
				defer func() { astTestMust(t, solver.kernel.paramContext.Del(&params)) }()
				key := "flat"
				astTestMust(t, solver.kernel.paramContext.SetBoolName(&params, &key, false))
				astTestMust(t, solver.UpdateParams(&params))
				if mode != "before-first-scope" {
					astTestMust(t, solver.Push())
				}
				s, err := m.BitVectorSort(8)
				astTestMust(t, err)
				args := make([]ast.ASTHandle, 4)
				for i := range args {
					args[i], err = m.MkConst(astTestSymbol(t, m, fmt.Sprint("sharing-", i)), s)
					astTestMust(t, err)
				}
				family, err := m.RegisterBitVectorPlugin()
				astTestMust(t, err)
				term, err := m.MkFamilyApp(family, 31, nil, args)
				astTestMust(t, err)
				b, err := m.MkBoolSort()
				astTestMust(t, err)
				p := smtQuickPredicate(t, m, "sharing-p", s, b)
				formula, err := m.MkApp(p, []ast.ASTHandle{term})
				astTestMust(t, err)
				astTestMust(t, solver.Assert(formula))
				k := solver.kernel
				k.preprocessState.MaxBVSharing = mode != "disabled"
				k.preprocess = mode != "no-preprocessing"
				astTestMust(t, k.maximizePendingBVSharing())
				row := k.assertions[0]
				a, err := m.GetArg(row.Formula, 0)
				astTestMust(t, err)
				n, err := m.GetNumArgs(a)
				astTestMust(t, err)
				want := uint32(4)
				if mode == "enabled" {
					want = 2
				}
				if n != want {
					t.Fatal("sharing argument count", n, want)
				}
				if proofs {
					fact, err := m.GetProofFact(row.Proof)
					astTestMust(t, err)
					if fact != row.Formula {
						t.Fatal("sharing assertion proof mismatch")
					}
				}
				if mode != "before-first-scope" {
					astTestMust(t, solver.Pop(1))
					if len(k.assertions) != 0 {
						t.Fatal("sharing assertion survived scope pop")
					}
					// Reusing the scoped owner after rollback must not retain stale operands.
					astTestMust(t, solver.Push())
					astTestMust(t, solver.Pop(1))
				}
			})
		}
	}
}

// An unchanged early row can still need theory reduction when sharing
// changes a later row. The generic sharing traversal folds constant ITEs
// even before its first scope registers the associative operators.
func TestSMTInitialBVSharingReducesWholeChangedBatch(t *testing.T) {
	m, solver := smtAssertedNNFFixture(t, false)
	k := solver.kernel
	k.reduceIfSharingChanged = true
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkFreshConst("p", b)
	astTestMust(t, err)
	q, err := m.MkFreshConst("q", b)
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	neg, err := m.MkNot(p)
	astTestMust(t, err)
	twice, err := m.MkNot(neg)
	astTestMust(t, err)
	astTestMust(t, k.appendAssertion(twice, ast.ASTHandle{}))
	choice, err := m.MkIte(truth, q, p)
	astTestMust(t, err)
	astTestMust(t, k.appendAssertion(choice, ast.ASTHandle{}))
	astTestMust(t, k.maximizePendingBVSharing())
	if len(k.assertions) != 2 || k.assertions[0].Formula != p || k.assertions[1].Formula != q {
		t.Fatal("changed sharing batch did not reduce both assertions", k.assertions)
	}
}
