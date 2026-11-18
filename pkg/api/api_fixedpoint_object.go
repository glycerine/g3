package api

// Source: src/api/api_datalog.cpp:39-52, 144-162.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

// APIFixedpoint is an owning fixedpoint engine view. It shares its parent's
// AST manager but owns declarations, rules, assertions, scopes, and query
// state, matching the independent Z3_fixedpoint object lifetime.
type APIFixedpoint struct {
	owner   *smtContextState
	command *cmd_context.Context
	// params mirrors api::Z3_fixedpoint_ref::m_params: the latest API update.
	params *ParameterSet
	// engineParams mirrors datalog::context::m_params_ref: cumulative updates.
	engineParams      *ParameterSet
	retiredParams     []*ParameterSet
	timeout           uint32
	hasTimeout        bool
	lastQuery         ast.ASTHandle
	hasLastQuery      bool
	hasLastExpression bool
	hasLastRelations  bool
	closed            bool
	last              apiLastHold // m_last_obj's reference (api_last_object.go)
}

func (c *SMTContext) NewFixedpoint() (*APIFixedpoint, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	command, err := c.command.NewParserContext()
	if err != nil {
		return nil, err
	}
	result := &APIFixedpoint{owner: c.smtContextState, command: command}
	if err = c.saveHeld(result, &result.last); err != nil {
		return nil, err
	}
	return result, nil
}

func (f *APIFixedpoint) open() error {
	if f == nil || f.owner == nil || f.command == nil || f.closed || f.owner.closed || f.last.closed {
		return ErrSMTContextOwner
	}
	return nil
}

// Close releases the handle's reference: the handle is unusable at once;
// the object dies now, or when the context's last-object reference to it
// is released (api_last_object.go).
func (f *APIFixedpoint) Close() error {
	if f == nil || f.last.closed {
		return nil
	}
	if !f.last.close() {
		return nil
	}
	return f.destroy()
}

func (f *APIFixedpoint) lastRelease() error { return f.last.release(f.destroy) }

func (f *APIFixedpoint) destroy() error {
	if f == nil || f.closed {
		return nil
	}
	if f.command == nil {
		return ErrSMTContextOwner
	}
	if f.params != nil {
		if err := f.params.Close(); err != nil {
			return err
		}
		f.params = nil
	}
	if f.engineParams != nil {
		if err := f.engineParams.Close(); err != nil {
			return err
		}
		f.engineParams = nil
	}
	for len(f.retiredParams) != 0 {
		i := len(f.retiredParams) - 1
		if err := f.retiredParams[i].Close(); err != nil {
			return err
		}
		f.retiredParams = f.retiredParams[:i]
	}
	if err := f.command.Close(); err != nil {
		return err
	}
	f.closed = true
	f.owner, f.command = nil, nil
	return nil
}
