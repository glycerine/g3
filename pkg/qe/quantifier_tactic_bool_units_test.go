package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierBoolUnitsEliminateExistentialAndUniversal(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	v := f.own(m.MkVar(0, f.boolean))
	r := f.constant("quant-unit-free", f.boolean)
	conjunction := f.own(m.MkAnd(v, r))
	disjunction := f.own(m.MkOr(v, r))
	xor := f.own(m.MkXor(v, r))
	for _, tc := range []struct {
		name     string
		body     ast.ASTHandle
		forall   bool
		want     ast.ASTHandle
		complete bool
	}{
		{"exists", v, false, f.own(m.MkTrue()), true},
		{"forall", v, true, f.own(m.MkFalse()), true},
		{"free", conjunction, false, r, true},
		{"forall-free", disjunction, true, r, true},
		{"xor-branch", xor, false, f.own(m.MkTrue()), true},
		{"forall-xor-branch", xor, true, f.own(m.MkFalse()), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: tc.forall, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: tc.body}))
			result, complete, err := eliminateQuantifierBoolUnits(m, quantifier, f.r.config)
			fmMust(t, err)
			if complete != tc.complete {
				t.Fatal("unexpected completion status", complete)
			}
			if !complete {
				if result != (ast.ASTRef{}) {
					t.Fatal("unsupported case returned a result")
				}
				return
			}
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			if value != tc.want {
				t.Fatal("incorrect quantifier result")
			}
		})
	}
}

func TestQuantifierBoolIndependentPredicatePartitions(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.boolean))
	y := f.own(m.MkVar(1, f.boolean))
	left := f.predicate("qe-partition-left", x)
	right := f.predicate("qe-partition-right", y)
	body := f.own(m.MkAnd(left, right))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean, f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	result, complete, err := eliminateQuantifierBoolUnits(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("independent Boolean predicate partitions were not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	truth := f.own(m.MkTrue())
	falsity := f.own(m.MkFalse())
	leftTrue := f.predicate("qe-partition-left", truth)
	leftFalse := f.predicate("qe-partition-left", falsity)
	rightTrue := f.predicate("qe-partition-right", truth)
	rightFalse := f.predicate("qe-partition-right", falsity)
	leftEither := f.own(m.MkOr(leftTrue, leftFalse))
	rightEither := f.own(m.MkOr(rightTrue, rightFalse))
	expected := f.own(m.MkAnd(leftEither, rightEither))
	iff := f.own(m.MkIff(value, expected))
	different := f.own(m.MkNot(iff))
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(different))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("partitioned result differs from Boolean expansion")
	}
}
