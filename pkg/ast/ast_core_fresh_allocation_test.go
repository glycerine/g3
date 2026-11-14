package ast

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestASTFreshConstApplicationFailureReleasesTemporaryDeclaration(t *testing.T) {
	for _, nullPrefix := range []bool{false, true} {
		t.Run(fmt.Sprintf("null-prefix=%t", nullPrefix), func(t *testing.T) {
			completed, publishedFailure := false, false
			for allowance := int64(0); allowance < 64 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					m := astTestManager(t)
					sort, err := m.MkBoolSort()
					astTestMust(t, err)
					astTestAlignFreshAppToNodePage(t, m, sort, true)
					prefix := "fresh-allocation"
					pointer := &prefix
					if nullPrefix {
						pointer = nil
					}
					counts := func() [4]uint32 {
						s := m.Stats()
						return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
					}
					before, initialID := counts(), m.freshID
					m.Budget().FailAfter(allowance)
					result, cause := m.MkFreshConstNullable(pointer, sort)
					m.Budget().FailAfter(-1)
					if cause != nil {
						if !errors.Is(cause, arena.ErrInjectedFailure) || !result.IsNull() {
							t.Fatal("unexpected fresh application failure", result, cause)
						}
						if m.freshID == initialID+1 {
							publishedFailure = true
						} else if m.freshID != initialID {
							t.Fatal("failure changed source fresh-ID order", initialID, m.freshID)
						}
						if after := counts(); after != before {
							t.Fatal("failed application left its declaration unowned", before, after)
						}
						initialID = m.freshID
						result, err = m.MkFreshConstNullable(pointer, sort)
						astTestMust(t, err)
					} else {
						completed = true
					}
					if m.freshID != initialID+1 {
						t.Fatal("successful fresh application changed counter order", initialID, m.freshID)
					}
					if refs, err := m.GetRefCount(result); err != nil || refs != 0 {
						t.Fatal("fresh application changed raw result contract", refs, err)
					}
					actualSort, err := m.GetSort(result)
					astTestMust(t, err)
					if actualSort != sort {
						t.Fatal("fresh application changed its range sort")
					}
					root, err := m.NewRef(result)
					astTestMust(t, err)
					astTestMust(t, m.DelRef(&root))
					if after := counts(); after != before {
						t.Fatal("fresh application retry left generated owners", before, after)
					}
				}) {
					return
				}
			}
			if !completed || !publishedFailure {
				t.Fatal("campaign missed declaration publication before app failure or completion", publishedFailure, completed)
			}
		})
	}
}

// astTestAlignFreshAppToNodePage warms m until the next MkFreshConst puts
// its declaration in the last slot of a node page, so its application needs
// a fresh node page (application records live in node slots). Fresh
// constants take two slots; a variable takes one, to fix the parity.
func astTestAlignFreshAppToNodePage(t *testing.T, m *ASTManager, sort ASTHandle, pin bool) {
	t.Helper()
	for index := uint32(0); m.freeNode != 0 || m.nextNode%astPageSize != 0; index++ {
		var warm ASTHandle
		var err error
		if (astPageSize-m.nextNode%astPageSize)%2 == 1 && m.freeNode == 0 {
			warm, err = m.MkVar(index, sort)
		} else {
			warm, err = m.MkFreshConst("fresh-allocation-warm", sort)
		}
		astTestMust(t, err)
		if pin {
			astTestPin(t, m, warm)
		}
	}
}
