package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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
	if status, e := s.Check(); e != nil || status != lbool.True {
		t.Fatal("check", status, e)
	}
	if _, err = s.Model(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("disabled model", err)
	}
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
