package api

// Source: src/api/api_tactic.cpp:439-459.

import (
	"errors"

	"github.com/glycerine/g3/pkg/tactic"
)

func (t *APITactic) Apply(goal *APIGoal) (result *APITacticResult, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := goal.open(); err != nil {
		return nil, err
	}
	if t.owner != goal.owner {
		return nil, ErrSMTContextOwner
	}
	working, err := tactic.NewGoalCopy(goal.goal)
	if err != nil {
		return nil, err
	}
	native := &tactic.TacticResult{}
	defer func() {
		resultErr = errors.Join(resultErr, native.Close(), working.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if err = t.tactic.Apply(working, native); err != nil {
		return nil, err
	}
	result = &APITacticResult{owner: t.owner, manager: goal.manager}
	for _, subgoal := range native.Goals {
		copy, copyErr := tactic.NewGoalCopy(subgoal)
		if copyErr != nil {
			return result, copyErr
		}
		result.goals = append(result.goals, copy)
	}
	return result, nil
}
