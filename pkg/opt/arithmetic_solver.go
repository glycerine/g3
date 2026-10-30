package opt

// Source: src/opt/opt_solver.cpp and src/opt/optsmt.cpp retained arithmetic
// objective loop.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ObjectiveValue is one result in source lexicographic order. SoftCost is set
// for a MaxSMT objective; Arithmetic is set for maximize/minimize.
type ObjectiveValue struct {
	Arithmetic           smtcore.SMTArithmeticObjectiveValue
	ArithmeticModel      smtcore.SMTArithmeticObjectiveValue
	ArithmeticModelValid bool
	ArithmeticInterval   bool
	ArithmeticOpenBounds bool
	SoftCost             big.Rat
	SoftSatisfied        big.Rat
	SoftLower            big.Rat
	SoftUpper            big.Rat
	SoftAssignment       []bool
	Soft                 bool
}

// ArithmeticResult owns the best model retained by one optimization check.
// Values are expressed in the caller's original maximize/minimize orientation.
type ArithmeticResult struct {
	Status        lbool.LBool
	Values        []ObjectiveValue
	Model         *model.ModelCore
	Optimal       bool
	MaxSMT        []MaxSMTStatistics
	Labels        []usymbol.Symbol
	ModelFiltered bool
}

func (r *ArithmeticResult) Close() error {
	if r == nil || r.Model == nil {
		return nil
	}
	err := r.Model.DecRef()
	if err == nil {
		r.Model = nil
	}
	return err
}

// ArithmeticSolver is the first retained opt_solver path. It deliberately
// owns a dedicated SMT backend, as native optimization does, so temporary
// blockers cannot mutate the command context's ordinary incremental solver.
type ArithmeticSolver struct {
	backend        *smtcore.SMTSolver
	manager        *ast.ASTManager
	config         smtcore.SMTSolverConfig
	objectives     []arithmeticObjective
	filter         *tactic.FilterModelConverter
	satCoreEnabled bool
	prepared       bool
}

type arithmeticObjective struct {
	term         ast.ASTRef
	maximize     bool
	backendIndex uint32
	soft         bool
	softTerms    []ast.ASTRef
	softWeights  []*big.Rat
	softOffset   big.Rat
}

func NewArithmeticSolver(m *ast.ASTManager, params *uparams.ParamsRef, logic string, config smtcore.SMTSolverConfig) (*ArithmeticSolver, error) {
	backend, err := smtcore.NewSMTSolver(m, params, logic, config)
	if err != nil {
		return nil, err
	}
	if err = backend.EnsurePseudoBooleanTheory(); err != nil {
		return nil, errors.Join(err, backend.Close())
	}
	if err = backend.SetProduceModels(true); err != nil {
		return nil, errors.Join(err, backend.Close())
	}
	filter, err := tactic.NewFilterModel(m)
	if err != nil {
		return nil, errors.Join(err, backend.Close())
	}
	return &ArithmeticSolver{backend: backend, manager: m, config: config, filter: filter}, nil
}

func (s *ArithmeticSolver) Assert(formula ast.ASTHandle) error {
	if s == nil || s.backend == nil {
		return ast.ErrASTHandle
	}
	return s.backend.Assert(formula)
}

// SetSATCoreEnabled records the source optimizer's eligible incremental SAT
// path for MaxRes core handling. Call it before registering objectives.
func (s *ArithmeticSolver) SetSATCoreEnabled(enabled bool) error {
	if s == nil || s.backend == nil || len(s.objectives) != 0 {
		return ast.ErrASTHandle
	}
	s.satCoreEnabled = enabled
	return nil
}

// AddObjective registers one lexicographic objective. Minimization uses the
// source normalization to maximization of the negated term.
func (s *ArithmeticSolver) AddObjective(term ast.ASTHandle, maximize bool) error {
	if s == nil || s.backend == nil {
		return ast.ErrASTHandle
	}
	// optsmt::setup forces the solver to its base level with an empty scoped
	// push. Besides committing hard assertions, this installs the logic-selected
	// arithmetic backend before objective internalization.
	if !s.prepared {
		if err := s.backend.Push(); err != nil {
			return err
		}
		if err := s.backend.Pop(1); err != nil {
			return err
		}
		s.prepared = true
	}
	objective := term
	var err error
	if !maximize {
		objective, err = s.manager.MkArithmeticNeg(term)
		if err != nil {
			return err
		}
	}
	ref, err := s.manager.NewRef(term)
	if err != nil {
		return err
	}
	backendIndex, err := s.backend.AddObjective(objective)
	if err != nil {
		return errors.Join(err, s.manager.DelRef(&ref))
	}
	s.objectives = append(s.objectives, arithmeticObjective{term: ref, maximize: maximize, backendIndex: backendIndex})
	return nil
}

// AddSoftObjective registers one source MaxSMT group. Negative weights are
// normalized by complementing the term and retaining the constant offset,
// exactly as opt_context::is_maxsat does before invoking MaxSMT.
func (s *ArithmeticSolver) AddSoftObjective(terms []ast.ASTHandle, weights []*big.Rat) (resultErr error) {
	if s == nil || s.backend == nil || len(terms) != len(weights) {
		return ast.ErrASTHandle
	}
	set, err := newWeightedSoftSet(s.manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, set.Close()) }()
	for i, term := range terms {
		if err = set.Add(term, weights[i]); err != nil {
			return err
		}
	}
	refs, normalizedWeights, offset, err := set.Release()
	if err != nil {
		return err
	}
	objective := arithmeticObjective{soft: true, softTerms: refs, softWeights: normalizedWeights, softOffset: offset}
	s.objectives = append(s.objectives, objective)
	return nil
}

func (s *ArithmeticSolver) replaceModel(result *ArithmeticResult) error {
	md, err := s.backend.AcquireModel()
	if err != nil {
		return err
	}
	if md == nil {
		return ast.ErrASTHandle
	}
	if result.Model != nil {
		if err = result.Model.DecRef(); err != nil {
			_ = md.DecRef()
			return err
		}
	}
	result.Model = md
	return s.captureResultLabels(result)
}

func (s *ArithmeticSolver) orient(index int, value smtcore.SMTArithmeticObjectiveValue) smtcore.SMTArithmeticObjectiveValue {
	if s.objectives[index].maximize || value.Unbounded {
		return value
	}
	value.Finite = value.Finite.Clone()
	value.Finite.Neg()
	return value
}

func (s *ArithmeticSolver) optimizeArithmetic(index int, result *ArithmeticResult) (status lbool.LBool, optimal bool, resultErr error) {
	if err := s.backend.Push(); err != nil {
		return lbool.Undef, false, err
	}
	active := true
	defer func() {
		if active {
			resultErr = errors.Join(resultErr, s.backend.Pop(1))
		}
	}()
	status, err := s.backend.CheckSat(nil)
	if err != nil || status != lbool.True {
		return status, false, err
	}
	if err = s.replaceModel(result); err != nil {
		return lbool.Undef, false, err
	}
	for status == lbool.True {
		value, blocker, shared, err := s.backend.MaximizeObjective(s.objectives[index].backendIndex)
		if err != nil {
			return lbool.Undef, false, err
		}
		blockerRef, err := s.manager.NewRef(blocker)
		if err != nil {
			return lbool.Undef, false, err
		}
		updated, updateErr := s.backend.UpdateModel(shared)
		if updateErr == nil && updated {
			updateErr = s.replaceModel(result)
		}
		if updateErr != nil {
			return lbool.Undef, false, errors.Join(updateErr, s.manager.DelRef(&blockerRef))
		}
		result.Values[index].Arithmetic = s.orient(index, value)
		if value.Unbounded {
			return lbool.True, true, s.manager.DelRef(&blockerRef)
		}
		blocker, err = s.manager.RefGet(blockerRef)
		if err == nil {
			err = s.backend.Assert(blocker)
		}
		err = errors.Join(err, s.manager.DelRef(&blockerRef))
		if err != nil {
			return lbool.Undef, false, err
		}
		status, err = s.backend.CheckSat(nil)
		if err != nil {
			return lbool.Undef, false, err
		}
	}
	return status, status == lbool.False, nil
}

func (s *ArithmeticSolver) softBound(objective *arithmeticObjective, bound *big.Rat, strict bool) (result ast.ASTHandle, resultErr error) {
	service := s.manager.NumeralService()
	if service == nil || bound == nil {
		return result, ast.ErrASTHandle
	}
	coefficients := make([]unumeral.OwnedRational, len(objective.softWeights))
	arguments := make([]ast.ASTHandle, len(objective.softTerms))
	var threshold unumeral.OwnedRational
	defer func() {
		if threshold != (unumeral.OwnedRational{}) {
			resultErr = errors.Join(resultErr, service.Del(&threshold))
		}
		for i := len(coefficients) - 1; i >= 0; i-- {
			if coefficients[i] != (unumeral.OwnedRational{}) {
				resultErr = errors.Join(resultErr, service.Del(&coefficients[i]))
			}
		}
	}()
	for i := range coefficients {
		value, err := service.NewValue()
		if err != nil {
			return result, err
		}
		coefficients[i] = value
		if err = service.SetString(&coefficients[i], objective.softWeights[i].RatString()); err != nil {
			return result, err
		}
		arguments[i], err = s.manager.RefGet(objective.softTerms[i])
		if err != nil {
			return result, err
		}
	}
	var err error
	threshold, err = service.NewValue()
	if err != nil {
		return result, err
	}
	if err = service.SetString(&threshold, bound.RatString()); err != nil {
		return result, err
	}
	if !strict {
		return s.manager.MkPseudoBooleanGE(coefficients, arguments, &threshold)
	}
	atMost, err := s.manager.MkPseudoBooleanLE(coefficients, arguments, &threshold)
	if err != nil {
		return result, err
	}
	ref, err := s.manager.NewRef(atMost)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&ref)) }()
	atMost, err = s.manager.RefGet(ref)
	if err != nil {
		return result, err
	}
	return s.manager.MkNot(atMost)
}

func (s *ArithmeticSolver) optimizeSoft(index int, result *ArithmeticResult) (status lbool.LBool, optimal bool, resultErr error) {
	if err := s.backend.Push(); err != nil {
		return lbool.Undef, false, err
	}
	var activeEngine maxSMTEngine
	defer func() {
		if popErr := s.backend.Pop(1); popErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pop MaxSMT scope: %w", popErr))
		}
		if activeEngine != nil {
			if closeErr := activeEngine.close(); closeErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("close MaxSMT engine: %w", closeErr))
			}
		}
	}()
	status, err := s.backend.CheckSat(nil)
	if err != nil || status != lbool.True {
		return status, false, err
	}
	if err = s.replaceModel(result); err != nil {
		return lbool.Undef, false, err
	}
	objective := &s.objectives[index]
	engine, err := newMaxSMTEngine(s, objective, index)
	if err != nil {
		return lbool.Undef, false, fmt.Errorf("construct MaxSMT engine: %w", err)
	}
	if engine != nil {
		activeEngine = engine
		status, optimal, err = engine.optimize(result)
		result.MaxSMT = append(result.MaxSMT, engine.statistics())
		if optimal {
			finalizeSoftBounds(result, index)
		}
		if err != nil {
			err = fmt.Errorf("run MaxSMT engine: %w", err)
		}
		return status, optimal, err
	}
	for {
		cost, satisfied, assignment, e := s.evaluateSoftDetailed(objective, result.Model)
		if e != nil {
			return lbool.Undef, false, e
		}
		setSoftObjectiveValue(result, index, cost, satisfied, assignment)
		if len(objective.softTerms) == 0 {
			finalizeSoftBounds(result, index)
			return lbool.False, true, nil
		}
		blocker, e := s.softBound(objective, &satisfied, true)
		if e != nil {
			return lbool.Undef, false, e
		}
		blockerRef, e := s.manager.NewRef(blocker)
		if e != nil {
			return lbool.Undef, false, e
		}
		blocker, e = s.manager.RefGet(blockerRef)
		if e == nil {
			e = s.backend.Assert(blocker)
		}
		e = errors.Join(e, s.manager.DelRef(&blockerRef))
		if e != nil {
			return lbool.Undef, false, e
		}
		status, e = s.backend.CheckSat(nil)
		if e != nil {
			return lbool.Undef, false, e
		}
		switch status {
		case lbool.True:
			if err = s.replaceModel(result); err != nil {
				return lbool.Undef, false, err
			}
			continue
		case lbool.False:
			finalizeSoftBounds(result, index)
			return status, true, nil
		default:
			return status, false, nil
		}
	}
}

func (s *ArithmeticSolver) optimizeOne(index int, result *ArithmeticResult) (lbool.LBool, bool, error) {
	if s.objectives[index].soft {
		return s.optimizeSoft(index, result)
	}
	return s.optimizeArithmetic(index, result)
}

func (s *ArithmeticSolver) captureArithmeticModel(index int, value *ObjectiveValue, md *model.ModelCore) (resultErr error) {
	if value == nil || md == nil {
		return ast.ErrASTHandle
	}
	evaluator, err := model.NewModelEvaluator(md, s.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	term, err := s.manager.RefGet(s.objectives[index].term)
	if err != nil {
		return err
	}
	ref, err := evaluator.EvaluateValue(term)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&ref)) }()
	h, err := s.manager.RefGet(ref)
	if err != nil {
		return err
	}
	text, _, matched, err := s.manager.ArithmeticNumeralText(h)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	rational, matched := new(big.Rat).SetString(text)
	if !matched {
		return ast.ErrASTDomain
	}
	value.ArithmeticModel = smtcore.SMTArithmeticObjectiveValue{Finite: simplex.RationalInfinitesimal(rational)}
	value.ArithmeticModelValid = true
	return nil
}

func (s *ArithmeticSolver) commitArithmeticObjective(index int, value ObjectiveValue) error {
	if !value.ArithmeticModelValid {
		return ast.ErrASTDomain
	}
	committed := value.ArithmeticModel
	if !s.objectives[index].maximize {
		committed.Finite = committed.Finite.Clone()
		committed.Finite.Neg()
	}
	bound, err := s.backend.MakeObjectiveLowerBound(s.objectives[index].backendIndex, s.filter, committed.Finite)
	if err != nil {
		return err
	}
	ref, err := s.manager.NewRef(bound)
	if err != nil {
		return err
	}
	bound, err = s.manager.RefGet(ref)
	if err == nil {
		err = s.backend.Assert(bound)
	}
	return errors.Join(err, s.manager.DelRef(&ref))
}

func (s *ArithmeticSolver) commitSoftObjective(index int, value ObjectiveValue) error {
	objective := &s.objectives[index]
	if err := s.validateSoftCommit(index, value); err != nil {
		return err
	}
	if len(objective.softTerms) == 0 {
		return nil
	}
	bound, err := s.softBound(objective, &value.SoftSatisfied, false)
	if err != nil {
		return err
	}
	ref, err := s.manager.NewRef(bound)
	if err != nil {
		return err
	}
	bound, err = s.manager.RefGet(ref)
	if err == nil {
		err = s.backend.Assert(bound)
	}
	return errors.Join(err, s.manager.DelRef(&ref))
}

func (s *ArithmeticSolver) commitObjective(index int, value ObjectiveValue) error {
	if s.objectives[index].soft {
		return s.commitSoftObjective(index, value)
	}
	return s.commitArithmeticObjective(index, value)
}

func (s *ArithmeticSolver) retainRemainingModelValues(start int, result *ArithmeticResult) error {
	for i := start; i < len(s.objectives); i++ {
		if s.objectives[i].soft {
			cost, satisfied, assignment, err := s.evaluateSoftDetailed(&s.objectives[i], result.Model)
			if err != nil {
				return err
			}
			setSoftObjectiveValue(result, i, cost, satisfied, assignment)
			continue
		}
		if err := s.captureArithmeticModel(i, &result.Values[i], result.Model); err != nil {
			return err
		}
		result.Values[i].Arithmetic = result.Values[i].ArithmeticModel
	}
	return nil
}

// Check runs the source blocker loop and preserves the last valid optimized
// model across final unsatisfiable blocker checks. Attained finite objectives
// are committed in order to implement the retained lexicographic priority.
func (s *ArithmeticSolver) Check() (result *ArithmeticResult, resultErr error) {
	if s == nil || s.backend == nil || len(s.objectives) == 0 {
		return nil, ast.ErrASTHandle
	}
	result = &ArithmeticResult{Status: lbool.Undef, Values: make([]ObjectiveValue, len(s.objectives))}
	defer func() {
		if resultErr == nil && result != nil && result.Model != nil {
			resultErr = s.filterResultModel(result)
		}
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	for i := range s.objectives {
		outer := i+1 < len(s.objectives)
		if outer {
			if err := s.backend.Push(); err != nil {
				return nil, err
			}
		}
		status, optimal, err := s.optimizeOne(i, result)
		if outer {
			err = errors.Join(err, s.backend.Pop(1))
		}
		if err != nil {
			return nil, err
		}
		if !optimal {
			result.Status = status
			return result, nil
		}
		if s.objectives[i].soft {
			finalizeSoftBounds(result, i)
		}
		if !s.objectives[i].soft {
			if err = s.captureArithmeticModel(i, &result.Values[i], result.Model); err != nil {
				return nil, err
			}
			value := result.Values[i].Arithmetic
			if value.Unbounded || value.Finite.Infinity.Sign() != 0 {
				for j := i + 1; j < len(s.objectives); j++ {
					if !s.objectives[j].soft {
						result.Values[j].ArithmeticOpenBounds = true
					}
				}
				result.Status, result.Optimal = lbool.True, true
				return result, nil
			}
			if i+1 < len(s.objectives) && value.Finite.Epsilon.Sign() != 0 {
				result.Values[i].ArithmeticInterval = true
				if err = s.retainRemainingModelValues(i+1, result); err != nil {
					return nil, err
				}
				result.Status, result.Optimal = lbool.True, true
				return result, nil
			}
		}
		if i+1 < len(s.objectives) {
			if err = s.commitObjective(i, result.Values[i]); err != nil {
				return nil, err
			}
		}
	}
	result.Status, result.Optimal = lbool.True, true
	return result, nil
}

func (s *ArithmeticSolver) Close() error {
	if s == nil || s.backend == nil {
		return nil
	}
	for len(s.objectives) != 0 {
		last := len(s.objectives) - 1
		objective := &s.objectives[last]
		if objective.term != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&objective.term); err != nil {
				return err
			}
		}
		for len(objective.softTerms) != 0 {
			i := len(objective.softTerms) - 1
			if err := s.manager.DelRef(&objective.softTerms[i]); err != nil {
				return err
			}
			objective.softTerms = objective.softTerms[:i]
		}
		s.objectives = s.objectives[:last]
	}
	var err error
	if s.filter != nil {
		err = s.filter.Close()
		if err == nil {
			s.filter = nil
		}
	}
	err = errors.Join(err, s.backend.Close())
	if err == nil {
		s.backend = nil
	}
	return err
}
