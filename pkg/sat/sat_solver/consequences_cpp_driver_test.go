package sat_solver_test

// Source: src/test/get_consequences.cpp test1.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestIncrementalConsequencesSourceDriver(t *testing.T) {
	f, s := newSolverFixture(t)
	g := f.goal(t, `(declare-const a Bool)(declare-const b Bool)(declare-const c Bool)(declare-const ba (_ BitVec 3))(declare-const bb (_ BitVec 3))(declare-const bc (_ BitVec 3))
 (assert (=> a b))(assert (=> b c))(assert (= ba bc))(assert (= #b010 ba))
 (assert a)(assert b)(assert c)(assert (= bb bb))(assert (= bc bc))`)
	get := func(i uint32) ast.ASTHandle { h, err := g.Form(i); incMust(t, err); return h }
	for i := uint32(0); i < 4; i++ {
		incMust(t, s.Assert(get(i)))
	}
	a, b, c := get(4), get(5), get(6)
	bb, err := f.manager.GetArg(get(7), 0)
	incMust(t, err)
	bc, err := f.manager.GetArg(get(8), 0)
	incMust(t, err)
	status, values, err := s.GetConsequences([]ast.ASTHandle{a}, []ast.ASTHandle{b, c, bb, bc})
	incMust(t, err)
	if status != lbool.True || len(values) != 3 {
		t.Fatal(status, len(values))
	}
}
