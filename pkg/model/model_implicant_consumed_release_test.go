package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The source deleted-ID guard forces an actual AST release to consume its
// reference and then report an error. This tests Go owner retry bookkeeping;
// it does not claim a naturally reached C++ deletion failure.
func TestModelImplicantConsumedReferenceReleaseRetry(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("requires source deleted-ID assertion")
	}
	for _, closing := range []bool{false, true} {
		for _, failedIndex := range []int{0, 1} {
			t.Run(fmt.Sprintf("close=%t/index=%d", closing, failedIndex), func(t *testing.T) {
				m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
				arrayTableMust(t, m.EnableDebugRefCount())
				boolean, err := m.MkBoolSort()
				arrayTableMust(t, err)
				before := m.Stats()
				arrayTableMust(t, implicant.setupModel(md))
				nodes := make([]ast.ASTHandle, 2)
				for n := range nodes {
					nodes[n], err = m.MkVar(uint32(1000+n), boolean)
					arrayTableMust(t, err)
					arrayTableMust(t, implicant.pin(nodes[n]))
				}
				id, err := m.GetASTID(nodes[failedIndex])
				arrayTableMust(t, err)
				m.DebugFreeIndices[id] = struct{}{}
				defer delete(m.DebugFreeIndices, id)
				release := implicant.reset
				if closing {
					release = implicant.Close
				}
				err = release()
				if !errors.Is(err, ast.ErrASTDomain) {
					t.Fatal("release did not stop at the deleted-ID guard", err)
				}
				if implicant.closed || implicant.evaluator == nil || implicant.model != md {
					t.Fatal("stopped cleanup discarded pending owner state")
				}
				for n := 0; n <= failedIndex; n++ {
					if implicant.refs[n] != (ast.ASTRef{}) {
						t.Fatal("consumed reference still published", n)
					}
				}
				if failedIndex == 0 {
					count, err := m.GetRefCount(nodes[1])
					arrayTableMust(t, err)
					if count != 1 {
						t.Fatal("stopped cleanup consumed its unreached tail", count)
					}
				}
				delete(m.DebugFreeIndices, id)
				arrayTableMust(t, release())
				if implicant.evaluator != nil || implicant.model != nil || len(implicant.refs) != 0 || implicant.drain != 0 || implicant.closed != closing {
					t.Fatal("cleanup retry did not complete its owner state")
				}
				arrayTableMust(t, release())
				if !closing {
					yes, err := m.MkTrue()
					arrayTableMust(t, err)
					value, err := implicant.EvalExpr(md, yes)
					arrayTableMust(t, err)
					actual, err := m.RefGet(value)
					arrayTableMust(t, err)
					if actual != yes {
						t.Fatal("reused implicant returned the wrong value")
					}
					arrayTableMust(t, m.DelRef(&value))
				}
				// The artificial guard stops AST deletion before native storage is
				// freed. That zero-ref node belongs to manager teardown, not the
				// implicant retry; the fixture checks its final budget release.
				count, err := m.GetRefCount(nodes[failedIndex])
				arrayTableMust(t, err)
				after := m.Stats()
				if count != 0 || after.LiveNodes != before.LiveNodes+1 || after.LiveReferences != before.LiveReferences {
					t.Fatal("cleanup retry lost consumed-reference bookkeeping", count, after, before)
				}
			})
		}
	}
}
