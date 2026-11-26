package sat

import (
	"io"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// ProbeDIMACSUniformModel tries the two uniform assignments against all
// clauses of a fresh formula. Undef leaves the verdict to normal ingestion
// and solving. The caller supplies an independent, bounded reader.
func (s *Solver) ProbeDIMACSUniformModel(input io.Reader) (uint32, lbool.LBool) {
	if s.open() != nil || input == nil || s.Variables.NumVars() != 0 || s.Variables.scopeLevel != 0 || s.propagation.inconsistent ||
		s.Limit.Closed() || s.Limit.Limit != 0 || s.Limit.CancelFlag() || s.propagation.extensionPresent {
		return 0, lbool.Undef
	}
	cfg, err := s.config.State()
	if err != nil || cfg.DimacsDisplay || cfg.MaxConflicts != ^uint32(0) || cfg.NumParallel > 1 || cfg.MaxMemory != uint64(^uint(0)) {
		return 0, lbool.Undef
	}
	verbose, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || verbose || s.budget.InjectionArmed() || s.budget.Limit()-s.budget.Used() < 1<<30 {
		return 0, lbool.Undef
	}
	return scanDIMACSUniformModel(input, s.Limit)
}

func scanDIMACSUniformModel(input io.Reader, limit *control.Reslimit) (uint32, lbool.LBool) {
	buffer := newDIMACSStreamBuffer(input)
	defer buffer.syncShared()
	const positive, negative = uint8(1), uint8(2)
	possible := positive | negative
	var maxVar, clauses uint32
	for buffer.nextClause() {
		clauses++
		if clauses%128 == 0 && limit.CancelFlag() {
			return 0, lbool.Undef
		}
		var signs uint8
		for {
			lit, err := buffer.parseInt()
			if err != nil {
				return 0, lbool.Undef
			}
			if lit == 0 {
				break
			}
			// Bound the model's size before taking the magnitude.
			if lit < -100000 || lit > 100000 {
				return 0, lbool.Undef
			}
			if lit > 0 {
				signs |= positive
			} else {
				signs |= negative
				lit = -lit
			}
			maxVar = max(maxVar, uint32(lit))
		}
		possible &= signs
		if possible == 0 {
			return 0, lbool.Undef
		}
	}
	if clauses == 0 || limit.CancelFlag() {
		return 0, lbool.Undef
	}
	return maxVar, lbool.FromBool(possible&positive != 0)
}
