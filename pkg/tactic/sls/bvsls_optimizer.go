package sls

// Source: src/tactic/sls/bvsls_opt_engine.h:16-82 and
// src/tactic/sls/bvsls_opt_engine.cpp:27-116.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrBVSLSOptimizer = errors.New("sls: invalid bit-vector optimizer state")

// OptimizationResult preserves the source distinction between an unknown
// search and a satisfiable search whose best value happens to be zero. The
// optimum is borrowed from the optimizer until its next Optimize or Close.
type OptimizationResult struct {
	Status  lbool.LBool
	Optimum ast.ASTHandle
}

// BVOptimizer is the source bvsls_opt_engine specialization. The embedded
// Engine owns the hard-constraint state. The additional tracker/evaluator
// owns the objective DAG while borrowing the same numeral manager and powers.
type BVOptimizer struct {
	self      *BVOptimizer
	hard      *Engine
	objective *Tracker
	search    *SearchTracker
	evaluator *Evaluator

	objectiveExpr ast.ASTHandle
	optimumRef    ast.ASTHandle
	objectiveBits uint32
	bestScore     numeral.Int
	bestModel     *model.Model
	maximize      bool
	closed        bool
}

func NewBVOptimizer(hard *Engine) (*BVOptimizer, error) {
	if hard == nil || hard.open() != nil {
		return nil, ErrBVSLSOptimizer
	}
	tracker, err := NewTracker(hard.manager, hard.numbers, hard.powers, hard.context, hard.globals)
	if err != nil {
		return nil, err
	}
	search, err := NewSearchTracker(tracker, 0)
	if err != nil {
		_ = tracker.Close()
		return nil, err
	}
	evaluator, err := NewEvaluator(hard.manager, tracker, hard.numbers, hard.powers)
	if err != nil {
		_ = search.Close()
		_ = tracker.Close()
		return nil, err
	}
	o := &BVOptimizer{hard: hard, objective: tracker, search: search, evaluator: evaluator, bestScore: numeral.NewInt(0)}
	o.self = o
	return o, nil
}

func (o *BVOptimizer) open() error {
	if o == nil || o.self != o || o.closed || o.hard == nil || o.hard.open() != nil || o.objective == nil || o.objective.open() != nil || o.search == nil || o.search.open() != nil || o.evaluator == nil || o.evaluator.open() != nil {
		return ErrBVSLSOptimizer
	}
	return nil
}

func (o *BVOptimizer) setupObjective(objective ast.ASTHandle, maximize bool) error {
	if err := o.open(); err != nil {
		return err
	}
	sort, err := o.hard.manager.GetSort(objective)
	if err != nil {
		return err
	}
	isBV, err := o.hard.manager.IsBitVectorSort(sort)
	if err != nil {
		return err
	}
	if !isBV {
		return ErrBVSLSOptimizer
	}
	obj := objective
	if !maximize {
		family, x := o.hard.manager.BitVectorUtilityFamilyID()
		if x != nil {
			return x
		}
		obj, x = o.hard.manager.MkFamilyApp(family, 3, nil, []ast.ASTHandle{objective}) // OP_BNEG
		if x != nil {
			return x
		}
	}
	if !o.objectiveExpr.IsNull() {
		if err = o.hard.manager.DecRef(o.objectiveExpr); err != nil {
			return err
		}
	}
	if err = o.hard.manager.IncRef(obj); err != nil {
		return err
	}
	o.objectiveExpr = obj
	sort, err = o.hard.manager.GetSort(obj)
	if err != nil {
		return err
	}
	o.objectiveBits, err = o.hard.manager.BitVectorSize(sort)
	if err != nil {
		return err
	}
	o.maximize = maximize
	if err = o.search.Initialize([]ast.ASTHandle{obj}); err != nil {
		return err
	}
	return o.search.Reset()
}

// Optimize follows the source's hard-satisfaction phase before entering the
// objective hill-climber. A supplied model seeds only declarations tracked by
// the hard formula, as in sls_tracker::set_model.
func (o *BVOptimizer) Optimize(objective ast.ASTHandle, initial *model.Model, maximize bool) (result OptimizationResult, resultErr error) {
	result = OptimizationResult{Status: lbool.Undef}
	if err := o.open(); err != nil {
		return result, err
	}
	if !o.optimumRef.IsNull() {
		if err := o.hard.manager.DecRef(o.optimumRef); err != nil {
			return result, err
		}
		o.optimumRef = ast.ASTHandle{}
	}
	if err := o.hard.search.Initialize(o.hard.assertions); err != nil {
		return result, err
	}
	if err := o.hard.search.Reset(); err != nil {
		return result, err
	}
	if err := o.setupObjective(objective, maximize); err != nil {
		return result, err
	}
	if initial != nil {
		if err := o.hard.tracker.SetModel(initial); err != nil {
			return result, err
		}
		if err := o.hard.evaluator.UpdateAll(o.hard.search); err != nil {
			return result, err
		}
	}
	sat, err := o.hard.tracker.IsSAT()
	if err != nil {
		return result, err
	}
	status := lbool.Undef
	if sat {
		status = lbool.True
	}
	oldBest := numeral.NewInt(0)
	defer func() { resultErr = errors.Join(resultErr, o.hard.numbers.Del(&oldBest)) }()
	for o.hard.stats.Restarts = 0; o.hard.stats.Restarts < o.hard.maxRestarts; o.hard.stats.Restarts++ {
		if err = o.hard.numbers.Copy(&oldBest, &o.bestScore); err != nil {
			return result, err
		}
		if status != lbool.True {
			for {
				if err = o.hard.checkpointNow(); err != nil {
					return result, err
				}
				status, err = o.hard.Search()
				if err != nil {
					return result, err
				}
				if status == lbool.Undef {
					if err = o.hard.search.Randomize(); err != nil {
						return result, err
					}
				}
				if status == lbool.True {
					break
				}
				previous := o.hard.stats.Restarts
				o.hard.stats.Restarts++
				if previous >= o.hard.maxRestarts {
					break
				}
			}
		}
		if status == lbool.True {
			result.Status = lbool.True
			hardModel, e := o.hard.tracker.GetModel()
			if e != nil {
				return result, e
			}
			e = o.objective.SetModel(hardModel)
			e = errors.Join(e, hardModel.Close())
			if e != nil {
				return result, e
			}
			if err = o.evaluator.UpdateAll(o.search); err != nil {
				return result, err
			}
			optimum, e := o.maximizeObjective()
			if e != nil {
				return result, e
			}
			var improved bool
			if maximize {
				improved, e = o.hard.numbers.Greater(&o.bestScore, &oldBest)
			} else {
				improved, e = o.hard.numbers.Less(&o.bestScore, &oldBest)
			}
			if e != nil {
				return result, e
			}
			if improved {
				if e = o.hard.manager.IncRef(optimum); e != nil {
					return result, e
				}
				if !o.optimumRef.IsNull() {
					if e = o.hard.manager.DecRef(o.optimumRef); e != nil {
						return result, errors.Join(e, o.hard.manager.DecRef(optimum))
					}
				}
				o.optimumRef, result.Optimum = optimum, optimum
			}
		}
		if err = o.hard.search.Randomize(); err != nil {
			return result, err
		}
		if err = o.hard.evaluator.UpdateAll(o.hard.search); err != nil {
			return result, err
		}
		sat, err = o.hard.tracker.IsSAT()
		if err != nil {
			return result, err
		}
		status = lbool.Undef
		if sat {
			status = lbool.True
		}
	}
	return result, nil
}

func (o *BVOptimizer) Close() (result error) {
	if o == nil || o.self != o {
		return ErrBVSLSOptimizer
	}
	if o.closed {
		return nil
	}
	if o.bestModel != nil {
		result = errors.Join(result, o.bestModel.Close())
		o.bestModel = nil
	}
	if !o.optimumRef.IsNull() {
		result = errors.Join(result, o.hard.manager.DecRef(o.optimumRef))
		o.optimumRef = ast.ASTHandle{}
	}
	if !o.objectiveExpr.IsNull() {
		result = errors.Join(result, o.hard.manager.DecRef(o.objectiveExpr))
		o.objectiveExpr = ast.ASTHandle{}
	}
	result = errors.Join(result, o.hard.numbers.Del(&o.bestScore))
	result = errors.Join(result, o.evaluator.Close())
	result = errors.Join(result, o.search.Close())
	result = errors.Join(result, o.objective.Close())
	if result == nil {
		o.closed = true
	}
	return result
}
