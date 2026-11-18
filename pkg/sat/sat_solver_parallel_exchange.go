package sat

import "github.com/glycerine/g3/pkg/util/lbool"

func (s *satSolverConflictResolution) exchangeParallelBound() error {
	if s.parallel == nil {
		return ErrSATMissingParallelBinding
	}
	rootSize, err := s.solver.sourceInitTrailSize()
	if err != nil {
		return err
	}
	var out []Literal
	numIn := uint32(0)
	for i := s.parallelLimitOut; i < rootSize; i++ {
		literal, readErr := s.analysis.propagation.variables.trailRead(i)
		if readErr != nil {
			return readErr
		}
		if uint32(literal.variable()) < s.parallelNumVars {
			out = append(out, literal)
		}
	}
	s.parallelLimitOut = rootSize
	in, limit, err := s.parallel.Exchange(out, s.parallelLimitIn)
	if err != nil {
		return err
	}
	s.parallelLimitIn = limit
	numOut := uint32(len(out))
	p := s.analysis.propagation
	for _, literal := range in {
		if p.inconsistent {
			break
		}
		if uint32(literal.variable()) >= s.parallelNumVars {
			return ErrSATAssertion
		}
		level, readErr := p.variables.levelVar(literal.variable())
		if readErr != nil {
			return readErr
		}
		value, readErr := p.variables.valueLiteral(literal)
		if readErr != nil {
			return readErr
		}
		if level != 0 || value != lbool.True {
			numIn++
			if err = p.assign(literal, satJustification{}); err != nil {
				return err
			}
		}
	}
	if numIn > 0 || numOut > 0 {
		// IF_VERBOSE(1, "(sat-sync out: " << num_out << " in: " << num_in
		// << ")\n") (AUDIT.md sat_03-F3); g3trace builds only.
		return satSyncVerbose(numOut, numIn)
	}
	return nil
}
