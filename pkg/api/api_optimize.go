package api

// Source: src/api/api_opt.cpp:32-68.

import "github.com/glycerine/g3/pkg/cmd_context"

type APIOptimize struct {
	owner   *smtContextState
	command *cmd_context.Context
	closed  bool
	last    apiLastHold // m_last_obj's reference (api_last_object.go)
}

func (c *SMTContext) NewOptimize() (*APIOptimize, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	command, err := c.command.NewParserContext()
	if err != nil {
		return nil, err
	}
	result := &APIOptimize{owner: c.smtContextState, command: command}
	if err = c.saveHeld(result, &result.last); err != nil {
		return nil, err
	}
	return result, nil
}

func (o *APIOptimize) open() error {
	if o == nil || o.owner == nil || o.command == nil || o.closed || o.owner.closed || o.last.closed {
		return ErrSMTContextOwner
	}
	return nil
}

// Close releases the handle's reference: the handle is unusable at once;
// the object dies now, or when the context's last-object reference to it
// is released (api_last_object.go).
func (o *APIOptimize) Close() error {
	if o == nil || o.last.closed {
		return nil
	}
	if !o.last.close() {
		return nil
	}
	return o.destroy()
}

func (o *APIOptimize) lastRelease() error { return o.last.release(o.destroy) }

func (o *APIOptimize) destroy() error {
	if o == nil || o.closed {
		return nil
	}
	if o.command == nil {
		return ErrSMTContextOwner
	}
	if err := o.command.Close(); err != nil {
		return err
	}
	o.closed = true
	o.owner, o.command = nil, nil
	return nil
}
