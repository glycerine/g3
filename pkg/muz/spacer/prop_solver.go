package spacer

// Source: src/muz/spacer/spacer_prop_solver.h and
// src/muz/spacer/spacer_prop_solver.cpp.

import (
	"errors"
	"fmt"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrPropSolverOwner = errors.New("muz/spacer: invalid or closed propositional solver")

type PropSolverConfig struct {
	Itp                     ItpSolverConfig
	KeepProxy               bool
	UseInductiveGeneralizer bool
	UseArrayEqGeneralizer   bool
	UseEqClassGeneralizer   bool
	LemmaSanityCheck        bool
	UseLemmaAsCTI           bool
}

type PropCheckOptions struct {
	Background      []ast.ASTHandle
	NeedCore        bool
	NeedModel       bool
	SubsetBasedCore bool
	Level           *uint32
	DeltaLevel      bool
	SolverID        uint32
	WeakAbstraction bool
	Weakness        *uint32
	// SoftRefs are the caller's references to the soft formulas (the
	// expr_ref_vector soft that maxsmt resets and repopulates). When set,
	// maxsmt releases them once the soft formulas are being minimized and
	// appends references for the kept soft formulas.
	SoftRefs *[]ast.ASTRef
}

type PropCheckResult struct {
	Status       lbool.LBool
	Soft         []ast.ASTHandle
	Core         []ast.ASTRef
	Model        *model.ModelCore
	UsesLevel    uint32
	AssumesLevel bool
	FarkasCore   bool
}

// Close releases the result's owning core references and model. Soft formulas
// borrow the caller's input expressions.
func (r *PropCheckResult) Close(manager *ast.ASTManager) (result error) {
	if r == nil {
		return nil
	}
	if manager == nil && len(r.Core) != 0 {
		return ast.ErrASTHandle
	}
	for i := range r.Core {
		result = errors.Join(result, manager.DelRef(&r.Core[i]))
	}
	r.Core = nil
	if r.Model != nil {
		result = errors.Join(result, r.Model.DecRef())
		r.Model = nil
	}
	return result
}

// PropSolver owns two ItpSolver wrappers and borrows their backends. It keeps
// source-compatible level guards and poor-man's MaxSAT behavior.
type PropSolver struct {
	manager       *ast.ASTManager
	contexts      [2]*ItpSolver
	name          string
	levelPositive []ast.ASTHandle
	levelNegative []ast.ASTHandle
	levelAtoms    map[ast.ASTHandle]bool
	// positiveRefs and negativeRefs are m_pos_level_atoms and
	// m_neg_level_atoms.
	positiveRefs  []ast.ASTRef
	negativeRefs  []ast.ASTRef
	keepProxy     bool
	ownedBackends [2]*VirtualSolver
	closed        bool
	closing       bool
}

func NewPropSolver(manager *ast.ASTManager, backends [2]solver.Solver, background ast.ASTHandle, name string, config PropSolverConfig) (_ *PropSolver, resultErr error) {
	if manager == nil || backends[0] == nil || backends[1] == nil {
		return nil, ErrPropSolverOwner
	}
	p := &PropSolver{manager: manager, name: name, levelAtoms: make(map[ast.ASTHandle]bool), keepProxy: config.KeepProxy}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	for i := range p.contexts {
		context, err := NewItpSolver(backends[i], config.Itp)
		if err != nil {
			return nil, err
		}
		if context.manager != manager {
			_ = context.Close()
			return nil, ErrPropSolverOwner
		}
		p.contexts[i] = context
		if err = context.Assert(background); err != nil {
			return nil, err
		}
	}
	committed = true
	return p, nil
}

func (p *PropSolver) open() error {
	if p == nil || p.closed || p.closing || p.manager == nil || p.contexts[0] == nil || p.contexts[1] == nil {
		return ErrPropSolverOwner
	}
	return nil
}

func (p *PropSolver) pin(vector *[]ast.ASTRef, expression ast.ASTHandle) error {
	reference, err := p.manager.NewRef(expression)
	if err != nil {
		return err
	}
	*vector = append(*vector, reference)
	return nil
}

func (p *PropSolver) addLevel() error {
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	positive, err := p.manager.MkFreshConst(fmt.Sprintf("%s#level_%d", p.name, len(p.levelPositive)), boolean)
	if err != nil {
		return err
	}
	if err = p.pin(&p.positiveRefs, positive); err != nil {
		return err
	}
	negative, err := p.manager.MkNot(positive)
	if err != nil {
		return err
	}
	if err = p.pin(&p.negativeRefs, negative); err != nil {
		return err
	}
	p.levelPositive = append(p.levelPositive, positive)
	p.levelNegative = append(p.levelNegative, negative)
	p.levelAtoms[positive] = true
	p.levelAtoms[negative] = true
	return nil
}

func (p *PropSolver) EnsureLevel(level uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	for uint32(len(p.levelPositive)) <= level {
		if err := p.addLevel(); err != nil {
			return err
		}
	}
	return nil
}

// AddLevel is prop_solver::add_level: one more frame guard, unconditionally.
func (p *PropSolver) AddLevel() error {
	if err := p.open(); err != nil {
		return err
	}
	return p.addLevel()
}

func (p *PropSolver) LevelCount() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	return uint32(len(p.levelPositive)), nil
}

func (p *PropSolver) Assert(expression ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.contexts[0].Assert(expression); err != nil {
		return err
	}
	return p.contexts[1].Assert(expression)
}

func (p *PropSolver) AssertAtLevel(expression ast.ASTHandle, level uint32) error {
	if err := p.EnsureLevel(level); err != nil {
		return err
	}
	guarded, err := p.manager.MkOr(expression, p.levelPositive[level])
	if err != nil {
		return err
	}
	// app_ref lform owns the guarded formula across both contexts'
	// assert_expr: the first context's rewrite may release its own copy
	// before the second context receives it.
	lform, err := newSpacerAssertOwner(p.manager, guarded)
	if err != nil {
		return err
	}
	return lform.finish(p.Assert(guarded))
}

func (p *PropSolver) levelAssumptions(level uint32, delta bool) []ast.ASTHandle {
	result := make([]ast.ASTHandle, len(p.levelPositive))
	for i := range result {
		active := uint32(i) >= level
		if delta {
			active = uint32(i) == level
		}
		if active {
			result[i] = p.levelNegative[i]
		} else {
			result[i] = p.levelPositive[i]
		}
	}
	return result
}

func containsHandle(values []ast.ASTHandle, want ast.ASTHandle) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (p *PropSolver) maxSMT(context *ItpSolver, hard, soft []ast.ASTHandle, softRefs *[]ast.ASTRef) (_ lbool.LBool, _ []ast.ASTHandle, resultErr error) {
	// itp_solver::scoped_mk_proxy _p_(*m_ctx, hard): the hard formulas get
	// their proxies before check_sat proxies the background assumptions.
	hard, err := context.MkProxies(hard)
	if err != nil {
		return lbool.Undef, nil, err
	}
	hardSize := len(hard)
	// hard.append(soft): the local vector references every assumption.
	assumptions := append(append([]ast.ASTHandle(nil), hard...), soft...)
	references, err := refsFromHandles(p.manager, assumptions)
	if err != nil {
		return lbool.Undef, nil, err
	}
	// expr_ref saved: the soft formula most recently found in the core.
	saved, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return lbool.Undef, nil, errors.Join(err, releaseNormalizeRefs(p.manager, references))
	}
	defer func() {
		// hard.resize(hard_sz) releases the remaining soft formulas front
		// to back; saved dies when maxsmt returns.
		for i := range references {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&references[i]))
		}
		resultErr = errors.Join(resultErr, p.manager.DelRef(&saved))
	}()
	status, err := context.CheckSat(assumptions)
	if err != nil || status != lbool.False || len(soft) == 0 {
		return status, append([]ast.ASTHandle(nil), soft...), err
	}
	// soft.reset(): the caller's references die here.
	if softRefs != nil {
		if err = releaseNormalizeRefs(p.manager, *softRefs); err != nil {
			return lbool.Undef, nil, err
		}
		*softRefs = nil
	}
	remaining := assumptions
	for len(remaining) > hardSize {
		var core []ast.ASTHandle
		core, err = context.UnsatCore()
		if err != nil {
			return lbool.Undef, nil, err
		}
		found := false
		for i := hardSize; i < len(remaining); i++ {
			if !containsHandle(core, remaining[i]) {
				continue
			}
			// saved = hard.get(i) releases the previously saved formula;
			// hard[i] = hard.back() releases the vector's reference.
			if err = p.manager.SetRef(saved, remaining[i]); err != nil {
				return lbool.Undef, nil, err
			}
			last := len(remaining) - 1
			remaining[i] = remaining[last]
			remaining = remaining[:last]
			if err = p.manager.DelRef(&references[i]); err != nil {
				return lbool.Undef, nil, err
			}
			references[i] = references[last]
			references = references[:last]
			found = true
			break
		}
		if !found {
			return lbool.False, nil, nil
		}
		status, err = context.CheckSat(remaining)
		if err != nil || status != lbool.False {
			break
		}
	}
	if status != lbool.True {
		return status, nil, err
	}
	// soft.push_back(hard.get(i)) for the kept soft formulas.
	kept := append([]ast.ASTHandle(nil), remaining[hardSize:]...)
	if softRefs != nil {
		for _, formula := range kept {
			reference, err := p.manager.NewRef(formula)
			if err != nil {
				return lbool.Undef, nil, err
			}
			*softRefs = append(*softRefs, reference)
		}
	}
	return status, kept, nil
}

func refsFromHandles(manager *ast.ASTManager, values []ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	result := make([]ast.ASTRef, 0, len(values))
	committed := false
	defer func() {
		if !committed {
			for i := range result {
				resultErr = errors.Join(resultErr, manager.DelRef(&result[i]))
			}
		}
	}()
	for _, value := range values {
		reference, err := manager.NewRef(value)
		if err != nil {
			return nil, err
		}
		result = append(result, reference)
	}
	committed = true
	return result, nil
}

func (p *PropSolver) usedLevel(core []ast.ASTHandle) uint32 {
	result := uint32(math.MaxUint32)
	for _, expression := range core {
		if !p.levelAtoms[expression] {
			continue
		}
		for i, negative := range p.levelNegative {
			if expression == negative && uint32(i) < result {
				result = uint32(i)
			}
		}
	}
	return result
}

func (p *PropSolver) scopedWeakAbstraction(context *ItpSolver, enabled bool, weakness uint32) (cleanup func() error, resultErr error) {
	if context == nil || context.backend == nil || p.contexts[0].config.Params == nil {
		return nil, ErrPropSolverOwner
	}
	params := p.contexts[0].config.Params
	current, err := context.backend.Params()
	if err != nil || current == nil {
		return nil, errors.Join(err, ErrPropSolverOwner)
	}
	copyParams := func(source *uparams.ParamsRef) (value uparams.ParamsRef, copyErr error) {
		value = params.EmptyValue()
		committed := false
		defer func() {
			if !committed {
				copyErr = errors.Join(copyErr, params.Del(&value))
			}
		}()
		ignoreName := "arith.ignore_int"
		if copyErr = params.SetBoolName(&value, &ignoreName, false); copyErr != nil {
			return uparams.ParamsRef{}, copyErr
		}
		if copyErr = params.Append(&value, source); copyErr != nil {
			return uparams.ParamsRef{}, copyErr
		}
		committed = true
		return value, nil
	}
	saved, err := copyParams(current)
	if err != nil {
		return nil, err
	}
	override, err := copyParams(&saved)
	if err != nil {
		return nil, errors.Join(err, params.Del(&saved))
	}
	ignoreName := "arith.ignore_int"
	if err = params.SetBoolName(&override, &ignoreName, enabled && weakness < 1); err == nil {
		arrayWeakName := "array.weak"
		err = params.SetBoolName(&override, &arrayWeakName, enabled && weakness < 2)
	}
	if err == nil {
		err = context.backend.UpdateParams(&override)
	}
	if err != nil {
		return nil, errors.Join(err, context.backend.UpdateParams(&saved), params.Del(&override), params.Del(&saved))
	}
	return func() error {
		return errors.Join(context.backend.UpdateParams(&saved), params.Del(&override), params.Del(&saved))
	}, nil
}

func (p *PropSolver) CheckAssumptions(hard, soft []ast.ASTHandle, options PropCheckOptions) (_ *PropCheckResult, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if options.Level != nil && *options.Level != InfinityLevel {
		if err := p.EnsureLevel(*options.Level); err != nil {
			return nil, err
		}
	}
	flatHard, err := p.manager.FlattenAnd(append([]ast.ASTHandle(nil), hard...))
	if err != nil {
		return nil, err
	}
	// The native source currently routes both solver IDs through context zero.
	context := p.contexts[0]
	if options.Weakness != nil {
		cleanup, err := p.scopedWeakAbstraction(context, options.WeakAbstraction, *options.Weakness)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, cleanup()) }()
	}
	if err = context.SetProduceModels(options.NeedModel); err != nil {
		return nil, err
	}
	backgroundCount, err := context.NumBackground()
	if err != nil {
		return nil, err
	}
	if !p.keepProxy {
		if err = context.Push(); err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, context.Pop(1)) }()
		for _, expression := range options.Background {
			if err = context.Assert(expression); err != nil {
				return nil, err
			}
		}
	} else {
		for _, expression := range options.Background {
			if err = context.PushBackground(expression); err != nil {
				return nil, err
			}
		}
	}
	defer func() {
		current, err := context.NumBackground()
		resultErr = errors.Join(resultErr, err)
		if err == nil && current > backgroundCount {
			resultErr = errors.Join(resultErr, context.PopBackground(current-backgroundCount))
		}
	}()
	if options.Level != nil {
		for _, expression := range p.levelAssumptions(*options.Level, options.DeltaLevel) {
			if err = context.PushBackground(expression); err != nil {
				return nil, err
			}
		}
	}
	status, keptSoft, err := p.maxSMT(context, flatHard, soft, options.SoftRefs)
	if err != nil {
		return nil, err
	}
	result := &PropCheckResult{Status: status, Soft: keptSoft, UsesLevel: math.MaxUint32}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, result.Close(p.manager))
		}
	}()
	if status != lbool.False && options.NeedModel {
		available := status == lbool.True
		if !available {
			if provider, ok := context.backend.(solver.CandidateModelProvider); ok {
				available, err = provider.PrepareCandidateModel()
				if err != nil {
					return nil, err
				}
			}
		}
		if available {
			result.Model, err = context.AcquireModel()
			if err != nil {
				return nil, err
			}
		}
	}
	if status == lbool.False {
		full, err := context.FullUnsatCore()
		if err != nil {
			return nil, err
		}
		result.UsesLevel = p.usedLevel(full)
		result.AssumesLevel = result.UsesLevel != math.MaxUint32
		if options.NeedCore {
			proofs, err := p.manager.ProofsEnabled()
			if err != nil {
				return nil, err
			}
			if proofs && !options.SubsetBasedCore {
				result.Core, err = context.ItpCore()
				result.FarkasCore = err == nil
			} else {
				var core []ast.ASTHandle
				core, err = context.UnsatCore()
				if err == nil {
					// manually undo proxies because maxsmt() adds them
					core, err = context.UndoProxies(core)
				}
				if err == nil {
					result.Core, err = refsFromHandles(p.manager, core)
				}
			}
			if err != nil {
				return nil, err
			}
		}
	}
	committed = true
	return result, nil
}

func (p *PropSolver) CollectStatistics(destination *statistics.Statistics) error {
	if err := p.open(); err != nil {
		return err
	}
	for _, context := range p.contexts {
		if err := context.CollectStatistics(destination); err != nil {
			return err
		}
	}
	return nil
}

func (p *PropSolver) ResetStatistics() error {
	if err := p.open(); err != nil {
		return err
	}
	for _, context := range p.contexts {
		if err := context.ResetStatistics(); err != nil {
			return err
		}
	}
	return nil
}

func (p *PropSolver) Close() (result error) {
	if p == nil {
		return ErrPropSolverOwner
	}
	if p.closed {
		return nil
	}
	p.closing = true
	// Members die in reverse declaration order: m_neg_level_atoms and then
	// m_pos_level_atoms (each front to back) before m_contexts[1], [0].
	if p.manager != nil {
		for _, vector := range [][]ast.ASTRef{p.negativeRefs, p.positiveRefs} {
			for i := range vector {
				if vector[i] != (ast.ASTRef{}) {
					if err := p.manager.DelRef(&vector[i]); err != nil {
						return err
					}
				}
			}
		}
		p.negativeRefs, p.positiveRefs = nil, nil
	}
	for i := len(p.contexts) - 1; i >= 0; i-- {
		if p.contexts[i] != nil {
			if err := p.contexts[i].Close(); err != nil {
				result = errors.Join(result, err)
			} else {
				p.contexts[i] = nil
			}
		}
	}
	// m_solvers are raw pointers into the virtual_solver_factory, which
	// destroys them.
	p.ownedBackends = [2]*VirtualSolver{}
	if result != nil {
		return result
	}
	p.manager = nil
	p.closing = false
	p.closed = true
	return result
}
