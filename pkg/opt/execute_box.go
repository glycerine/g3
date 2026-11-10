package opt

// Source: src/opt/opt_context.cpp execute_box.

import (
	"errors"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// ExecuteBox is execute_box's first call: m_optsmt.box() optimizes the
// arithmetic objectives independently, then each MaxSMT objective runs
// under a scoped push. Every objective contributes its model to
// m_box_models; m_model becomes the first.
func (s *ArithmeticSolver) ExecuteBox() (result *ArithmeticResult, resultErr error) {
	if s == nil || s.backend == nil || len(s.objectives) == 0 {
		return nil, errMaxResState
	}
	result = &ArithmeticResult{Status: lbool.True, Optimal: true, Values: make([]ObjectiveValue, len(s.objectives)), ObjectiveModels: make([]*model.ModelCore, len(s.objectives))}
	if s.initial != nil {
		result.Model, result.Labels = s.initial.Model, s.initial.Labels
		s.initial.Model, s.initial.Labels = nil, nil
		s.initial = nil
	}
	result.InitialLabels = append(result.InitialLabels[:0], result.Labels...)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	var arith []int
	for i := range s.objectives {
		if !s.objectives[i].soft {
			arith = append(arith, i)
		}
	}
	// lbool r = m_optsmt.box();
	if len(arith) != 0 {
		box := *s
		box.objectives = make([]arithmeticObjective, 0, len(arith))
		for _, i := range arith {
			box.objectives = append(box.objectives, s.objectives[i])
		}
		combined, err := box.CheckBox()
		if err != nil {
			return nil, err
		}
		if combined.Status != lbool.True || !combined.Optimal {
			result.Status, result.Optimal, result.ReasonUnknown = combined.Status, false, combined.ReasonUnknown
			return result, combined.Close()
		}
		for k, i := range arith {
			result.Values[i] = combined.Values[k]
			if k < len(combined.ObjectiveModels) {
				result.ObjectiveModels[i] = combined.ObjectiveModels[k]
				combined.ObjectiveModels[k] = nil
			}
		}
		if err = combined.Close(); err != nil {
			return nil, err
		}
	}
	for i := range s.objectives {
		if !s.objectives[i].soft {
			continue
		}
		// solver::scoped_push _sp(get_solver()); r = execute(obj, false, false);
		if err := s.solver().Push(); err != nil {
			return nil, err
		}
		status, optimal, err := s.optimizeSoft(i, result)
		err = errors.Join(err, s.solver().Pop(1))
		if err != nil {
			return nil, err
		}
		if !optimal {
			result.Status, result.Optimal = status, false
			return result, nil
		}
		finalizeSoftBounds(result, i)
		// m_box_models.push_back(m_model.get())
		if result.Model != nil {
			snapshot := &ArithmeticResult{Model: result.Model}
			if err = result.Model.IncRef(); err != nil {
				return nil, err
			}
			if !s.rawModels {
				if err = s.filterResultModel(snapshot); err != nil {
					return nil, errors.Join(err, snapshot.Close())
				}
			}
			result.ObjectiveModels[i] = snapshot.Model
		}
	}
	// m_model = m_box_models[0]
	if first := result.ObjectiveModels[0]; first != nil {
		if err := first.IncRef(); err != nil {
			return nil, err
		}
		if result.Model != nil {
			if err := result.Model.DecRef(); err != nil {
				return nil, errors.Join(err, first.DecRef())
			}
		}
		result.Model = first
		result.ModelFiltered = !s.rawModels
	}
	return result, nil
}
