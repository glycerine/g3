package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIOptimizeBoundVectorsReturnInfinityRationalEpsilon(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Assert(f.upper))
	index, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	_, err = f.opt.Check()
	astTestMust(t, err)
	for _, get := range []func(uint32) ([]ast.ASTRef, error){f.opt.LowerVector, f.opt.UpperVector} {
		values, getErr := get(index)
		astTestMust(t, getErr)
		if len(values) != 3 {
			t.Fatal("vector size", len(values))
		}
		want := []string{"0", "5", "0"}
		for i := range values {
			if text := optimizeBoundNumeral(t, f, &values[i]); text != want[i] {
				t.Fatal("component", i, text)
			}
		}
		for i := range values {
			astTestMust(t, f.m.DelRef(&values[i]))
		}
	}
}

func TestAPIOptimizeUnboundedUpperVector(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	index, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	_, err = f.opt.Check()
	astTestMust(t, err)
	values, err := f.opt.UpperVector(index)
	astTestMust(t, err)
	want := []string{"1", "0", "0"}
	for i := range values {
		if text := optimizeBoundNumeral(t, f, &values[i]); text != want[i] {
			t.Fatal("unbounded component", i, text)
		}
		astTestMust(t, f.m.DelRef(&values[i]))
	}
}
