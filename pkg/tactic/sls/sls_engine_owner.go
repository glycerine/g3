package sls

// Source: src/tactic/sls/sls_engine.cpp:31-48. This file makes the source
// constructor/destructor ownership explicit for production tactic use.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type EngineOwner struct {
	self      *EngineOwner
	numbers   *numeral.IntegerManager
	powers    *powers
	tracker   *Tracker
	search    *SearchTracker
	evaluator *Evaluator
	engine    *Engine
	closed    bool
}

func NewEngineOwner(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, local *uparams.ParamsRef, checkpoint func() error) (owner *EngineOwner, result error) {
	if manager == nil || context == nil || globals == nil || globals.Context != context {
		return nil, ErrSLSSearch
	}
	if err := context.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		return nil, err
	}
	o := &EngineOwner{}
	o.self = o
	defer func() {
		if result != nil {
			result = errors.Join(result, o.Close())
			owner = nil
		}
	}()
	var err error
	o.numbers, err = numeral.NewIntegerManager(numeral.IntegerConfig{
		Cells: arena.Config{PageSize: 64, Budget: manager.Budget()},
		Words: arena.VectorConfig{PageSize: 256, Descriptors: arena.Config{PageSize: 64, Budget: manager.Budget()}},
	})
	if err != nil {
		return nil, err
	}
	o.powers, err = newPowers(o.numbers)
	if err != nil {
		return nil, err
	}
	o.tracker, err = NewTracker(manager, o.numbers, o.powers, context, globals)
	if err != nil {
		return nil, err
	}
	o.search, err = NewSearchTracker(o.tracker, 0)
	if err != nil {
		return nil, err
	}
	o.evaluator, err = NewEvaluator(manager, o.tracker, o.numbers, o.powers)
	if err != nil {
		return nil, err
	}
	o.engine, err = NewEngine(manager, o.tracker, o.search, o.evaluator, context, globals, local, checkpoint)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (o *EngineOwner) open() error {
	if o == nil || o.self != o || o.closed || o.engine == nil || o.engine.open() != nil {
		return ErrSLSSearch
	}
	return nil
}

func (o *EngineOwner) Engine() (*Engine, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.engine, nil
}

func (o *EngineOwner) Close() (result error) {
	if o == nil || o.self != o {
		return ErrSLSSearch
	}
	if o.closed {
		return nil
	}
	if o.engine != nil {
		result = errors.Join(result, o.engine.Close())
		o.engine = nil
	}
	if o.evaluator != nil {
		result = errors.Join(result, o.evaluator.Close())
		o.evaluator = nil
	}
	if o.search != nil {
		result = errors.Join(result, o.search.Close())
		o.search = nil
	}
	if o.tracker != nil {
		result = errors.Join(result, o.tracker.Close())
		o.tracker = nil
	}
	if o.powers != nil {
		result = errors.Join(result, o.powers.closeChecked())
		o.powers = nil
	}
	if o.numbers != nil {
		result = errors.Join(result, o.numbers.CloseChecked())
		o.numbers = nil
	}
	if result == nil {
		o.closed = true
	}
	return result
}
