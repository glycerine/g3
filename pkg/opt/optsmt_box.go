package opt

// Source: src/opt/optsmt.cpp symba_opt, geometric_opt, update_lower and set_max.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
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
	defer func() {
		if resultErr == nil && result != nil && result.Status == lbool.True {
			var unknown bool
			unknown, resultErr = s.backend.OptimizationWasUnknown()
			if resultErr == nil && unknown {
				result.Status, result.Optimal = lbool.Undef, false
				result.ReasonUnknown, resultErr = s.backend.ReasonUnknown()
			}
		}
		if resultErr == nil && result != nil && !s.rawModels {
			resultErr = s.filterResultModel(result)
			for i, snapshot := range result.ObjectiveModels {
				if resultErr != nil {
					break
				}
				if snapshot == nil {
					continue
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
	// optsmt::box: the engine runs inside the scoped push, starting from
	// the m_lower/m_lower_fmls that update_lower left.
	switch s.optsmtEngine {
	case "farkas":
		return s.farkasBox(result)
	case "symba":
		return s.symbaBox(result)
	}
	return s.geometricBox(result)
}

func readOptSMTEngine(m *ast.ASTManager, local *uparams.ParamsRef, config smtcore.SMTSolverConfig) (engine string, resultErr error) {
	globals := config.Globals
	if globals == nil {
		globals = config.ModelEvaluator.Globals
	}
	if globals == nil {
		globals = uparams.BorrowProcessGlobalParamsService()
	}
	if globals == nil || config.Params == nil {
		return "basic", nil
	}
	params, err := NewParams(config.Params, globals, local)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, params.Close()) }()
	value, err := params.OptSMTEngine()
	if err != nil {
		return "", err
	}
	return m.Symbols.String(value)
}

// boxOrigin maps a box objective to its index in the solver that built the
// box (optsmt's objective index for m_lower and m_lower_fmls).
func (s *ArithmeticSolver) boxOrigin(k int) int {
	if s.boxIndices == nil {
		return k
	}
	return s.boxIndices[k]
}

// lowerFormula is m_lower_fmls[i]; optsmt::add initializes it to true.
func (s *ArithmeticSolver) lowerFormula(i int) (ast.ASTHandle, error) {
	if i < len(s.lowerFmls) && s.lowerFmls[i] != (ast.ASTRef{}) {
		return s.manager.RefGet(s.lowerFmls[i])
	}
	return s.manager.MkTrue()
}

// setLowerFormula is m_lower_fmls[i] = f.
func (s *ArithmeticSolver) setLowerFormula(i int, f ast.ASTHandle) error {
	held, err := s.manager.NewRef(f)
	if err != nil {
		return err
	}
	for len(s.lowerFmls) <= i {
		s.lowerFmls = append(s.lowerFmls, ast.ASTRef{})
	}
	if s.lowerFmls[i] != (ast.ASTRef{}) {
		if err = s.manager.DelRef(&s.lowerFmls[i]); err != nil {
			return errors.Join(err, s.manager.DelRef(&held))
		}
	}
	s.lowerFmls[i] = held
	return nil
}

// geometricBox is optsmt::geometric_opt for box(). lower is the entry copy
// of m_lower that get_max_delta measures progress against.
func (s *ArithmeticSolver) geometricBox(result *ArithmeticResult) (_ *ArithmeticResult, resultErr error) {
	models, release := s.boxModels()
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	status, err := s.geometricOpt(result, models)
	if err != nil {
		return nil, err
	}
	return s.finishBox(result, models, status)
}

// geometricOpt is optsmt::geometric_opt; its bound local dies with it.
func (s *ArithmeticSolver) geometricOpt(result *ArithmeticResult, models *OptsmtModels) (_ lbool.LBool, resultErr error) {
	n := len(s.objectives)
	lower := make([]simplex.Infinitesimal, n)
	for k := range lower {
		current := s.optLowerOf(s.boxOrigin(k))
		lower[k] = current.Clone()
	}
	// expr_ref bound(m)
	var bound ast.ASTRef
	defer func() {
		if bound != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, s.manager.DelRef(&bound))
		}
	}()
	replaceBound := func(h ast.ASTHandle) error {
		held, err := s.manager.NewRef(h)
		if err != nil {
			return err
		}
		if bound != (ast.ASTRef{}) {
			if err = s.manager.DelRef(&bound); err != nil {
				return errors.Join(err, s.manager.DelRef(&held))
			}
		}
		bound = held
		return nil
	}
	var delta big.Int
	delta.SetInt64(1)
	one := big.NewInt(1)
	var steps, stepIncs, numScopes uint32
	deltaIndex := 0
	status := lbool.True
	for {
		canceled, err := s.manager.Canceled()
		if err != nil {
			return lbool.Undef, err
		}
		if canceled {
			break
		}
		status, err = s.backend.CheckSat(nil)
		if err != nil {
			return lbool.Undef, err
		}
		if status == lbool.True {
			if err = s.replaceModel(result); err != nil {
				return lbool.Undef, err
			}
			held, err := s.boxUpdateLower(models)
			if err != nil {
				return lbool.Undef, err
			}
			// bound = update_lower(): the old bound goes after disj.
			if bound != (ast.ASTRef{}) {
				if err = s.manager.DelRef(&bound); err != nil {
					return lbool.Undef, errors.Join(err, s.manager.DelRef(&held))
				}
			}
			bound = held
			if !s.boxMaxDelta(lower) {
				delta.SetInt64(1)
			} else if steps > stepIncs {
				delta.Lsh(&delta, 1)
				stepIncs++
				steps = 0
			} else {
				steps++
			}
			if delta.Cmp(one) > 0 {
				if err = s.backend.Push(); err != nil {
					return lbool.Undef, err
				}
				numScopes++
				// only try to improve delta_index.
				i := s.boxOrigin(deltaIndex)
				current := s.optLowerOf(i)
				jump := current.Clone()
				jump.Main.Add(&jump.Main, new(big.Rat).SetInt(&delta))
				ge, err := s.backend.OptimizationObjectiveGE(s.objectives[deltaIndex].backendIndex, s.filter, jump)
				if err != nil {
					return lbool.Undef, err
				}
				if err = replaceBound(ge); err != nil {
					return lbool.Undef, err
				}
			}
			h, err := s.manager.RefGet(bound)
			if err != nil {
				return lbool.Undef, err
			}
			if err = s.backend.Assert(h); err != nil {
				return lbool.Undef, err
			}
		} else if status == lbool.False && delta.Cmp(one) > 0 {
			steps, stepIncs = 0, 0
			delta.SetInt64(1)
			numScopes--
			if err = s.backend.Pop(1); err != nil {
				return lbool.Undef, err
			}
		} else {
			break
		}
	}
	if numScopes != 0 {
		if err := s.backend.Pop(numScopes); err != nil {
			return lbool.Undef, err
		}
	}
	return status, nil
}

// boxModels is optsmt::m_models: the opt_solver model taken when set_max
// accepts. opt::context owns it; optsmt::reset leaves it, so entries
// persist. Without a context a local one lives for the call.
func (s *ArithmeticSolver) boxModels() (*OptsmtModels, func() error) {
	if s.optsmtModels != nil {
		return s.optsmtModels, func() error { return nil }
	}
	models := &OptsmtModels{}
	return models, models.Close
}

// finishBox is the engine's exit: l_undef when canceled or undecided;
// otherwise "set the solution tight" (m_upper = m_lower), and execute_box
// collects m_optsmt.get_model(j) with the objective values.
func (s *ArithmeticSolver) finishBox(result *ArithmeticResult, models *OptsmtModels, status lbool.LBool) (*ArithmeticResult, error) {
	n := len(s.objectives)
	canceled, err := s.manager.Canceled()
	if err != nil {
		return nil, err
	}
	if canceled || status == lbool.Undef {
		result.Status = lbool.Undef
		result.ReasonUnknown, err = s.backend.ReasonUnknown()
		return result, err
	}
	// set the solution tight.
	for k := range s.objectives {
		i := s.boxOrigin(k)
		s.setOptUpper(i, s.optLowerOf(i))
	}
	result.Status, result.Optimal = lbool.True, true
	result.ObjectiveModels = make([]*model.ModelCore, n)
	for k := range s.objectives {
		// m_box_models.push_back(m_optsmt.get_model(j))
		md, err := models.get(k)
		if err != nil {
			return nil, err
		}
		// An entry set_max never filled stays null (farkas_opt without a
		// satisfiable midpoint); m_box_models then holds nullptr.
		result.ObjectiveModels[k] = md
		current := s.optLowerOf(s.boxOrigin(k))
		value := smtcore.SMTArithmeticObjectiveValue{Finite: current.Clone()}
		value.Unbounded = value.Finite.Infinity.Sign() > 0
		result.Values[k].Arithmetic = s.orient(k, value)
		if md == nil {
			continue
		}
		if err = s.captureArithmeticModel(k, &result.Values[k], md); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// boxMaxDelta is optsmt::get_max_delta: whether an integer objective's
// finite m_lower moved past the entry value lower.
func (s *ArithmeticSolver) boxMaxDelta(lower []simplex.Infinitesimal) bool {
	var maxDelta simplex.Infinitesimal
	for k := range s.objectives {
		if !s.objectives[k].integer {
			continue
		}
		current := s.optLowerOf(s.boxOrigin(k))
		if current.Infinity.Sign() != 0 {
			continue
		}
		var d simplex.Infinitesimal
		d.Set(&current)
		d.Sub(&lower[k])
		if d.Cmp(&maxDelta) > 0 {
			maxDelta = d
		}
	}
	return maxDelta.Sign() > 0
}

// boxUpdateLower is optsmt::update_lower: maximize_objectives(disj), then
// set_max(m_lower, get_objective_values(), disj); it returns mk_or(disj).
func (s *ArithmeticSolver) boxUpdateLower(models *OptsmtModels) (_ ast.ASTRef, resultErr error) {
	n := len(s.objectives)
	// expr_ref_vector disj(m)
	disj := make([]ast.ASTRef, 0, n)
	defer func() {
		for i := range disj {
			resultErr = errors.Join(resultErr, s.manager.DelRef(&disj[i]))
		}
	}()
	values, err := s.boxMaximizeObjectives(&disj)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = s.boxSetMax(models, values, disj); err != nil {
		return ast.ASTRef{}, err
	}
	args := make([]ast.ASTHandle, n)
	for k := range disj {
		if args[k], err = s.manager.RefGet(disj[k]); err != nil {
			return ast.ASTRef{}, err
		}
	}
	or, err := s.manager.MkOrArray(args)
	if err != nil {
		return ast.ASTRef{}, err
	}
	// The returned expr_ref holds mk_or(disj) while disj goes away.
	return s.manager.NewRef(or)
}

// boxMaximizeObjectives is opt_solver::maximize_objectives(disj): one
// expr_ref blocker shared by every maximize_objective call, each blocker
// pushed onto disj. It returns get_objective_values().
func (s *ArithmeticSolver) boxMaximizeObjectives(disj *[]ast.ASTRef) (_ []smtcore.SMTArithmeticObjectiveValue, err error) {
	values := make([]smtcore.SMTArithmeticObjectiveValue, len(s.objectives))
	var blocker ast.ASTRef
	defer func() {
		if blocker != (ast.ASTRef{}) {
			err = errors.Join(err, s.manager.DelRef(&blocker))
		}
	}()
	for k := range s.objectives {
		if values[k], err = s.backend.MaximizeOptimizationObjectiveInto(s.objectives[k].backendIndex, s.filter, &blocker); err != nil {
			return nil, err
		}
		h, err := s.manager.RefGet(blocker)
		if err != nil {
			return nil, err
		}
		ref, err := s.manager.NewRef(h)
		if err != nil {
			return nil, err
		}
		*disj = append(*disj, ref)
	}
	return values, nil
}

// boxSetMax is set_max(m_lower, m_s->get_objective_values(), disj).
func (s *ArithmeticSolver) boxSetMax(models *OptsmtModels, values []smtcore.SMTArithmeticObjectiveValue, disj []ast.ASTRef) error {
	setDisj := func(k int, h ast.ASTHandle) error {
		ref, err := s.manager.NewRef(h)
		if err != nil {
			return err
		}
		err = s.manager.DelRef(&disj[k])
		disj[k] = ref
		return err
	}
	for k := range s.objectives {
		i := s.boxOrigin(k)
		src := savedValue(values[k])
		dst := s.optLowerOf(i)
		if src.Cmp(&dst) >= 0 {
			s.setOptLower(i, src)
			md, err := s.backend.AcquireSavedObjectiveModel(s.objectives[k].backendIndex)
			if err != nil {
				return err
			}
			if err = models.setOwned(k, md); err != nil {
				return err
			}
			h, err := s.manager.RefGet(disj[k])
			if err != nil {
				return err
			}
			if err = s.setLowerFormula(i, h); err != nil {
				return err
			}
			if src.Infinity.Sign() > 0 {
				falsity, err := s.manager.MkFalse()
				if err != nil {
					return err
				}
				if err = s.setLowerFormula(i, falsity); err != nil {
					return err
				}
				if err = setDisj(k, falsity); err != nil {
					return err
				}
			}
			continue
		}
		f, err := s.lowerFormula(i)
		if err != nil {
			return err
		}
		if !s.manager.IsTrue(f) {
			if err = setDisj(k, f); err != nil {
				return err
			}
		}
	}
	return nil
}
