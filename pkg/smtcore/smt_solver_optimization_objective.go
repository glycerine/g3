package smtcore

// Source: src/opt/opt_solver.cpp:226-286,342-363.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// cloneOptimizationValue is the inf_eps copy constructor: a fresh value of
// q the caller owns.
func cloneOptimizationValue(q *numeral.RationalManager, v *SMTArithmeticObjectiveValue) (SMTArithmeticObjectiveValue, error) {
	finite, err := v.Finite.Clone(q)
	return SMTArithmeticObjectiveValue{Unbounded: v.Unbounded, Finite: finite}, err
}

// equalOptimizationValues compares two values as inf_eps (Unbounded is
// infinity 1); the copies are temporaries.
func equalOptimizationValues(q *numeral.RationalManager, a, b *SMTArithmeticObjectiveValue) (equal bool, err error) {
	left, err := a.Finite.Clone(q)
	defer func() { err = smtErrJoin(err, left.Del(q)) }()
	if err != nil {
		return false, err
	}
	right, err := b.Finite.Clone(q)
	defer func() { err = smtErrJoin(err, right.Del(q)) }()
	if err != nil {
		return false, err
	}
	if a.Unbounded {
		if err = q.SetInt64(&left.Infinity, 1); err != nil {
			return false, err
		}
	}
	if b.Unbounded {
		if err = q.SetInt64(&right.Infinity, 1); err != nil {
			return false, err
		}
	}
	c, err := left.Cmp(q, &right)
	return c == 0, err
}

func (s *SMTSolver) saveOptimizationModel(index uint32) error {
	md, err := s.AcquireModel()
	if err != nil {
		return err
	}
	old := s.optimization.models[index]
	if old != nil {
		if err = old.DecRef(); err != nil {
			if md != nil {
				err = errors.Join(err, md.DecRef())
			}
			return err
		}
	}
	s.optimization.models[index] = md
	return nil
}

// OptimizationObjectiveGE is opt_solver::mk_ge(var, val): false or true for
// an infinite val, otherwise the theory's bound atom. value is borrowed.
func (s *SMTSolver) OptimizationObjectiveGE(index uint32, filter *tactic.FilterModelConverter, value simplex.Infinitesimal) (ast.ASTHandle, error) {
	return s.optimizationObjectiveGE(index, filter, &SMTArithmeticObjectiveValue{Finite: value})
}

func (s *SMTSolver) optimizationObjectiveGE(index uint32, filter *tactic.FilterModelConverter, value *SMTArithmeticObjectiveValue) (ast.ASTHandle, error) {
	q := s.manager.NumeralService().Manager
	infinity, err := q.Sign(&value.Finite.Infinity)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if value.Unbounded || infinity > 0 {
		return s.manager.MkFalse()
	}
	if infinity < 0 {
		return s.manager.MkTrue()
	}
	objective, err := s.objective(index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	epsilon, err := q.Sign(&value.Finite.Epsilon)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if epsilon != 0 {
		switch objective.theory.(type) {
		case *SMTRealDiffTheory, *SMTDenseDiffTheory:
			// opt_solver::mk_ge supports these concrete difference families
			// only at rational thresholds. Its fallback deliberately imposes
			// no constraint; do not call their lower-bound builders here.
			enabled, err := diagnostic.ProcessVerboseEnabled(0)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if enabled {
				stream, err := diagnostic.BorrowProcessVerboseStream()
				if err != nil {
					return ast.ASTHandle{}, err
				}
				_ = stream.AppendString("WARNING: unhandled theory " + fmt.Sprintf("%T", objective.theory) + "\n")
			}
			return s.manager.MkTrue()
		}
	}
	return s.MakeObjectiveLowerBound(index, filter, value.Finite)
}

func (s *SMTSolver) decrementOptimizationValue(index uint32, filter *tactic.FilterModelConverter, value *SMTArithmeticObjectiveValue) (status lbool.LBool, resultErr error) {
	if err := s.PushCore(); err != nil {
		return lbool.Undef, err
	}
	pushed := true
	defer func() {
		if pushed {
			resultErr = errors.Join(resultErr, s.PopCore(1))
		}
	}()
	bound, err := s.optimizationObjectiveGE(index, filter, value)
	if err != nil {
		return lbool.Undef, err
	}
	root, err := s.manager.NewRef(bound)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&root)) }()
	if err = s.Assert(bound); err != nil {
		return lbool.Undef, err
	}
	// Source bypasses opt_solver::check_sat_core here: no dump, trace or
	// first-check transition, but quantifier uncertainty is still adjusted.
	status, err = s.kernel.CheckAssumptions(nil)
	if err != nil {
		return lbool.Undef, err
	}
	status = s.adjustOptimizationResult(status)
	if status == lbool.True {
		if s.kernel.GeneratedModel() == nil && s.produceModels {
			if err = s.kernel.buildGeneratedModel(); err != nil {
				return lbool.Undef, err
			}
		}
		if err = s.saveOptimizationModel(index); err != nil {
			return lbool.Undef, err
		}
	}
	if err = s.PopCore(1); err != nil {
		return lbool.Undef, err
	}
	pushed = false
	if status != lbool.True {
		term, err := s.manager.RefGet(s.objectives[index].term)
		if err != nil {
			return status, err
		}
		sort, err := s.manager.GetSort(term)
		if err != nil {
			return status, err
		}
		kind, err := s.manager.GetDeclKind(sort)
		if err != nil {
			return status, err
		}
		q := s.manager.NumeralService().Manager
		if kind == 0 {
			err = q.Dec(&value.Finite.Epsilon)
		} else {
			err = q.Dec(&value.Finite.Main)
		}
		if err != nil {
			return status, err
		}
		s.objectives[index].valid = false
	}
	return status, nil
}

// MaximizeOptimizationObjective implements the opt_solver wrapper around the
// raw theory maximizer. Its returned blocker reference belongs to the caller.
func (s *SMTSolver) MaximizeOptimizationObjective(index uint32, filter *tactic.FilterModelConverter) (value SMTArithmeticObjectiveValue, blocker ast.ASTRef, resultErr error) {
	value, resultErr = s.MaximizeOptimizationObjectiveInto(index, filter, &blocker)
	if resultErr != nil && blocker != (ast.ASTRef{}) {
		resultErr = errors.Join(resultErr, s.manager.DelRef(&blocker))
	}
	return value, blocker, resultErr
}

// MaximizeOptimizationObjectiveInto is maximize_objective(i, expr_ref&
// blocker): the theory's new blocker overwrites *blocker (releasing the old
// one) before the model update and decrement_value run. *blocker stays owned
// by the caller, also on error. The returned value belongs to the caller
// (rationals of the AST manager's rational manager); on error it is 0.
func (s *SMTSolver) MaximizeOptimizationObjectiveInto(index uint32, filter *tactic.FilterModelConverter, blocker *ast.ASTRef) (value SMTArithmeticObjectiveValue, resultErr error) {
	value.Finite = simplex.ZeroInfinitesimal()
	if blocker == nil {
		return value, ast.ErrASTHandle
	}
	q := s.manager.NumeralService().Manager
	defer func() {
		if resultErr != nil {
			resultErr = smtErrJoin(resultErr, value.Del(q))
		}
	}()
	objective, err := s.objective(index)
	if err != nil {
		return value, err
	}
	if s.optimization == nil || filter == nil || !s.produceModels {
		return value, ast.ErrASTDomain
	}
	raw, condition, shared, err := objective.theory.MaximizeObjective(objective.variable)
	// inf_eps val = ...maximize(...): the theory's value becomes this value.
	value = raw
	if err != nil {
		return value, err
	}
	root, err := s.manager.NewRef(condition)
	if err != nil {
		return value, err
	}
	if *blocker != (ast.ASTRef{}) {
		if err = s.manager.DelRef(blocker); err != nil {
			return value, errors.Join(err, s.manager.DelRef(&root))
		}
	}
	*blocker = root
	md, err := s.AcquireModel()
	if err != nil {
		return value, err
	}
	if md != nil {
		if err = md.DecRef(); err != nil {
			return value, err
		}
	}
	objective.valid = true
	if s.optimization.models[index] == nil {
		if err = s.saveOptimizationModel(index); err != nil {
			return value, err
		}
	}
	updated, err := s.UpdateModel(shared)
	if err != nil {
		return value, err
	}
	repair := !updated
	if updated && shared {
		current, err := s.ObjectiveValue(index)
		equal := false
		if err == nil {
			equal, err = equalOptimizationValues(q, &value, &current)
		}
		if err = smtErrJoin(err, current.Del(q)); err != nil {
			return value, err
		}
		repair = !equal
	}
	if repair {
		if _, err = s.decrementOptimizationValue(index, filter, &value); err != nil {
			return value, err
		}
	} else if err = s.saveOptimizationModel(index); err != nil {
		return value, err
	}
	saved, err := cloneOptimizationValue(q, &value)
	if err = smtErrJoin(err, s.objectives[index].saved.Del(q)); err != nil {
		return value, smtErrJoin(err, saved.Del(q))
	}
	s.objectives[index].saved = saved
	return value, nil
}

func (s *SMTSolver) OptimizationObjectiveModelValid(index uint32) (bool, error) {
	objective, err := s.objective(index)
	if err != nil {
		return false, err
	}
	return objective.valid, nil
}

func (s *SMTSolver) SavedObjectiveValue(index uint32) (SMTArithmeticObjectiveValue, error) {
	objective, err := s.objective(index)
	if err != nil {
		return SMTArithmeticObjectiveValue{Finite: simplex.ZeroInfinitesimal()}, err
	}
	return cloneOptimizationValue(s.manager.NumeralService().Manager, &objective.saved)
}

// AcquireSavedObjectiveModel returns an independently owned reference to the
// model snapshot saved when this objective was last maximized. Box optimization
// keeps one best model per objective while continuing the joint search.
func (s *SMTSolver) AcquireSavedObjectiveModel(index uint32) (*model.ModelCore, error) {
	if _, err := s.objective(index); err != nil {
		return nil, err
	}
	if s.optimization == nil || uint64(index) >= uint64(len(s.optimization.models)) {
		return nil, ast.ErrASTDomain
	}
	md := s.optimization.models[index]
	if md == nil {
		return nil, nil
	}
	if err := md.IncRef(); err != nil {
		return nil, err
	}
	return md, nil
}
