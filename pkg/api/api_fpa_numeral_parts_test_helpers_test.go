package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type fpaNumeralPartsFixture struct {
	context        *SMTContext
	manager        *ast.ASTManager
	single, double ast.ASTHandle
}

func newFPANumeralPartsFixture(t *testing.T) *fpaNumeralPartsFixture {
	t.Helper()
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	f := &fpaNumeralPartsFixture{context: context, manager: manager}
	value, err := context.FloatingPointSingleSort()
	f.single = f.pin(t, value, err)
	value, err = context.FloatingPointDoubleSort()
	f.double = f.pin(t, value, err)
	return f
}

func (f *fpaNumeralPartsFixture) pin(t *testing.T, value ast.ASTHandle, err error) ast.ASTHandle {
	t.Helper()
	astTestMust(t, err)
	ref, err := f.manager.NewRef(value)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.manager.DelRef(&ref)) })
	return value
}

func (f *fpaNumeralPartsFixture) uint(t *testing.T, value ast.ASTHandle) uint64 {
	t.Helper()
	n, err := f.context.bitVectorNumeralUint64(value)
	astTestMust(t, err)
	return n
}
