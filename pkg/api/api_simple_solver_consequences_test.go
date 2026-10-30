package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverConsequencesOwnResults(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", func(c *SMTContext) { astTestMust(t, c.SetBoolOption("model", true)) })
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-simple-consequence", boolean))
	astTestMust(t, s.Assert(p))
	status, consequences, err := s.Consequences(nil, []ast.ASTHandle{p})
	astTestMust(t, err)
	if status != lbool.True || len(consequences) != 1 {
		t.Fatal("consequences", status, len(consequences))
	}
	astTestMust(t, s.Close())
	value, err := f.m.RefGet(consequences[0])
	astTestMust(t, err)
	if sort, e := f.m.GetSort(value); e != nil || sort != boolean {
		t.Fatal("consequence sort", sort, e)
	}
	astTestMust(t, f.m.DelRef(&consequences[0]))
}
