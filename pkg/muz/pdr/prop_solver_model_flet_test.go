package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// AUDIT.md muz_pdr_02-F6: check_safe_assumptions flets m_fparams.m_model to
// m_model != nullptr and restores it afterwards. mk_proto_model still runs
// when m_qmanager->model_based() (smt.mbqi, on by default) holds, so a
// satisfiable check builds its model either way (AUDIT.md FUZZ-F5: the
// proto model's values are live AST nodes that shift later IDs).
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
		if md == nil {
			t.Fatal("smt.mbqi check built no model", wantModel)
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
