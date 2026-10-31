package api

import (
	"math/big"
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

func (c *SMTContext) bitVectorNumeralUint64(value ast.ASTHandle) (uint64, error) {
	text, err := c.NumeralString(value)
	if err != nil {
		return 0, err
	}
	n, ok := new(big.Int).SetString(text, 10)
	if !ok || n.Sign() < 0 || !n.IsUint64() {
		return 0, ast.ErrASTDomain
	}
	return n.Uint64(), nil
}
