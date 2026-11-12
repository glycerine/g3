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
		if resultErr == nil && result != nil && !s.rawModels {
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
	if s.optsmtEngine != "farkas" && s.optsmtEngine != "symba" {
		// optsmt::box: geometric_opt runs inside the scoped push, starting
		// from the m_lower/m_lower_fmls that update_lower left.
		return s.geometricBox(result)
	}
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
	// geometric_opt initializes delta_index to zero. get_max_delta in the
	// source reports whether an integral objective advanced but does not assign
	// the index reference, so the jump bound remains attached to objective 0.
	deltaIndex := uint32(0)
	var steps, stepIncs uint32
	farkas := s.optsmtEngine == "farkas"
	if farkas {
		status, err = s.farkasBoxSearch(result, &owner, lower, lowerFormulas)
	} else if s.optsmtEngine == "symba" {
		status, err = s.symbaBoxSearch(&owner, lowerFormulas)
	}
	if err == nil && status == lbool.True && !farkas {
		status, err = s.geometricBoxSearch(result, &owner, lower, lowerFormulas, deltaIndex, &delta, &steps, &stepIncs, &deltaScopes)
	}
	if err != nil {
		return nil, err
	}
	if status == lbool.Undef {
		result.ReasonUnknown, err = s.backend.ReasonUnknown()
		result.Status = status
		return result, err
	}
	if status != lbool.False && !(farkas && status == lbool.True) {
		result.Status = status
		return result, nil
	}
	result.Status, result.Optimal = lbool.True, true
	if farkas {
		if len(result.ObjectiveModels) != len(s.objectives) {
			return nil, ast.ErrASTHandle
		}
	} else {
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
		}
	}
	for i := range result.ObjectiveModels {
		if err = s.captureArithmeticModel(i, &result.Values[i], result.ObjectiveModels[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
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

// symba_opt first asks whether any objective still reaches its current upper
// endpoint. optsmt::setup initializes those endpoints to +infinity, while the
// live optimizer context only updates lower endpoints before box execution;
// therefore every initial endpoint atom is false and the guarded probe is
// unsatisfiable. The source then asserts the disjunction of the initial true
// lower formulas and enters geometric_opt.
func (s *ArithmeticSolver) symbaBoxSearch(owner *maxSMTASTOwner, lowerFormulas []ast.ASTRef) (lbool.LBool, error) {
	tors := make([]ast.ASTHandle, len(s.objectives))
	for i := range tors {
		var err error
		tors[i], err = s.manager.MkFalse()
		if err != nil {
			return lbool.Undef, err
		}
	}
	upperDisjunction, err := s.manager.MkOrArray(tors)
	if err != nil {
		return lbool.Undef, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return lbool.Undef, err
	}
	probe, err := owner.hold(s.manager.MkFreshConst("b", boolean))
	if err != nil {
		return lbool.Undef, err
	}
	implication, err := owner.hold(s.manager.MkImplies(probe, upperDisjunction))
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.backend.Assert(implication); err != nil {
		return lbool.Undef, err
	}
	status, err := s.backend.CheckSat([]ast.ASTHandle{probe})
	if err != nil || status != lbool.False {
		return status, err
	}
	lower := make([]ast.ASTHandle, len(lowerFormulas))
	for i := range lowerFormulas {
		lower[i], err = s.manager.RefGet(lowerFormulas[i])
		if err != nil {
			return lbool.Undef, err
		}
	}
	bound, err := s.manager.MkOrArray(lower)
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.backend.Assert(bound); err != nil {
		return lbool.Undef, err
	}
	return lbool.True, nil
}

// farkasBoxSearch ports optsmt::farkas_opt and update_upper. Each objective
// bound is internalized before the assumption checks begin, matching the
// source rule that all bound atoms must exist at base scope.
func (s *ArithmeticSolver) farkasBoxSearch(result *ArithmeticResult, owner *maxSMTASTOwner, lower []simplex.Infinitesimal, lowerFormulas []ast.ASTRef) (lbool.LBool, error) {
	result.ObjectiveModels = make([]*model.ModelCore, len(s.objectives))
	for _, objective := range s.objectives {
		supported, err := s.backend.ObjectiveSupportsConflictRecording(objective.backendIndex)
		if err != nil {
			return lbool.Undef, err
		}
		if !supported {
			return lbool.Undef, nil
		}
	}
	upper := make([]simplex.Infinitesimal, len(s.objectives))
	for i := range upper {
		upper[i].Infinity.SetInt64(1)
	}
	for {
		status, err := s.updateFarkasUpper(result, owner, lower, upper, lowerFormulas)
		if err != nil {
			return lbool.Undef, err
		}
		if status == lbool.True {
			continue
		}
		if status == lbool.Undef {
			return lbool.Undef, nil
		}
		for i := range upper {
			upper[i] = lower[i].Clone()
		}
		return lbool.True, nil
	}
}

func (s *ArithmeticSolver) updateFarkasUpper(result *ArithmeticResult, owner *maxSMTASTOwner, lower, upper []simplex.Infinitesimal, lowerFormulas []ast.ASTRef) (status lbool.LBool, resultErr error) {
	if len(lower) != len(s.objectives) || len(upper) != len(lower) || len(lowerFormulas) != len(lower) {
		return lbool.Undef, ast.ErrASTDomain
	}
	if err := s.backend.Push(); err != nil {
		return lbool.Undef, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, s.backend.Pop(1))
		if resultErr != nil {
			status = lbool.Undef
		}
	}()
	mids := make([]simplex.Infinitesimal, len(lower))
	bounds := make([]ast.ASTHandle, len(lower))
	active := make([]bool, len(lower))
	for i := range lower {
		if lower[i].Cmp(&upper[i]) >= 0 {
			continue
		}
		mids[i] = upper[i].Clone()
		mids[i].Add(&lower[i])
		mids[i].Quo(big.NewRat(2, 1))
		bound, err := s.backend.MakeObjectiveLowerBound(s.objectives[i].backendIndex, s.filter, mids[i])
		if err != nil {
			return lbool.Undef, err
		}
		bounds[i], err = owner.hold(bound, nil)
		if err != nil {
			return lbool.Undef, err
		}
		active[i] = true
	}
	progress := false
	for i := range lower {
		if !active[i] || mids[i].Cmp(&lower[i]) < 0 || mids[i].Cmp(&upper[i]) > 0 {
			continue
		}
		if err := s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, bounds[i]); err != nil {
			return lbool.Undef, err
		}
		status, err := s.backend.CheckSat([]ast.ASTHandle{bounds[i]})
		if err != nil {
			err = errors.Join(err, s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}))
			return lbool.Undef, err
		}
		switch status {
		case lbool.True:
			lower[i] = mids[i].Clone()
			if err = s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}); err != nil {
				return lbool.Undef, err
			}
			if err = s.updateFarkasLower(result, owner, lower, lowerFormulas); err != nil {
				return lbool.Undef, err
			}
		case lbool.False:
			bound, finite, cause := s.backend.ObjectiveConflictMinimize(s.objectives[i].backendIndex)
			if cause != nil {
				return lbool.Undef, cause
			}
			if !finite {
				upper[i] = lower[i].Clone()
				return lbool.False, nil
			}
			if bound.Cmp(&upper[i]) < 0 {
				upper[i] = bound
			}
		case lbool.Undef:
			if err = s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}); err != nil {
				return lbool.Undef, err
			}
			return lbool.Undef, nil
		}
		if err = s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}); err != nil {
			return lbool.Undef, err
		}
		progress = true
	}
	if !progress {
		return lbool.False, nil
	}
	return lbool.True, nil
}

func (s *ArithmeticSolver) updateFarkasLower(result *ArithmeticResult, owner *maxSMTASTOwner, lower []simplex.Infinitesimal, lowerFormulas []ast.ASTRef) error {
	if err := s.replaceModel(result); err != nil {
		return err
	}
	blockers := make([]ast.ASTHandle, len(lower))
	for i := range lower {
		value, blocker, err := s.backend.MaximizeOptimizationObjective(s.objectives[i].backendIndex, s.filter)
		if err != nil {
			if blocker != (ast.ASTRef{}) {
				err = errors.Join(err, s.manager.DelRef(&blocker))
			}
			return err
		}
		candidate := value.Finite.Clone()
		if value.Unbounded {
			candidate.Infinity.SetInt64(1)
		}
		if candidate.Cmp(&lower[i]) >= 0 {
			lower[i] = candidate
			if err = owner.take(blocker); err != nil {
				return err
			}
			lowerFormulas[i] = owner.refs[len(owner.refs)-1]
			if candidate.Infinity.Sign() > 0 {
				falsity, cause := s.manager.MkFalse()
				if cause != nil {
					return cause
				}
				ref, cause := s.manager.NewRef(falsity)
				if cause != nil {
					return cause
				}
				owner.refs = append(owner.refs, ref)
				lowerFormulas[i] = ref
			}
			result.Values[i].Arithmetic = s.orient(i, value)
			modelSnapshot, cause := s.backend.AcquireSavedObjectiveModel(s.objectives[i].backendIndex)
			if cause != nil {
				return cause
			}
			if modelSnapshot == nil {
				return ast.ErrASTHandle
			}
			if old := result.ObjectiveModels[i]; old != nil {
				if cause = old.DecRef(); cause != nil {
					return errors.Join(cause, modelSnapshot.DecRef())
				}
			}
			result.ObjectiveModels[i] = modelSnapshot
		}
		blockers[i], err = s.manager.RefGet(lowerFormulas[i])
		if err != nil {
			return err
		}
	}
	disjunction, err := s.manager.MkOrArray(blockers)
	if err != nil {
		return err
	}
	return s.backend.Assert(disjunction)
}

func (s *ArithmeticSolver) geometricBoxSearch(result *ArithmeticResult, owner *maxSMTASTOwner, lower []simplex.Infinitesimal, lowerFormulas []ast.ASTRef, deltaIndex uint32, delta *big.Int, steps, stepIncs *uint32, deltaScopes *uint32) (lbool.LBool, error) {
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
			if int(deltaIndex) >= len(lower) {
				return lbool.Undef, ast.ErrASTDomain
			}
			jump := lower[deltaIndex].Clone()
			jump.Main.Add(&jump.Main, new(big.Rat).SetInt(delta))
			bound, cause := s.backend.MakeObjectiveLowerBound(s.objectives[deltaIndex].backendIndex, s.filter, jump)
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
	// optsmt::m_models: the opt_solver model taken when set_max accepts.
	models := make([]*model.ModelCore, n)
	defer func() {
		for _, md := range models {
			if md != nil {
				resultErr = errors.Join(resultErr, md.DecRef())
			}
		}
	}()
	var delta big.Int
	delta.SetInt64(1)
	one := big.NewInt(1)
	var steps, stepIncs, numScopes uint32
	deltaIndex := 0
	status := lbool.True
	for {
		canceled, err := s.manager.Canceled()
		if err != nil {
			return nil, err
		}
		if canceled {
			break
		}
		status, err = s.backend.CheckSat(nil)
		if err != nil {
			return nil, err
		}
		if status == lbool.True {
			if err = s.replaceModel(result); err != nil {
				return nil, err
			}
			held, err := s.boxUpdateLower(models)
			if err != nil {
				return nil, err
			}
			// bound = update_lower(): the old bound goes after disj.
			if bound != (ast.ASTRef{}) {
				if err = s.manager.DelRef(&bound); err != nil {
					return nil, errors.Join(err, s.manager.DelRef(&held))
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
					return nil, err
				}
				numScopes++
				// only try to improve delta_index.
				i := s.boxOrigin(deltaIndex)
				current := s.optLowerOf(i)
				jump := current.Clone()
				jump.Main.Add(&jump.Main, new(big.Rat).SetInt(&delta))
				ge, err := s.backend.OptimizationObjectiveGE(s.objectives[deltaIndex].backendIndex, s.filter, jump)
				if err != nil {
					return nil, err
				}
				if err = replaceBound(ge); err != nil {
					return nil, err
				}
			}
			h, err := s.manager.RefGet(bound)
			if err != nil {
				return nil, err
			}
			if err = s.backend.Assert(h); err != nil {
				return nil, err
			}
		} else if status == lbool.False && delta.Cmp(one) > 0 {
			steps, stepIncs = 0, 0
			delta.SetInt64(1)
			numScopes--
			if err = s.backend.Pop(1); err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	if numScopes != 0 {
		if err := s.backend.Pop(numScopes); err != nil {
			return nil, err
		}
	}
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
		md := models[k]
		models[k] = nil
		if md == nil {
			// update_lower's initial bound came without an optsmt model;
			// keep the opt_solver's saved one.
			if md, err = s.backend.AcquireSavedObjectiveModel(s.objectives[k].backendIndex); err != nil {
				return nil, err
			}
			if md == nil {
				return nil, ast.ErrASTHandle
			}
		}
		result.ObjectiveModels[k] = md
		current := s.optLowerOf(s.boxOrigin(k))
		value := smtcore.SMTArithmeticObjectiveValue{Finite: current.Clone()}
		value.Unbounded = value.Finite.Infinity.Sign() > 0
		result.Values[k].Arithmetic = s.orient(k, value)
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
func (s *ArithmeticSolver) boxUpdateLower(models []*model.ModelCore) (_ ast.ASTRef, resultErr error) {
	n := len(s.objectives)
	// expr_ref_vector disj(m)
	disj := make([]ast.ASTRef, 0, n)
	defer func() {
		for i := range disj {
			resultErr = errors.Join(resultErr, s.manager.DelRef(&disj[i]))
		}
	}()
	values := make([]smtcore.SMTArithmeticObjectiveValue, n)
	// opt_solver::maximize_objectives: one expr_ref blocker shared by
	// every maximize_objective call.
	err := func() (err error) {
		var blocker ast.ASTRef
		defer func() {
			if blocker != (ast.ASTRef{}) {
				err = errors.Join(err, s.manager.DelRef(&blocker))
			}
		}()
		for k := range s.objectives {
			if values[k], err = s.backend.MaximizeOptimizationObjectiveInto(s.objectives[k].backendIndex, s.filter, &blocker); err != nil {
				return err
			}
			h, err := s.manager.RefGet(blocker)
			if err != nil {
				return err
			}
			ref, err := s.manager.NewRef(h)
			if err != nil {
				return err
			}
			disj = append(disj, ref)
		}
		return nil
	}()
	if err != nil {
		return ast.ASTRef{}, err
	}
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
				return ast.ASTRef{}, err
			}
			if models[k] != nil {
				err = models[k].DecRef()
			}
			models[k] = md
			if err != nil {
				return ast.ASTRef{}, err
			}
			h, err := s.manager.RefGet(disj[k])
			if err != nil {
				return ast.ASTRef{}, err
			}
			if err = s.setLowerFormula(i, h); err != nil {
				return ast.ASTRef{}, err
			}
			if src.Infinity.Sign() > 0 {
				falsity, err := s.manager.MkFalse()
				if err != nil {
					return ast.ASTRef{}, err
				}
				if err = s.setLowerFormula(i, falsity); err != nil {
					return ast.ASTRef{}, err
				}
				if err = setDisj(k, falsity); err != nil {
					return ast.ASTRef{}, err
				}
			}
			continue
		}
		f, err := s.lowerFormula(i)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if !s.manager.IsTrue(f) {
			if err = setDisj(k, f); err != nil {
				return ast.ASTRef{}, err
			}
		}
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
