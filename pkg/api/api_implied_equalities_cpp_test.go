package api

// Source: src/test/get_implied_equalities.cpp.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestImpliedEqualitiesSourceArithmeticAndArrays(t *testing.T) {
	t.Run("arithmetic and functions", func(t *testing.T) {
		f := newSMTAPIFixture(t, "ALL", nil)
		solver, err := f.c.NewSimpleSolver()
		astTestMust(t, err)
		defer func() { astTestMust(t, solver.Close()) }()
		intSort := f.pin(f.c.IntegerSort())
		a := f.pin(f.c.Constant("a", intSort))
		b := f.pin(f.c.Constant("b", intSort))
		c := f.pin(f.c.Constant("c", intSort))
		d := f.pin(f.c.Constant("d", intSort))
		decl := f.pin(f.c.Function("f", []ast.ASTHandle{intSort}, intSort))
		fa := f.pin(f.c.Apply(decl, a))
		fb := f.pin(f.c.Apply(decl, b))
		fc := f.pin(f.c.Apply(decl, c))
		terms := []ast.ASTHandle{a, b, c, d, fa, fb, fc}
		astTestMust(t, solver.Assert(f.pin(f.c.Equal(a, b))))
		astTestMust(t, solver.Assert(f.pin(f.c.Equal(b, d))))
		astTestMust(t, solver.Assert(f.pin(f.c.ArithmeticLE(fa, fc))))
		astTestMust(t, solver.Assert(f.pin(f.c.ArithmeticLE(fc, d))))
		check := func(merged bool) {
			t.Helper()
			status, classes, err := solver.ImpliedEqualities(terms)
			astTestMust(t, err)
			if status != lbool.True || len(classes) != 7 {
				t.Fatalf("%v %v", status, classes)
			}
			if classes[1] != classes[0] || classes[2] == classes[0] || classes[3] != classes[0] || classes[4] != classes[5] {
				t.Fatal(classes)
			}
			for _, i := range []int{4, 5, 6} {
				if (classes[i] == classes[0]) != merged {
					t.Fatal(classes)
				}
			}
		}
		check(false)
		astTestMust(t, solver.Assert(f.pin(f.c.ArithmeticLE(b, fa))))
		check(true)
	})
	t.Run("array stores", func(t *testing.T) {
		f := newSMTAPIFixture(t, "ALL", nil)
		solver, err := f.c.NewSimpleSolver()
		astTestMust(t, err)
		defer func() { astTestMust(t, solver.Close()) }()
		intSort := f.pin(f.c.IntegerSort())
		a := f.pin(f.c.Constant("a", intSort))
		b := f.pin(f.c.Constant("b", intSort))
		one := f.pin(f.c.IntNumeral(1, intSort))
		two := f.pin(f.c.IntNumeral(2, intSort))
		arraySort := f.pin(f.c.ArraySort([]ast.ASTHandle{intSort}, intSort))
		x := f.pin(f.c.ConstantArray(arraySort, one))
		y := f.pin(f.c.ArrayStore(x, []ast.ASTHandle{one}, a))
		z := f.pin(f.c.ArrayStore(y, []ast.ASTHandle{two}, b))
		u := f.pin(f.c.ArrayStore(x, []ast.ASTHandle{two}, b))
		v := f.pin(f.c.ArrayStore(u, []ast.ASTHandle{one}, a))
		status, classes, err := solver.ImpliedEqualities([]ast.ASTHandle{x, y, z, u, v})
		astTestMust(t, err)
		if status != lbool.True || len(classes) != 5 {
			t.Fatal(status, classes)
		}
		if classes[2] != classes[4] || classes[0] == classes[1] || classes[0] == classes[2] || classes[0] == classes[3] || classes[1] == classes[2] || classes[1] == classes[3] || classes[2] == classes[3] {
			t.Fatal(classes)
		}
	})
}
