package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The deleted-ID guard forces a consumed DEC error in a real extracted row.
// This checks Go lexical-owner recovery, not a naturally reached C++ failure.
func TestModelImplicantOwnsInterruptedArrayFrameCleanup(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("requires source deleted-ID assertion")
	}
	for _, closing := range []bool{false, true} {
		for _, failedRow := range []int{0, 1} {
			t.Run(fmt.Sprintf("close=%t/row=%d", closing, failedRow), func(t *testing.T) {
				m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
				arrayTableMust(t, m.EnableDebugRefCount())
				zero, err := m.ArithmeticIntegerNumeralText("0")
				arrayTableMust(t, err)
				integer, err := m.GetSort(zero)
				arrayTableMust(t, err)
				boolean, err := m.MkBoolSort()
				arrayTableMust(t, err)
				sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), boolean)
				arrayTableMust(t, err)
				sortRef, err := m.NewRef(sort)
				arrayTableMust(t, err)
				defer func() { arrayTableMust(t, m.DelRef(&sortRef)) }()
				no, err := m.MkFalse()
				arrayTableMust(t, err)
				yes, err := m.MkTrue()
				arrayTableMust(t, err)
				before := m.Stats()
				arrayTableMust(t, implicant.setupModel(md))
				base, err := m.MkConstArray(sort, no)
				arrayTableMust(t, err)
				input, err := m.NewRef(base)
				arrayTableMust(t, err)
				for n := 0; n < 2; n++ {
					index, err := m.MkVar(uint32(2048+n), integer)
					arrayTableMust(t, err)
					current, err := m.RefGet(input)
					arrayTableMust(t, err)
					family, err := m.ArrayFamilyID()
					arrayTableMust(t, err)
					stored, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{current, index, yes})
					arrayTableMust(t, err)
					arrayTableMust(t, m.SetRef(input, stored))
				}
				root, err := m.RefGet(input)
				arrayTableMust(t, err)
				frame, ok, err := implicant.extractArray(root)
				arrayTableMust(t, err)
				if !ok || len(frame.stores) != 2 {
					t.Fatal("failed to extract actual array rows")
				}
				defer func() { arrayTableMust(t, frame.close(m)) }()
				// Transfer the row lifetime entirely to the extraction owner before
				// forcing its release stop; no synthetic row or private state is installed.
				arrayTableMust(t, m.DelRef(&input))
				failed := frame.stores[failedRow][0]
				id, err := m.GetASTID(failed)
				arrayTableMust(t, err)
				m.DebugFreeIndices[id] = struct{}{}
				defer delete(m.DebugFreeIndices, id)
				err = implicant.closeArrayFrame(frame)
				if !errors.Is(err, ast.ErrASTDomain) || !frame.active || frame.drain != failedRow {
					t.Fatal("row release did not preserve its stop", err, frame.active, frame.drain)
				}
				if len(frame.stores[failedRow]) != 1 || frame.stores[failedRow][0] != yes {
					t.Fatal("consumed row index was not retired")
				}
				if failedRow == 0 {
					count, err := m.GetRefCount(frame.stores[1][0])
					arrayTableMust(t, err)
					if count != 1 {
						t.Fatal("unreached row was released", count)
					}
				}
				delete(m.DebugFreeIndices, id)
				if closing {
					arrayTableMust(t, implicant.Close())
				} else {
					arrayTableMust(t, implicant.reset())
				}
				if frame.active || len(frame.stores) != 0 || frame.elseCase != (ast.ASTRef{}) {
					t.Fatal("implicant lost interrupted lexical frame ownership")
				}
				count, err := m.GetRefCount(failed)
				arrayTableMust(t, err)
				after := m.Stats()
				// The guard leaves one zero-ref native node for manager teardown.
				if count != 0 || after.LiveNodes != before.LiveNodes+1 || after.LiveReferences != before.LiveReferences {
					t.Fatal("array cleanup retry lost ownership bookkeeping", count, after, before)
				}
				if implicant.model != nil || implicant.evaluator != nil || len(implicant.pendingArrays) != 0 || implicant.closed != closing {
					t.Fatal("array cleanup did not finish implicant teardown")
				}
			})
		}
	}
}
