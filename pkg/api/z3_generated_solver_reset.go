package api

// Source: src/api/api_solver.cpp, persistent solver parameters and reset.

import "errors"

func (s *replaySolver) setParameters(p *ParameterSet) (err error) {
	next, err := s.owner.context.NewParameterSet()
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, next.Close())
		}
	}()
	if s.params != nil {
		if err = next.params.Append(&next.value, &s.params.value); err != nil {
			return err
		}
	}
	if err = next.params.Append(&next.value, &p.value); err != nil {
		return err
	}
	if err = s.value.SetParameters(p); err != nil {
		return err
	}
	old := s.params
	s.params = next
	published = true
	if old != nil {
		if err = old.Close(); err != nil {
			s.retiredParams = append(s.retiredParams, old)
			return err
		}
	}
	return nil
}

// replaySolverReset is Z3_solver_reset: m_solver = nullptr; the handle
// keeps its factory and parameters and init_solver rebuilds on next use
// (a translated handle has none: FORK FIX #122).
func replaySolverReset(r *Z3Replayer) error {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	return s.value.Reset()
}
