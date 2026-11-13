package fp

// Source: src/muz/bmc/dl_bmc_engine.h/.cpp bmc::m_solver and
// src/muz/clp/clp_context.cpp clp::imp::m_solver. The smt::kernel is a
// member of the engine: context::ensure_engine constructs it, and the
// engine's query resets it (a fresh smt::context) before obtaining the
// rules. The kernel then lives until the next query or engine destruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

// bmcSolverSlot is the engine's smt::kernel, shared by the query clones.
type bmcSolverSlot struct {
	solver *smtcore.SMTSolver
	// fresh records that bmc::query's reset already ran for the current
	// query; a Go-only retry of the same query resets again.
	fresh bool
	// owner is the engine (bmc) whose m_rule_trace and m_answer the query
	// clones replace.
	owner *PropositionalEngine
	// clp holds clp::imp's m_ground and m_goals for the CLP engine.
	clp *clp.State
	// TAB: tb::index's kernel (constructed first) and tab::imp::m_solver,
	// which persist across queries without a reset.
	tabSub, tabQuery *smtcore.SMTSolver
	tabCoop          *cooperate.Cooperation
}

func (e *PropositionalEngine) isBMCBackend() bool {
	return e.backend == EngineBMC.String() || e.backend == EngineQBMC.String()
}

// usesEngineKernel selects the engines whose interface owns one smt::kernel.
func (e *PropositionalEngine) usesEngineKernel() bool {
	return e.isBMCBackend() || e.backend == EngineCLP.String()
}

func (e *PropositionalEngine) newBMCKernel() (_ *smtcore.SMTSolver, resultErr error) {
	config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
	if e.backend != EngineCLP.String() {
		return smtcore.NewSMTSolver(e.manager, nil, "", config)
	}
	// clp::imp sets m_fparams.m_mbqi = false and m_timeout = 1000.
	local, err := e.params.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.params.Del(&local)) }()
	mbqi, timeout := "mbqi", "timeout"
	if err = e.params.SetBoolName(&local, &mbqi, false); err != nil {
		return nil, err
	}
	if err = e.params.SetUIntName(&local, &timeout, 1000); err != nil {
		return nil, err
	}
	return smtcore.NewSMTSolver(e.manager, &local, "", config)
}

// ensureBMCSolver is the bmc constructor's m_solver(m, m_fparams), run by
// context::ensure_engine.
func (e *PropositionalEngine) ensureBMCSolver() error {
	if e.bmcSlot != nil || e.params == nil || e.globals == nil {
		return nil
	}
	if e.backend == EngineTab.String() {
		return e.ensureTabKernels()
	}
	solver, err := e.newBMCKernel()
	if err != nil {
		return err
	}
	e.bmcSlot = &bmcSolverSlot{solver: solver, owner: e}
	if e.backend == EngineCLP.String() {
		e.bmcSlot.clp = clp.NewState(e.manager)
	}
	return nil
}

// resetBMCSolver is smt::kernel::reset: the old context is destroyed before
// the new one is constructed.
func (e *PropositionalEngine) resetBMCSolver() error {
	if e.bmcSlot == nil {
		if err := e.ensureBMCSolver(); err != nil || e.bmcSlot == nil {
			return err
		}
	}
	if e.bmcSlot.solver != nil {
		old := e.bmcSlot.solver
		e.bmcSlot.solver = nil
		if err := old.Close(); err != nil {
			return err
		}
	}
	solver, err := e.newBMCKernel()
	if err != nil {
		return err
	}
	e.bmcSlot.solver = solver
	e.bmcSlot.fresh = true
	// clp::imp::query: m_goals.reset() follows the kernel reset.
	if err := e.bmcSlot.clp.ResetGoals(); err != nil {
		return err
	}
	// bmc::query: m_answer = nullptr follows the kernel reset.
	if e.bmcProof != (ast.ASTRef{}) {
		if err := e.manager.DelRef(&e.bmcProof); err != nil {
			return err
		}
	}
	return nil
}

// bmcQuerySolver returns the engine kernel for one search. The first search
// of a query uses the context bmc::query reset; a retry resets it again.
func (e *PropositionalEngine) bmcQuerySolver() (*smtcore.SMTSolver, error) {
	if e.bmcSlot == nil || e.bmcSlot.solver == nil || !e.bmcSlot.fresh {
		if err := e.resetBMCSolver(); err != nil {
			return nil, err
		}
		if e.bmcSlot == nil {
			return nil, ErrPropositionalEngine
		}
	}
	e.bmcSlot.fresh = false
	return e.bmcSlot.solver, nil
}

func (e *PropositionalEngine) closeBMCSolver() error {
	slot := e.bmcSlot
	e.bmcSlot = nil
	if slot == nil || e.bmcSlotBorrowed {
		e.bmcSlotBorrowed = false
		return nil
	}
	if err := slot.clp.Close(); err != nil {
		return err
	}
	for _, kernel := range []**smtcore.SMTSolver{&slot.tabQuery, &slot.tabSub} {
		if *kernel != nil {
			if err := (*kernel).Close(); err != nil {
				return err
			}
			*kernel = nil
		}
	}
	if slot.tabCoop != nil {
		if err := slot.tabCoop.Close(); err != nil {
			return err
		}
		slot.tabCoop = nil
	}
	if slot.solver == nil {
		return nil
	}
	return slot.solver.Close()
}

// resetBMCRuleTrace is the compile setup()'s b.m_rule_trace.reset() on the
// engine that owns the previous query's trace.
func (e *PropositionalEngine) resetBMCRuleTrace() error {
	if e.bmcSlot == nil || e.bmcSlot.owner == nil {
		return nil
	}
	return e.bmcSlot.owner.clearBMCTrace()
}

// tabModelEvaluator is the evaluator configuration of the TAB kernels.
func (e *PropositionalEngine) tabModelEvaluator(coop *cooperate.Cooperation) model.ModelEvaluatorConfig {
	return model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: coop}
}

// ensureTabKernels is the tab::imp constructor: m_index(m) builds the
// subsumption kernel, then m_solver(m, m_fparams) with m_mbqi = false and
// m_timeout = 1000.
func (e *PropositionalEngine) ensureTabKernels() (resultErr error) {
	slot := &bmcSolverSlot{owner: e}
	defer func() {
		if resultErr != nil {
			slot.owner = nil
			e.bmcSlot = slot
			resultErr = errors.Join(resultErr, e.closeBMCSolver())
		}
	}()
	var err error
	slot.tabCoop, err = cooperate.NewCooperation(e.manager.Budget())
	if err != nil {
		return err
	}
	config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals, ModelEvaluator: e.tabModelEvaluator(slot.tabCoop)}
	slot.tabSub, err = smtcore.NewSMTSolver(e.manager, nil, "", config)
	if err != nil {
		return err
	}
	if err = slot.tabSub.SetProduceModels(true); err != nil {
		return err
	}
	local, err := e.params.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.params.Del(&local)) }()
	mbqi, timeout := "mbqi", "timeout"
	if err = e.params.SetBoolName(&local, &mbqi, false); err != nil {
		return err
	}
	if err = e.params.SetUIntName(&local, &timeout, 1000); err != nil {
		return err
	}
	slot.tabQuery, err = smtcore.NewSMTSolver(e.manager, &local, "", config)
	if err != nil {
		return err
	}
	e.bmcSlot = slot
	return nil
}
