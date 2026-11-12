package opt

// Source: src/opt/maxres.cpp maxres state and init_local.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
)

type maxResEngine struct {
	solver    *ArithmeticSolver
	objective *arithmeticObjective
	index     int
	config    maxSMTConfig
	strategy  maxResStrategy
	// trail is maxres::m_trail (new_assumption's literals, max_resolve's
	// conjunctions); defOwner holds m_defs; baseTrail is
	// maxsmt_solver_base::m_trail (process_mutex's disjunctions).
	trail     maxSMTASTOwner
	defOwner  maxSMTASTOwner
	baseTrail maxSMTASTOwner
	// lastCore is m_B: the last core or correction set max_resolve copied.
	lastCore          []ast.ASTHandle
	active            []ast.ASTHandle
	weights           map[ast.ASTHandle]*big.Rat
	proxies           map[ast.ASTHandle]ast.ASTHandle
	mus               *solver.MUS
	lower             big.Rat
	upper             big.Rat
	bestSet           bool
	correction        []ast.ASTHandle
	correctionModel   *model.ModelCore
	correctionSize    int
	currentCorrection []ast.ASTHandle
	stats             maxResStatistics
	// defs is maxres::m_defs: the max_resolve/cs_max_resolve definitions
	// commit_assignment re-asserts.
	defs                 []ast.ASTHandle
	foundFeasibleOptimum bool
	initialCandidate     bool
}

func newMaxResEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (result *maxResEngine, resultErr error) {
	e := &maxResEngine{solver: s, objective: objective, index: index, config: config, strategy: maxResStrategyFromConfig(config), weights: make(map[ast.ASTHandle]*big.Rat), proxies: make(map[ast.ASTHandle]ast.ASTHandle)}
	e.lower.Set(&objective.softOffset)
	e.trail.manager, e.defOwner.manager, e.baseTrail.manager = s.manager, s.manager, s.manager
	mus, err := solver.NewMUS(s.backend, solver.MUSConfig{})
	if err != nil {
		return nil, err
	}
	e.mus = mus
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, e.close())
			result = nil
		}
	}()
	terms, err := e.preprocessMutexes(objective)
	if err != nil {
		return nil, err
	}
	for i := range terms {
		if err = e.addSoft(terms[i].expression, &terms[i].weight); err != nil {
			return nil, fmt.Errorf("maxres mutex term %d: %w", i, err)
		}
	}
	return e, nil
}

func (e *maxResEngine) close() error {
	if e == nil {
		return nil
	}
	// ~maxres releases its members in reverse declaration order: m_csmodel,
	// m_trail, m_mus, m_defs, m_asms, m_B; then maxsmt_solver_base's
	// m_trail. m_asms and m_B only hold literals m_trail also kept, so
	// they are pinned here for their release after m_defs.
	m := e.trail.manager
	var result error
	pin := func(hs []ast.ASTHandle) []ast.ASTRef {
		refs := make([]ast.ASTRef, 0, len(hs))
		for _, h := range hs {
			if m == nil {
				break
			}
			ref, err := m.NewRef(h)
			if err != nil {
				result = errors.Join(result, err)
				continue
			}
			refs = append(refs, ref)
		}
		return refs
	}
	unpin := func(refs []ast.ASTRef) {
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}
	asms := pin(e.active)
	core := pin(e.lastCore)
	e.active, e.lastCore, e.correction, e.currentCorrection, e.weights, e.proxies = nil, nil, nil, nil, nil, nil
	if e.correctionModel != nil {
		result = errors.Join(result, e.correctionModel.DecRef())
		e.correctionModel = nil
	}
	result = errors.Join(result, e.trail.close())
	if e.mus != nil {
		result = errors.Join(result, e.mus.Close())
		e.mus = nil
	}
	result = errors.Join(result, e.defOwner.close())
	e.defs = nil
	unpin(asms)
	unpin(core)
	return errors.Join(result, e.baseTrail.close())
}

func (e *maxResEngine) clearCorrectionModel() error {
	if e.correctionModel == nil {
		e.correctionSize = 0
		return nil
	}
	err := e.correctionModel.DecRef()
	if err == nil {
		e.correctionModel = nil
		e.correctionSize = 0
	}
	return err
}
