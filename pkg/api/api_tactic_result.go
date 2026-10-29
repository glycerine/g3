package api

// Source: src/api/api_tactic.cpp:461-531.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APITacticResult struct {
	owner          *smtContextState
	manager        *ast.ASTManager
	goals          []*tactic.Goal
	modelConverter tactic.ModelConverter
	closed         bool
}

func (r *APITacticResult) open() error {
	if r == nil || r.owner == nil || r.manager == nil || r.closed || r.owner.closed || r.owner.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (r *APITacticResult) Close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	for i := len(r.goals) - 1; i >= 0; i-- {
		if r.goals[i] == nil {
			continue
		}
		if err := r.goals[i].Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			r.goals[i] = nil
		}
	}
	if r.modelConverter != nil {
		if err := r.modelConverter.DecRef(); err != nil {
			result = errors.Join(result, err)
		} else {
			r.modelConverter = nil
		}
	}
	if result == nil {
		r.goals = nil
		r.closed = true
		r.owner, r.manager = nil, nil
	}
	return result
}

func (r *APITacticResult) NumSubgoals() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return uint32(len(r.goals)), nil
}

// Subgoal returns an independently owning copy, matching the source goal
// wrapper returned from an apply result.
func (r *APITacticResult) Subgoal(index uint32) (*APIGoal, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if uint64(index) >= uint64(len(r.goals)) {
		return nil, ast.ErrASTDomain
	}
	goal, err := tactic.NewGoalCopy(r.goals[index])
	if err != nil {
		return nil, err
	}
	return &APIGoal{owner: r.owner, manager: r.manager, goal: goal}, nil
}

func (r *APITacticResult) String() (string, error) {
	if err := r.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	output.WriteString("(goals\n")
	for i := range r.goals {
		view := &APIGoal{owner: r.owner, manager: r.manager, goal: r.goals[i]}
		text, err := view.String()
		if err != nil {
			return "", err
		}
		output.WriteString(text)
		output.WriteByte('\n')
	}
	output.WriteByte(')')
	return output.String(), nil
}
