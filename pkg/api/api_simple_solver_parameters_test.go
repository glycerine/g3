package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverParameterDescriptionsAndApplication(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	descriptions, err := s.ParameterDescriptions()
	astTestMust(t, err)
	for _, name := range []string{"proof", "model", "unsat_core", "random_seed"} {
		if kind, e := descriptions.Kind(name); e != nil || (name != "random_seed" && kind != ParameterBool) {
			t.Fatal("parameter description", name, kind, e)
		}
	}
	astTestMust(t, descriptions.Close())

	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetUInt("relevancy", 1))
	astTestMust(t, s.SetParameters(parameters))
	overlay, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, overlay.SetBool("model", false))
	astTestMust(t, overlay.SetUInt("random_seed", 17))
	astTestMust(t, s.SetParameters(overlay))
	if s.solver != nil {
		t.Fatal("Z3_solver_set_params built the solver")
	}
	// Z3_solver_get_num_scopes runs init_solver, which applies m_params.
	_, err = s.ScopeLevel()
	astTestMust(t, err)
	current, err := s.solver.Params()
	astTestMust(t, err)
	name := "random_seed"
	if seed, e := overlay.params.GetUIntName(current, &name, 0); e != nil || seed != 17 {
		t.Fatal("merged parameter", seed, e)
	}
	name = "relevancy"
	if relevancy, e := overlay.params.GetUIntName(current, &name, 0); e != nil || relevancy != 1 {
		t.Fatal("preserved parameter", relevancy, e)
	}
	name = "relevancy"
	if relevancy, e := overlay.params.GetUIntName(&s.explicitParams, &name, 0); e != nil || relevancy != 1 {
		t.Fatal("explicit parameter overlay", relevancy, e)
	}
	if status, e := s.Check(); e != nil || status != lbool.True {
		t.Fatal("check", status, e)
	}
	// smt_solver ignores set_produce_models: the C++ simple solver still
	// answers Z3_solver_get_model after model=false (AUDIT.md
	// smtcore_03-F10 follow-up; checked against build/libz3.a).
	md, err := s.Model()
	astTestMust(t, err)
	astTestMust(t, md.Close())
	astTestMust(t, overlay.Close())
	astTestMust(t, parameters.Close())

	invalid, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, invalid.SetUInt("not_a_solver_parameter", 1))
	if err = s.SetParameters(invalid); err == nil {
		t.Fatal("unknown parameter", err)
	}
	astTestMust(t, invalid.Close())
	astTestMust(t, s.Close())
}
