package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// AUDIT.md muz_pdr_02-F6: check_safe_assumptions flets m_fparams.m_model to
// m_model != nullptr, so a satisfiable check that wants no model builds none,
// and the previous setting is restored afterwards.
func TestPropSolverModelGenerationFollowsRequest(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer closeBase()
	defer func() { closureMust(t, owner.Close()) }()
	defer func() { closureMust(t, transformer.Close()) }()
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	atom, err := m.MkFreshConst("model_flet_atom", boolean)
	closureMust(t, err)
	solver := transformer.solver
	before, produceBefore, err := solver.context.solver.ModelGeneration()
	closureMust(t, err)
	for _, wantModel := range []bool{false, true} {
		result, err := solver.CheckAssumptions([]ast.ASTHandle{atom}, false, wantModel)
		closureMust(t, err)
		if result.Status != lbool.True {
			t.Fatal("satisfiable check", result.Status)
		}
		md, err := solver.context.solver.AcquireModel()
		closureMust(t, err)
		if (md != nil) != wantModel {
			t.Fatal("model built against the request", wantModel, md != nil)
		}
		if md != nil {
			closureMust(t, md.DecRef())
		}
		if result.Model != nil {
			closureMust(t, result.Model.DecRef())
		}
		after, produceAfter, err := solver.context.solver.ModelGeneration()
		closureMust(t, err)
		if after != before || produceAfter != produceBefore {
			t.Fatal("model generation not restored")
		}
	}
}
