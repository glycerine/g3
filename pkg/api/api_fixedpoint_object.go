package api

// Source: src/api/api_datalog.cpp:39-52, 144-162.

import "github.com/glycerine/g3/pkg/cmd_context"

// APIFixedpoint is an owning fixedpoint engine view. It shares its parent's
// AST manager but owns declarations, rules, assertions, scopes, and query
// state, matching the independent Z3_fixedpoint object lifetime.
type APIFixedpoint struct {
	owner   *smtContextState
	command *cmd_context.Context
	closed  bool
}

func (c *SMTContext) NewFixedpoint() (*APIFixedpoint, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	command, err := c.command.NewParserContext()
	if err != nil {
		return nil, err
	}
	return &APIFixedpoint{owner: c.smtContextState, command: command}, nil
}

func (f *APIFixedpoint) open() error {
	if f == nil || f.owner == nil || f.command == nil || f.closed || f.owner.closed {
		return ErrSMTContextOwner
	}
	return nil
}

func (f *APIFixedpoint) Close() error {
	if f == nil || f.closed {
		return nil
	}
	if f.command == nil {
		return ErrSMTContextOwner
	}
	if err := f.command.Close(); err != nil {
		return err
	}
	f.closed = true
	f.owner, f.command = nil, nil
	return nil
}
