package clp

// Source: src/muz/clp/clp_context.cpp clp::imp members m_ground and m_goals.
// Both live with the engine: m_ground is reset at every rule attempt and
// m_goals at every query, so their terms survive until then.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// State is the persistent part of clp::imp shared by its queries.
type State struct {
	manager *ast.ASTManager
	ground  *groundEnvironment
	goals   []ast.ASTRef
	closed  bool
}

func NewState(m *ast.ASTManager) *State {
	return &State{manager: m, ground: &groundEnvironment{manager: m}}
}

// releaseForward is ref_vector reset/resize: dec_ref front to back.
func releaseForward(m *ast.ASTManager, refs []ast.ASTRef) error {
	for i := range refs {
		if refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := m.DelRef(&refs[i]); err != nil {
			return err
		}
	}
	return nil
}

// resetGround is reset_ground(): m_ground.reset().
func (s *State) resetGround() error {
	g := s.ground
	if g.substitution != nil || len(g.temporaries) != 0 {
		if err := g.close(); err != nil {
			return err
		}
	}
	if err := releaseForward(s.manager, g.refs); err != nil {
		return err
	}
	g.values, g.refs = nil, nil
	return nil
}

// ResizeGoals is m_goals.resize(n) for n no larger than the current size.
func (s *State) resizeGoals(n int) error {
	if n >= len(s.goals) {
		return nil
	}
	if err := releaseForward(s.manager, s.goals[n:]); err != nil {
		return err
	}
	s.goals = s.goals[:n]
	return nil
}

// ResetGoals is clp::imp::query's m_goals.reset().
func (s *State) ResetGoals() error {
	if s == nil || s.closed {
		return nil
	}
	return s.resizeGoals(0)
}

func (s *State) goal(index int) (ast.ASTHandle, error) {
	if index < 0 || index >= len(s.goals) {
		return ast.ASTHandle{}, ErrCLP
	}
	return s.manager.RefGet(s.goals[index])
}

// Close is the imp destructor for these members.
func (s *State) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if err := s.resizeGoals(0); err != nil {
		return err
	}
	if err := s.ground.close(); err != nil {
		return err
	}
	s.closed = true
	return nil
}
