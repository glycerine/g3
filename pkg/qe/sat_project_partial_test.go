package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func TestSATPartialProjectionBooleanAndBitVector(t *testing.T) {
	for _, bitvector := range []bool{false, true} {
		t.Run(map[bool]string{false: "boolean", true: "bitvector"}[bitvector], func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			k := qsatKernelOwner(t, f)
			solver, err := k.solver()
			fmMust(t, err)
			c, err := newSATSolverContext(m, solver, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, c.Close()) }()
			rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
			fmMust(t, err)
			defer func() { fmMust(t, rw.Close()) }()
			md, err := model.NewModel(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			var variable, value, formula ast.ASTHandle
			if bitvector {
				sort, e := m.BitVectorSort(4)
				fmMust(t, e)
				variable = f.constant("partial-bv", sort)
				value = f.own(m.BitVectorNumeralUint64(3, 4))
				formula = f.own(m.MkEq(variable, value))
			} else {
				variable = f.constant("partial-bool", f.boolean)
				value = f.own(m.MkTrue())
				formula = variable
			}
			decl, err := m.GetDecl(variable)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, value))
			fmMust(t, c.addVariable(variable))
			fmMust(t, c.initializeFormula(formula))
			fmMust(t, c.projectPartial(0, md, rw, f.r.config))
			if len(c.variables) != 0 {
				t.Fatal("projected variable remains")
			}
			result, err := m.RefGet(c.formula)
			fmMust(t, err)
			if !m.IsTrue(result) {
				t.Fatal("model projection did not simplify to true")
			}
			if err = c.projectPartial(0, md, rw, f.r.config); err == nil {
				t.Fatal("invalid variable index accepted")
			}
		})
	}
}

func TestSATPartialProjectionArithmetic(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "5")
	m := f.r.manager
	k := qsatKernelOwner(t, f.fmFixture)
	solver, err := k.solver()
	fmMust(t, err)
	c, err := newSATSolverContext(m, solver, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, c.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	zero := f.number("0")
	minus := f.number("-1")
	lower := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticMul(minus, f.x)), zero))
	upper := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(f.x, f.number("-5"))), zero))
	formula := f.own(m.MkAnd(lower, upper))
	fmMust(t, c.addVariable(f.x))
	equality := f.own(m.MkEq(f.x, f.number("5")))
	fmMust(t, c.initializeFormula(equality))
	if err = c.projectPartial(0, f.model, rw, f.r.config); err == nil {
		t.Fatal("unsupported equality should fail arithmetic bound preparation")
	}
	if len(c.variables) != 1 {
		t.Fatal("failed projection removed variable")
	}
	fmMust(t, c.initializeFormula(formula))
	fmMust(t, c.projectPartial(0, f.model, rw, f.r.config))
	if len(c.variables) != 0 {
		t.Fatal("projected variable remains")
	}
	result, err := m.RefGet(c.formula)
	fmMust(t, err)
	contains, err := f.contains.Check(result)
	fmMust(t, err)
	if contains {
		t.Fatal("arithmetic variable survived projection")
	}
}
