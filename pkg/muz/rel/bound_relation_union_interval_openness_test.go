package rel

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBoundRelationUnionIntervalIgnoresEndpointOpenness(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, cause error) ast.ASTHandle {
		astTestMust(t, cause)
		astTestPin(t, m, h)
		return h
	}
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	one := own(m.ArithmeticRealNumeralText("1"))
	for _, endpoint := range []struct {
		name  string
		left  ast.ASTHandle
		right ast.ASTHandle
	}{
		{"closed", own(m.MkArithmeticLE(x, one)), own(m.MkArithmeticGE(y, one))},
		{"open", own(m.MkArithmeticLT(x, one)), own(m.MkArithmeticGT(y, one))},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			interval := NewIntervalFull(2)
			astTestMust(t, interval.FilterInterpreted(m, endpoint.left))
			astTestMust(t, interval.FilterInterpreted(m, endpoint.right))
			for _, strict := range []bool{true, false} {
				bound := NewBoundRelation(2, false)
				if strict {
					astTestMust(t, bound.AddLess(0, 1))
				} else {
					astTestMust(t, bound.AddLessEqual(0, 1))
				}
				astTestMust(t, bound.UnionInterval(interval))
				lt, le, cause := bound.Bounds(0)
				astTestMust(t, cause)
				if strict && len(lt) != 0 || !strict && !reflect.DeepEqual(le, []uint32{1}) {
					t.Fatalf("source endpoint-value comparison changed strict=%t: lt=%v le=%v", strict, lt, le)
				}
			}
		})
	}
}
