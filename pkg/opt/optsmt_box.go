package opt

// Source: src/opt/optsmt.cpp geometric_opt, update_lower and set_max.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// CheckBox computes the joint arithmetic box optimum and keeps one independent
// model snapshot per objective. The returned ObjectiveModels are owned by the
// result and follow objective registration order.
func (s *ArithmeticSolver) CheckBox() (result *ArithmeticResult, resultErr error) {
	if s == nil || s.backend == nil || len(s.objectives) == 0 {
		return nil, ast.ErrASTHandle
	}
	for _, objective := range s.objectives {
		if objective.soft {
			return nil, ast.ErrASTDomain
		}
	}
	result = &ArithmeticResult{Status: lbool.Undef, Values: make([]ObjectiveValue, len(s.objectives))}
	owner := maxSMTASTOwner{manager: s.manager}
	defer func() {
		resultErr = errors.Join(resultErr, owner.close())
		if resultErr == nil && result != nil && result.Status == lbool.True {
			var unknown bool
			unknown, resultErr = s.backend.OptimizationWasUnknown()
			if resultErr == nil && unknown {
				result.Status, result.Optimal = lbool.Undef, false
				result.ReasonUnknown, resultErr = s.backend.ReasonUnknown()
			}
		}
		if resultErr == nil && result != nil {
			resultErr = s.filterResultModel(result)
			for i, snapshot := range result.ObjectiveModels {
				if resultErr != nil || snapshot == nil {
					break
				}
				filtered := &ArithmeticResult{Model: snapshot}
				resultErr = s.filterResultModel(filtered)
				result.ObjectiveModels[i] = filtered.Model
			}
		}
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if err := s.backend.Push(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.backend.Pop(1)) }()
	status, err := s.backend.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	result.Status = status
	if status != lbool.True {
		if status == lbool.Undef {
			result.ReasonUnknown, err = s.backend.ReasonUnknown()
		}
		return result, err
	}
	if err = s.replaceModel(result); err != nil {
		return nil, err
	}
	result.InitialLabels = append([]usymbol.Symbol(nil), result.Labels...)
	lower := make([]simplex.Infinitesimal, len(s.objectives))
	lowerFormulas := make([]ast.ASTRef, len(s.objectives))
	for i := range lower {
		lower[i].Infinity.SetInt64(-1)
		truth, cause := s.manager.MkTrue()
		if cause != nil {
			return nil, cause
		}
		ref, cause := s.manager.NewRef(truth)
		if cause != nil {
			return nil, cause
		}
		owner.refs = append(owner.refs, ref)
		lowerFormulas[i] = ref
	}
	var deltaScopes uint32
	defer func() {
		if deltaScopes != 0 {
			resultErr = errors.Join(resultErr, s.backend.Pop(deltaScopes))
		}
	}()
	var delta big.Int
	delta.SetInt64(1)
	var steps, stepIncs uint32
	status, err = s.geometricBoxSearch(result, &owner, lower, lowerFormulas, &delta, &steps, &stepIncs, &deltaScopes)
	if err != nil {
		return nil, err
	}
	if status == lbool.Undef {
		result.ReasonUnknown, err = s.backend.ReasonUnknown()
		result.Status = status
		return result, err
	}
	if status != lbool.False {
		result.Status = status
		return result, nil
	}
	result.Status, result.Optimal = lbool.True, true
	result.ObjectiveModels = make([]*model.ModelCore, len(s.objectives))
	for i := range result.ObjectiveModels {
		result.ObjectiveModels[i], err = s.backend.AcquireSavedObjectiveModel(uint32(i))
		if err != nil {
			return nil, err
		}
		if result.ObjectiveModels[i] == nil {
			return nil, ast.ErrASTHandle
		}
		value, cause := s.backend.SavedObjectiveValue(uint32(i))
		if cause != nil {
			return nil, cause
		}
		result.Values[i].Arithmetic = s.orient(i, value)
		if err = s.captureArithmeticModel(i, &result.Values[i], result.ObjectiveModels[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *ArithmeticSolver) geometricBoxSearch(result *ArithmeticResult, owner *maxSMTASTOwner, lower []simplex.Infinitesimal, lowerFormulas []ast.ASTRef, delta *big.Int, steps, stepIncs *uint32, deltaScopes *uint32) (lbool.LBool, error) {
	for {
		status, err := s.backend.CheckSat(nil)
		if err != nil {
			return lbool.Undef, err
		}
		if status != lbool.True {
			if status == lbool.False && delta.Cmp(big.NewInt(1)) > 0 {
				if *deltaScopes == 0 {
					return lbool.Undef, ast.ErrASTDomain
				}
				if err = s.backend.Pop(1); err != nil {
					return lbool.Undef, err
				}
				*deltaScopes--
				delta.SetInt64(1)
				*steps, *stepIncs = 0, 0
				continue
			}
			return status, nil
		}
		if err = s.replaceModel(result); err != nil {
			return lbool.Undef, err
		}
		previous := make([]simplex.Infinitesimal, len(lower))
		for i := range lower {
			previous[i] = lower[i].Clone()
		}
		blockers := make([]ast.ASTHandle, len(s.objectives))
		for i := range s.objectives {
			value, blocker, cause := s.backend.MaximizeOptimizationObjective(s.objectives[i].backendIndex, s.filter)
			if cause != nil {
				if blocker != (ast.ASTRef{}) {
					cause = errors.Join(cause, s.manager.DelRef(&blocker))
				}
				return lbool.Undef, cause
			}
			bound := value.Finite.Clone()
			if value.Unbounded {
				bound.Infinity.SetInt64(1)
			}
			if bound.Cmp(&lower[i]) >= 0 {
				lower[i] = bound
				if err = owner.take(blocker); err != nil {
					return lbool.Undef, err
				}
				lowerFormulas[i] = owner.refs[len(owner.refs)-1]
				if bound.Infinity.Sign() > 0 {
					falsity, cause := s.manager.MkFalse()
					if cause != nil {
						return lbool.Undef, cause
					}
					ref, cause := s.manager.NewRef(falsity)
					if cause != nil {
						return lbool.Undef, cause
					}
					owner.refs = append(owner.refs, ref)
					lowerFormulas[i] = ref
				}
			} else {
				if err = s.manager.DelRef(&blocker); err != nil {
					return lbool.Undef, err
				}
				formula, cause := s.manager.RefGet(lowerFormulas[i])
				if cause != nil {
					return lbool.Undef, cause
				}
				blockers[i] = formula
				continue
			}
			blockers[i], err = s.manager.RefGet(lowerFormulas[i])
			if err != nil {
				return lbool.Undef, err
			}
			result.Values[i].Arithmetic = s.orient(i, value)
		}
		increase := false
		for i := range lower {
			if !s.objectives[i].integer || lower[i].Infinity.Sign() != 0 {
				continue
			}
			var difference simplex.Infinitesimal
			difference.Set(&lower[i])
			difference.Sub(&previous[i])
			if difference.Sign() > 0 {
				increase = true
				break
			}
		}
		if !increase {
			delta.SetInt64(1)
		} else if *steps > *stepIncs {
			delta.Lsh(delta, 1)
			*stepIncs++
			*steps = 0
		} else {
			*steps++
		}
		if delta.Cmp(big.NewInt(1)) > 0 {
			if err = s.backend.Push(); err != nil {
				return lbool.Undef, err
			}
			*deltaScopes++
			if len(lower) == 0 {
				return lbool.Undef, ast.ErrASTDomain
			}
			jump := lower[0].Clone()
			jump.Main.Add(&jump.Main, new(big.Rat).SetInt(delta))
			bound, cause := s.backend.MakeObjectiveLowerBound(s.objectives[0].backendIndex, s.filter, jump)
			if cause != nil {
				return lbool.Undef, cause
			}
			if err = s.backend.Assert(bound); err != nil {
				return lbool.Undef, err
			}
		}
		bound, err := s.manager.MkOrArray(blockers)
		if err != nil {
			return lbool.Undef, err
		}
		if err = s.backend.Assert(bound); err != nil {
			return lbool.Undef, err
		}
	}
}

func (o *maxSMTASTOwner) take(ref ast.ASTRef) error {
	if ref == (ast.ASTRef{}) {
		return ast.ErrASTHandle
	}
	o.refs = append(o.refs, ref)
	return nil
}
