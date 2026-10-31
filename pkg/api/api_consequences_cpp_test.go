package api

// Source: src/test/get_consequences.cpp.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestConsequencesSourceFiniteDomain(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	s, err := f.c.NewSolverForLogic("QF_FD")
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	e, err := f.c.EnumerationSort("RGB", []string{"R", "G", "B"})
	astTestMust(t, err)
	sort := f.pin(e.Sort, nil)
	for i := range e.Constants {
		e.Constants[i] = f.pin(e.Constants[i], nil)
		e.Recognizers[i] = f.pin(e.Recognizers[i], nil)
	}
	x := f.pin(f.c.Constant("x", sort))
	y := f.pin(f.c.Constant("y", sort))
	r := f.pin(f.c.Apply(e.Constants[0]))
	g := f.pin(f.c.Apply(e.Constants[1]))
	b := f.pin(f.c.Apply(e.Constants[2]))
	astTestMust(t, s.Assert(f.pin(f.c.Not(f.pin(f.c.Equal(x, r))))))
	astTestMust(t, s.Assert(f.pin(f.c.Not(f.pin(f.c.Equal(x, b))))))
	check := func() {
		t.Helper()
		status, values, err := s.Consequences(nil, []ast.ASTHandle{x, y})
		astTestMust(t, err)
		if status != lbool.True || len(values) == 0 {
			t.Fatal(status, len(values))
		}
		for i := range values {
			astTestMust(t, f.m.DelRef(&values[i]))
		}
	}
	check()
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(f.pin(f.c.Not(f.pin(f.c.Equal(x, g))))))
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	astTestMust(t, s.Pop(1))
	check()
	md, err := s.Model()
	astTestMust(t, err)
	astTestMust(t, md.Close())
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	md, err = s.Model()
	astTestMust(t, err)
	astTestMust(t, md.Close())
}

func TestConsequencesSourceBooleanAndBitVectors(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	s, err := f.c.NewSolverForLogic("QF_FD")
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	bs := f.pin(f.c.BoolSort())
	bv := f.pin(f.c.BitVectorSort(3))
	a := f.pin(f.c.Constant("a", bs))
	b := f.pin(f.c.Constant("b", bs))
	c := f.pin(f.c.Constant("c", bs))
	ba := f.pin(f.c.Constant("ba", bv))
	bb := f.pin(f.c.Constant("bb", bv))
	bc := f.pin(f.c.Constant("bc", bv))
	astTestMust(t, s.Assert(f.pin(f.c.Implies(a, b))))
	astTestMust(t, s.Assert(f.pin(f.c.Implies(b, c))))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(ba, bc))))
	two := f.pin(f.c.IntNumeral(2, bv))
	astTestMust(t, s.Assert(f.pin(f.c.Equal(two, ba))))
	status, values, err := s.Consequences([]ast.ASTHandle{a}, []ast.ASTHandle{b, c, bb, bc})
	astTestMust(t, err)
	if status != lbool.True || len(values) != 3 {
		t.Fatal(status, len(values))
	}
	for i := range values {
		astTestMust(t, f.m.DelRef(&values[i]))
	}
}
