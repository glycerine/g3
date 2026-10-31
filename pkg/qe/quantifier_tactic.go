package qe

// Source: src/qe/qe_tactic.cpp/.h, lifecycle and quantifier-free goal path.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// QuantifierTactic handles quantifier-free goals and Boolean quantifiers that
// source unit and polarity rules eliminate completely. General expression
// elimination remains under construction.
type QuantifierTactic struct {
	self                *QuantifierTactic
	manager             *ast.ASTManager
	config              Config
	params              uparams.ParamsRef
	options             quantifierTacticOptions
	refs                uint32
	bytes               uint64
	paramsReady, closed bool
}

var _ tactic.Tactic = (*QuantifierTactic)(nil)

func NewQuantifierTactic(m *ast.ASTManager, params *uparams.ParamsRef, config Config) (*QuantifierTactic, error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	size := uint64(reflect.TypeOf(QuantifierTactic{}).Size())
	if err := m.Budget().Reserve(size); err != nil {
		return nil, err
	}
	t := &QuantifierTactic{manager: m, config: config, bytes: size}
	t.self = t
	var err error
	t.params, err = config.Context.NewValue()
	t.paramsReady = err == nil
	if err == nil && params != nil {
		err = config.Context.Assign(&t.params, params)
	}
	if err == nil {
		err = t.options.update(config.Context, &t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func (t *QuantifierTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.config.Context.Check(&t.params)
}
func (t *QuantifierTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *QuantifierTactic) DecRef() error {
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
func (t *QuantifierTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.paramsReady {
		if err := t.config.Context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
func (t *QuantifierTactic) Cleanup() error { return t.open() }
func (t *QuantifierTactic) Reset() error   { return t.open() }
func (t *QuantifierTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewQuantifierTactic(m, &t.params, t.config)
}
func (t *QuantifierTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return ast.ErrASTHandle
	}
	if err := t.config.Context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.options.update(t.config.Context, p)
}
func (t *QuantifierTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return collectQuantifierTacticOptions(out)
}
func (t *QuantifierTactic) CollectStatistics() map[string]float64  { return nil }
func (t *QuantifierTactic) ResetStatistics() error                 { return t.open() }
func (t *QuantifierTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *QuantifierTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *QuantifierTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != t.manager || out == nil {
		return ast.ErrASTHandle
	}
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	n, err := goal.Size()
	if err != nil {
		return err
	}
	updates := make([]ast.ASTRef, n)
	defer func() {
		for i := range updates {
			if updates[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, t.manager.DelRef(&updates[i]))
			}
		}
	}()
	for i := uint32(0); i < n; i++ {
		canceled, e := t.manager.Canceled()
		if e != nil {
			return e
		}
		if canceled {
			return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
		}
		if e = t.config.Cooperation.Checkpoint("qe"); e != nil {
			return e
		}
		inconsistent, e := goal.Inconsistent()
		if e != nil {
			return e
		}
		if inconsistent {
			break
		}
		form, e := goal.Form(i)
		if e != nil {
			return e
		}
		quantified, e := t.manager.HasQuantifiers(form)
		if e != nil {
			return e
		}
		if quantified {
			proofs, e := goal.ProofsEnabled()
			if e != nil {
				return e
			}
			if proofs {
				return tactic.NewTacticException("quantifier elimination proofs are not implemented")
			}
			var complete bool
			updates[i], complete, e = eliminateQuantifierBoolUnits(t.manager, form, t.config)
			if e != nil {
				return e
			}
			if !complete {
				return tactic.NewTacticException("general quantifier elimination is not implemented")
			}
		}
	}
	for i, update := range updates {
		inconsistent, e := goal.Inconsistent()
		if e != nil {
			return e
		}
		if inconsistent {
			break
		}
		if update == (ast.ASTRef{}) {
			continue
		}
		formula, e := t.manager.RefGet(update)
		if e != nil {
			return e
		}
		dependency, e := goal.Dependency(uint32(i))
		if e != nil {
			return e
		}
		if e = goal.Update(uint32(i), formula, ast.ASTHandle{}, dependency); e != nil {
			return e
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = goal.ElimTrue(); err != nil {
		return err
	}
	return out.AddGoal(goal)
}
