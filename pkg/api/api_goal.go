package api

// Source: src/api/api_goal.cpp:27-57.
//
// APIGoal is the public Go ownership bridge for Z3_goal_ref. It owns one
// concrete tactic goal and borrows its SMTContext; Close replaces opaque
// inc_ref/dec_ref traffic at the Go boundary.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APIGoal struct {
	self            *APIGoal
	owner           *smtContextState
	manager         *ast.ASTManager
	goal            *tactic.Goal
	closed          bool
	shared, cleanup bool
	last            apiLastHold // m_last_obj's reference (api_last_object.go)
}

func (c *SMTContext) NewGoal(models, unsatCores, proofs bool) (*APIGoal, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	goal, err := tactic.NewGoal(manager, proofs, models, unsatCores)
	if err != nil {
		return nil, err
	}
	result := &APIGoal{owner: c.smtContextState, manager: manager, goal: goal}
	result.self = result
	if err = c.saveHeld(result, &result.last); err != nil {
		return nil, err
	}
	return result, nil
}

func (g *APIGoal) open() error {
	if g == nil || g.self != g || g.owner == nil || g.manager == nil || g.goal == nil || g.closed || g.owner.closed || g.owner.command == nil || g.last.closed {
		return ErrSMTContextOwner
	}
	return g.goal.CheckOwner()
}

// Close releases the handle's reference: the handle is unusable at once;
// the object dies now, or when the context's last-object reference to it
// is released (api_last_object.go).
func (g *APIGoal) Close() error {
	if g == nil || g.last.closed {
		return nil
	}
	if g.self != g {
		return ErrSMTContextOwner
	}
	if !g.last.close() {
		return nil
	}
	return g.destroy()
}

func (g *APIGoal) lastRelease() error { return g.last.release(g.destroy) }

func (g *APIGoal) destroy() error {
	if g == nil || g.closed {
		return nil
	}
	if g.self != g || g.goal == nil {
		return ErrSMTContextOwner
	}
	var err error
	if g.shared && !g.cleanup {
		var consumed bool
		consumed, err = g.goal.DecRefConsumed()
		if consumed {
			g.cleanup = err != nil
		}
	} else {
		err = g.goal.Close()
	}
	if err == nil {
		g.closed = true
		g.owner, g.manager, g.goal = nil, nil, nil
	}
	return err
}
