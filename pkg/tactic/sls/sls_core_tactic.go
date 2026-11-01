package sls

// Source: src/tactic/sls/sls_tactic.cpp:25-91 and
// src/tactic/sls/sls_engine.cpp:50-100.

import (
	"errors"
	"time"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type CoreTactic struct {
	self      *CoreTactic
	manager   *ast.ASTManager
	context   *uparams.ParamsContext
	globals   *uparams.GlobalParamsService
	params    uparams.ParamsRef
	owner     *EngineOwner
	progress  func(uint32)
	refs      uint32
	paramLive bool
	closed    bool
}

var _ tactic.Tactic = (*CoreTactic)(nil)

func NewCoreTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (answer *CoreTactic, result error) {
	if manager == nil || context == nil || globals == nil || globals.Context != context {
		return nil, ErrSLSSearch
	}
	t := &CoreTactic{manager: manager, context: context, globals: globals}
	t.self = t
	defer func() {
		if result != nil {
			result = errors.Join(result, t.Close())
			answer = nil
		}
	}()
	var err error
	t.params, err = context.NewValue()
	t.paramLive = err == nil
	if err != nil {
		return nil, err
	}
	if params != nil {
		if err = context.Assign(&t.params, params); err != nil {
			return nil, err
		}
	}
	if err = t.rebuild(); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *CoreTactic) open() error {
	if t == nil || t.self != t {
		return ErrSLSSearch
	}
	if t.closed {
		return arena.ErrClosed
	}
	if err := t.context.Check(&t.params); err != nil {
		return err
	}
	return t.owner.open()
}

func (t *CoreTactic) rebuild() error {
	owner, err := NewEngineOwner(t.manager, t.context, t.globals, &t.params, nil)
	if err != nil {
		return err
	}
	if t.owner != nil {
		if err = t.owner.Close(); err != nil {
			return errors.Join(err, owner.Close())
		}
	}
	t.owner = owner
	return nil
}

func (t *CoreTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *CoreTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}

func (t *CoreTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	_, err := ApplyGoal(t.owner.engine, goal, result)
	return err
}

func (t *CoreTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if params == nil {
		empty := t.context.EmptyValue()
		params = &empty
	}
	if err := t.context.Assign(&t.params, params); err != nil {
		return err
	}
	return t.owner.engine.UpdateParams(&t.params)
}

func (t *CoreTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
		return uparams.ErrParamDescrsPrecondition
	}
	return catalogCollectSlsParams(out)
}

func (t *CoreTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.self != t || t.closed || t.owner == nil || t.owner.engine == nil {
		return nil
	}
	s := t.owner.engine.stats
	seconds := time.Since(s.Started).Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	return map[string]float64{
		"sls restarts":       float64(s.Restarts),
		"sls full evals":     float64(s.FullEvaluations),
		"sls incr evals":     float64(s.IncrementalEvaluations),
		"sls incr evals/sec": float64(s.IncrementalEvaluations) / seconds,
		"sls FLIP moves":     float64(s.Flips),
		"sls INC moves":      float64(s.Increments),
		"sls DEC moves":      float64(s.Decrements),
		"sls INV moves":      float64(s.Inversions),
		"sls moves":          float64(s.Moves),
		"sls moves/sec":      float64(s.Moves) / seconds,
	}
}

func (t *CoreTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.owner.engine.ResetStatistics()
}

func (t *CoreTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.rebuild()
}

func (t *CoreTactic) Reset() error { return t.Cleanup() }

func (t *CoreTactic) SetLogic(usymbol.Symbol) error {
	return t.open()
}

func (t *CoreTactic) SetProgressCallback(callback func(uint32)) error {
	if err := t.open(); err != nil {
		return err
	}
	t.progress = callback
	return nil
}

func (t *CoreTactic) Translate(manager *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewCoreTactic(manager, t.context, t.globals, &t.params)
}

func (t *CoreTactic) Close() (result error) {
	if t == nil || t.self != t {
		return ErrSLSSearch
	}
	if t.closed {
		return nil
	}
	if t.owner != nil {
		result = errors.Join(result, t.owner.Close())
		t.owner = nil
	}
	if t.paramLive {
		result = errors.Join(result, t.context.Del(&t.params))
		t.paramLive = false
	}
	if result == nil {
		t.closed = true
	}
	return result
}
