package core

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPBPreprocessSubsumesWeakerConstraint(t *testing.T) {
	f := newPBPreprocessFixture(t)
	small := f.ge(f.vars[:2], []string{"1", "1"}, "1")
	large := f.ge(f.vars, []string{"1", "1", "1"}, "1")
	notX, e := f.m.MkNot(f.vars[0])
	simplifyMust(t, e)
	neg := f.ge([]ast.ASTHandle{notX}, []string{"1"}, "1")
	eq, e := f.m.MkEq(f.vars[1], f.vars[2])
	simplifyMust(t, e)
	g := f.goal(small, large, neg, eq)
	f.apply(g)
	n, e := g.Size()
	simplifyMust(t, e)
	if n >= 4 {
		t.Fatal("subsumed constraint survived", n)
	}
}

func TestPBPreprocessSubsumptionCoefficientDirection(t *testing.T) {
	f := newPBPreprocessFixture(t)
	a := &pbPreprocessConstraint{terms: []pbPreprocessTerm{{expression: f.vars[0], coefficient: big.NewRat(2, 1)}}, bound: big.NewRat(1, 1)}
	b := &pbPreprocessConstraint{terms: []pbPreprocessTerm{{expression: f.vars[0], coefficient: big.NewRat(1, 1)}}, bound: big.NewRat(1, 1)}
	if pbPreprocessSubsumes(a, b) {
		t.Fatal("larger source coefficient cannot subsume smaller target")
	}
}
