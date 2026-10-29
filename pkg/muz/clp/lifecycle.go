package clp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (e *Engine) Statistics() (Statistics, error) {
	if err := e.open(); err != nil {
		return Statistics{}, err
	}
	return e.stats, nil
}
func (e *Engine) ResetStatistics() error {
	if err := e.open(); err != nil {
		return err
	}
	e.stats = Statistics{}
	return nil
}
func (e *Engine) Answer() (ast.ASTHandle, error) {
	if err := e.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	h, err := e.manager.RefGet(e.answer)
	if err != nil || h.IsNull() {
		return e.manager.MkTrue()
	}
	return h, nil
}
func (e *Engine) Close() error {
	if e == nil {
		return ErrCLP
	}
	if e.closed {
		return nil
	}
	var err error
	if e.answer != (ast.ASTRef{}) {
		err = e.manager.DelRef(&e.answer)
		if err != nil {
			return errors.Join(err, ErrCLP)
		}
	}
	e.closed = true
	e.manager = nil
	e.rules = nil
	e.solver = nil
	return nil
}
