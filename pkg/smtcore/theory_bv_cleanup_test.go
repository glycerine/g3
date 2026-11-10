package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTBVCleanupPreservesNativeRefusedRoots(t *testing.T) {
	// After internalization m_bits holds literals only, so no bit owner is
	// left for a refused close to preserve; the temporaries remain.
	for _, operation := range []string{"temporary", "close-temporary"} {
		t.Run(operation, func(t *testing.T) {
			var bv *SMTBitVectorTheory
			var failure error
			m, _, quantifiers, trigger := smtQuantifierRefusalFixture(t, func() {
				if operation == "temporary" {
					failure = bv.releaseTemporary()
				} else {
					failure = bv.Close()
				}
			})
			state := quantifiers.state
			family, err := m.RegisterBitVectorPlugin()
			astTestMust(t, err)
			bv, err = NewSMTBitVectorTheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(bv))
			_, _ = bvFixedSymbolic(t, m, state, bv, "refused-bv-word", 2)
			// mk_bits leaves only literals in m_bits; the context owns the atoms.
			if len(bv.literalBits) == 0 {
				t.Fatal("internalized word has no literal Boolean representation")
			}
			wordOwners := len(bv.bitOwners)
			bit := smtRelevancyBool(t, m, "refused-bv-temporary")
			caller, err := m.NewRef(bit)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&caller)) })
			if operation != "close-owned" {
				output, err := m.NewRef(bit)
				astTestMust(t, err)
				_, err = bv.adoptBitBlasterRefs([]ast.ASTRef{output})
				astTestMust(t, err)
			}
			temporaries := append([]ast.ASTRef(nil), bv.temporary...)
			trigger()
			if !errors.Is(failure, ast.ErrASTPending) {
				t.Fatal("cleanup did not reach native AST refusal", failure)
			}
			if len(bv.temporary) != len(temporaries) || len(bv.bitOwners) != wordOwners {
				t.Fatal("refused cleanup discarded a live owner", len(bv.temporary), len(bv.bitOwners), wordOwners)
			}
			for i, owner := range temporaries {
				if bv.temporary[i] != owner {
					t.Fatal("refused cleanup replaced a temporary owner")
				}
			}
			astTestMust(t, bv.releaseTemporary())
			refs, err := m.GetRefCount(bit)
			astTestMust(t, err)
			if refs != 1 {
				t.Fatal("temporary cleanup changed caller ownership", refs)
			}
			astTestMust(t, bv.Close())
			astTestMust(t, bv.Close())
			if len(bv.temporary) != 0 || len(bv.bitOwners) != 0 || len(bv.bitTrail) != 0 || len(bv.literalBits) != 0 {
				t.Fatal("cleanup retry left bit-vector roots")
			}
		})
	}
}
