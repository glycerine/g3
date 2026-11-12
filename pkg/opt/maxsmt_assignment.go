package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::init and assignment cost
// accounting.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/model"
)

func (s *ArithmeticSolver) evaluateSoftDetailed(objective *arithmeticObjective, md *model.ModelCore) (cost, satisfied big.Rat, assignment []bool, resultErr error) {
	return s.evaluateSoftAssignment(objective, md, true)
}

// evaluateSoftAssignment evaluates the soft terms in md. maxsmt_solver_base::init
// uses m_model->eval(m_soft[i], val) without model completion, so a soft
// term over a constant the model leaves unassigned counts as false there;
// the maxres is_true helpers evaluate with completion.
func (s *ArithmeticSolver) evaluateSoftAssignment(objective *arithmeticObjective, md *model.ModelCore, completion bool) (cost, satisfied big.Rat, assignment []bool, resultErr error) {
	evaluator, err := model.NewModelEvaluator(md, s.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return cost, satisfied, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(completion); err != nil {
		return cost, satisfied, nil, err
	}
	cost.Set(&objective.softOffset)
	assignment = make([]bool, len(objective.softTerms))
	for i := range objective.softTerms {
		term, e := s.manager.RefGet(objective.softTerms[i])
		if e != nil {
			return cost, satisfied, nil, e
		}
		value, e := evaluator.EvaluateValue(term)
		if e != nil {
			return cost, satisfied, nil, e
		}
		resolved, e := s.manager.RefGet(value)
		truth := e == nil && s.manager.IsTrue(resolved)
		e = errors.Join(e, s.manager.DelRef(&value))
		if e != nil {
			return cost, satisfied, nil, e
		}
		assignment[i] = truth
		if truth {
			satisfied.Add(&satisfied, objective.softWeights[i])
		} else {
			cost.Add(&cost, objective.softWeights[i])
		}
	}
	return cost, satisfied, assignment, nil
}
