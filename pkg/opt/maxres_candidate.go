package opt

// Source: src/opt/maxres.cpp update_assignment upper-bound/model retention.

import "errors"
import "fmt"

func (e *maxResEngine) updateCandidate(result *ArithmeticResult) error {
	md, err := e.solver.backend.AcquireModel()
	if err != nil {
		return fmt.Errorf("maxres candidate model: %w", err)
	}
	if md == nil {
		return errMaxResState
	}
	cost, satisfied, assignment, err := e.solver.evaluateSoftDetailed(e.objective, md)
	if err != nil {
		return errors.Join(fmt.Errorf("maxres candidate objective: %w", err), md.DecRef())
	}
	e.stats.Candidates++
	correction, err := e.correctionSet(md)
	if err != nil {
		return errors.Join(fmt.Errorf("maxres candidate correction: %w", err), md.DecRef())
	}
	if e.bestSet && cost.Cmp(&e.upper) >= 0 {
		return md.DecRef()
	}
	if result.Model != nil {
		if err = result.Model.DecRef(); err != nil {
			return errors.Join(err, md.DecRef())
		}
	}
	result.Model = md
	if err = e.solver.captureResultLabels(result); err != nil {
		return err
	}
	setSoftObjectiveValue(result, e.index, cost, satisfied, assignment)
	e.upper.Set(&cost)
	e.bestSet = true
	e.correction = append(e.correction[:0], correction...)
	e.stats.Improvements++
	return e.assertUpperBound(&satisfied, len(correction))
}
