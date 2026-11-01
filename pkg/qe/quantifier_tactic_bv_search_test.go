package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierBitVectorSearch(t *testing.T) {
	for _, forall := range []bool{false, true} {
		t.Run(map[bool]string{false: "exists", true: "forall"}[forall], func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			sort := f.own(m.BitVectorSort(2))
			x := f.own(m.MkVar(0, sort))
			y := f.constant("bv-search-free", sort)
			one := f.own(m.BitVectorNumeralUint64(1, 2))
			sum := f.own(m.MkBitVectorAdd(x, y))
			body := f.own(m.MkEq(sum, one))
			quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
			result, complete, err := eliminateQuantifierBitVectorSearch(m, quantifier, f.r.config)
			fmMust(t, err)
			if !complete {
				t.Fatal("bit-vector quantifier was not eliminated")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			if !forall {
				value = f.own(m.MkNot(value))
			}
			k := qsatKernelOwner(t, f)
			solver, err := k.solver()
			fmMust(t, err)
			fmMust(t, solver.AssertFormula(value))
			status, err := solver.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("bit-vector quantifier result differs from the expected truth value")
			}
		})
	}
}

func TestQuantifierBitVectorBlockSearch(t *testing.T) {
	for _, forall := range []bool{false, true} {
		t.Run(map[bool]string{false: "exists", true: "forall"}[forall], func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			sort := f.own(m.BitVectorSort(2))
			x, y := f.own(m.MkVar(1, sort)), f.own(m.MkVar(0, sort))
			z := f.constant("bv-block-free", sort)
			body := f.own(m.MkEq(f.own(m.MkBitVectorAdd(x, y)), z))
			quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: []ast.ASTHandle{sort, sort}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
			result, complete, err := eliminateQuantifierBitVectorBlock(m, quantifier, f.r.config)
			fmMust(t, err)
			if !complete {
				t.Fatal("bit-vector block was not eliminated")
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			if !forall {
				value = f.own(m.MkNot(value))
			}
			kernel := qsatKernelOwner(t, f)
			solver, err := kernel.solver()
			fmMust(t, err)
			fmMust(t, solver.AssertFormula(value))
			status, err := solver.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("bit-vector block result differs from expected", status)
			}
		})
	}
}
