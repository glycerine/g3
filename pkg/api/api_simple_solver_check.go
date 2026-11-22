package api

// Source: src/api/api_solver.cpp:376-391,443-449.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *APISimpleSolver) Check(assumptions ...ast.ASTHandle) (lbool.LBool, error) {
	if err := s.use(); err != nil {
		return lbool.Undef, err
	}
	pending := s.pendingRefutation
	if pending == nil {
		return s.solver.CheckSat(assumptions)
	}
	s.pendingRefutation = nil
	remember := len(assumptions) == 0 && pending.matchesCurrent(s.solver)
	answer, err := s.solver.CheckSat(assumptions)
	if remember && err == nil && answer == lbool.False {
		pending.remember()
	}
	return answer, err
}

func (s *APISimpleSolver) ReasonUnknown() (string, error) {
	if err := s.use(); err != nil {
		return "", err
	}
	return s.solver.ReasonUnknown()
}
