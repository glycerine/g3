package api

// Source: src/api/api_tactic.cpp:49-77,185-200.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APITactic struct {
	owner  *smtContextState
	tactic tactic.Tactic
	closed bool
}

func newAPITactic(owner *smtContextState, value tactic.Tactic) (*APITactic, error) {
	if owner == nil || value == nil {
		return nil, ast.ErrASTHandle
	}
	if err := value.IncRef(); err != nil {
		return nil, err
	}
	return &APITactic{owner: owner, tactic: value}, nil
}

func (t *APITactic) open() error {
	if t == nil || t.owner == nil || t.tactic == nil || t.closed || t.owner.closed || t.owner.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (t *APITactic) Close() error {
	if t == nil || t.closed {
		return nil
	}
	if t.tactic == nil {
		return ErrSMTContextOwner
	}
	err := t.tactic.DecRef()
	if err == nil {
		t.closed = true
		t.owner, t.tactic = nil, nil
	}
	return err
}

func (c *SMTContext) Tactic(name string) (*APITactic, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	value, err := c.command.NewBuiltinTactic(name)
	if err != nil {
		return nil, err
	}
	return newAPITactic(c.smtContextState, value)
}

func (c *SMTContext) SkipTactic() (*APITactic, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return newAPITactic(c.smtContextState, tactic.NewSkipTactic())
}

func (c *SMTContext) FailTactic() (*APITactic, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return newAPITactic(c.smtContextState, tactic.NewFailTactic())
}

func (c *SMTContext) FailIfUndecidedTactic() (*APITactic, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return newAPITactic(c.smtContextState, tactic.NewFailIfUndecidedTactic())
}
