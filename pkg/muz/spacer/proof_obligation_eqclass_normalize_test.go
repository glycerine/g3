package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestProofObligationEqualityClassNormalizationSourceOrder(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, err)
	defer func() { depthMust(t, owner.Close()) }()
	truth, err := m.MkTrue()
	depthMust(t, err)
	falsehood, err := m.MkFalse()
	depthMust(t, err)
	transformer, err := NewPredicateTransformer(owner, "eqclass-normalize", falsehood, truth, PropSolverConfig{})
	depthMust(t, err)
	defer func() { depthMust(t, transformer.Close()) }()
	depthMust(t, transformer.SetProofObligationNormalization(false, true))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	depthMust(t, err)
	integer, err := m.GetSort(zero)
	depthMust(t, err)
	fresh := func(name string) ast.ASTHandle {
		t.Helper()
		h, err := m.MkFreshConst(name, integer)
		depthMust(t, err)
		return h
	}
	x, y, z := fresh("eqclass-x"), fresh("eqclass-y"), fresh("eqclass-z")
	five, err := m.ArithmeticIntegerNumeralText("5")
	depthMust(t, err)
	eqXY, err := m.MkEq(x, y)
	depthMust(t, err)
	eqYZ, err := m.MkEq(y, z)
	depthMust(t, err)
	eqZ0, err := m.MkEq(z, zero)
	depthMust(t, err)
	bound, err := m.MkArithmeticLE(x, five)
	depthMust(t, err)
	original, err := m.MkAndArray([]ast.ASTHandle{eqXY, bound, eqZ0, eqYZ})
	depthMust(t, err)
	pin, err := m.NewRef(original)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&pin)) }()
	obligation, err := NewProofObligation(nil, transformer, 1, 0, original, nil)
	depthMust(t, err)
	defer func() { depthMust(t, obligation.Release()) }()
	normalized, err := obligation.Post()
	depthMust(t, err)
	count, err := m.GetNumArgs(normalized)
	depthMust(t, err)
	if count != 4 {
		t.Fatal("factored proof-obligation arity", count)
	}
	eqX0, err := m.MkEq(x, zero)
	depthMust(t, err)
	eqXZ, err := m.MkEq(x, z)
	depthMust(t, err)
	want := []ast.ASTHandle{bound, eqX0, eqXY, eqXZ}
	for i := uint32(0); i < count; i++ {
		arg, err := m.GetArg(normalized, i)
		depthMust(t, err)
		if arg != want[i] {
			t.Fatal("source equality-class order", i, arg, want[i])
		}
	}
	expected, err := m.MkAndArray(want)
	depthMust(t, err)
	if normalized != expected {
		t.Fatal("proof-obligation did not publish the exact factored conjunction", normalized, expected)
	}
	equivalent, err := m.MkEq(original, normalized)
	depthMust(t, err)
	notEquivalent, err := m.MkNot(equivalent)
	depthMust(t, err)
	solver, err := smtcore.NewSMTSolver(m, nil, "QF_LIA", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, err)
	defer func() { depthMust(t, solver.Close()) }()
	depthMust(t, solver.Assert(notEquivalent))
	status, err := solver.CheckSat(nil)
	depthMust(t, err)
	if status != lbool.False {
		t.Fatal("factored proof obligation is not equivalent", status)
	}
}
