package api

// Source: src/api/api_goal.cpp:154-165.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (g *APIGoal) Translate(target *SMTContext) (result *APIGoal, resultErr error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if target == nil {
		return nil, ErrSMTContextOwner
	}
	if err := target.open(); err != nil {
		return nil, err
	}
	targetManager, err := target.Manager()
	if err != nil {
		return nil, err
	}
	// Z3_goal_translate has no same-context check: an ast_translation of
	// a manager onto itself rebuilds every node as the hash-consed node it
	// already is, so goal::translate yields a copy (goal::copy_to keeps the
	// same flags, depth and precision). ast.ASTTranslation refuses from ==
	// to, so the same-context case copies directly.
	if target.smtContextState == g.owner {
		copied, e := tactic.NewGoalCopy(g.goal)
		if e != nil {
			return nil, e
		}
		result = &APIGoal{owner: target.smtContextState, manager: targetManager, goal: copied}
		result.self = result
		if err = target.saveHeld(result, &result.last); err != nil {
			return nil, err
		}
		return result, nil
	}
	translation, err := ast.NewASTTranslation(g.manager, targetManager, true)
	if err != nil {
		return nil, err
	}
	translated, err := g.goal.Translate(translation)
	if err != nil {
		return nil, errors.Join(err, translation.Close())
	}
	if err = translation.Close(); err != nil {
		return nil, errors.Join(err, translated.Close())
	}
	result = &APIGoal{owner: target.smtContextState, manager: targetManager, goal: translated}
	result.self = result
	if err = target.saveHeld(result, &result.last); err != nil {
		return nil, err
	}
	return result, nil
}
