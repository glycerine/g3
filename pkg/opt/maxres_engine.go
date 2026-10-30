package opt

// Source: src/opt/maxres.cpp maxres state and init_local.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type maxResEngine struct {
	solver           *ArithmeticSolver
	objective        *arithmeticObjective
	index            int
	config           maxSMTConfig
	strategy         maxResStrategy
	owner            maxSMTASTOwner
	active           []ast.ASTHandle
	weights          map[ast.ASTHandle]*big.Rat
	proxies          map[ast.ASTHandle]ast.ASTHandle
	lower            big.Rat
	upper            big.Rat
	bestSet          bool
	correction       []ast.ASTHandle
	upperBoundActive bool
	stats            maxResStatistics
}

func newMaxResEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (result *maxResEngine, resultErr error) {
	e := &maxResEngine{solver: s, objective: objective, index: index, config: config, strategy: maxResStrategyFromConfig(config), weights: make(map[ast.ASTHandle]*big.Rat), proxies: make(map[ast.ASTHandle]ast.ASTHandle)}
	e.lower.Set(&objective.softOffset)
	e.owner.manager = s.manager
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
	e.active, e.correction, e.weights, e.proxies = nil, nil, nil, nil
	return e.owner.close()
}
