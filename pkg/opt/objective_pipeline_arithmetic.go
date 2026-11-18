package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ObjectivePipeline) arithmeticSolver() (solver *ArithmeticSolver, resultErr error) {
	solver, err := NewArithmeticSolver(p.manager, p.params, p.logic, p.config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, solver.Close())
			solver = nil
		}
	}()
	for _, ref := range p.hard {
		formula, e := p.manager.RefGet(ref)
		if e != nil {
			return nil, e
		}
		if e = solver.Assert(formula); e != nil {
			return nil, e
		}
	}
	for i := range p.objectives {
		objective := &p.objectives[i]
		if objective.term != (ast.ASTRef{}) {
			term, e := p.manager.RefGet(objective.term)
			if e != nil {
				return nil, e
			}
			if e = solver.AddObjective(term, objective.sourceKind == FormulaMaximize); e != nil {
				return nil, e
			}
			continue
		}
		terms, e := p.refHandles(objective.soft)
		if e != nil {
			return nil, e
		}
		if e = solver.AddSoftObjective(terms, objective.weights); e != nil {
			return nil, e
		}
	}
	return solver, nil
}

func (p *ObjectivePipeline) checkArithmetic() (result *ObjectivePipelineResult, resultErr error) {
	solver, err := p.arithmeticSolver()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, solver.Close()) }()
	backend, err := solver.Check()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, backend.Close()) }()
	q := p.manager.NumeralService().Manager
	result = &ObjectivePipelineResult{Status: backend.Status, Optimal: backend.Optimal, Model: backend.Model, Values: make([]ObjectivePipelineValue, len(backend.Values)), Q: q}
	for i := range result.Values {
		result.Values[i].Exact = numeral.NewRational()
	}
	backend.Model = nil
	result.Statistics = ObjectivePipelineStatistics{Arithmetic: true, MaxSMT: append([]MaxSMTStatistics(nil), backend.MaxSMT...), Objectives: uint32(len(p.objectives)), Hard: uint32(len(p.hard))}
	for i := range backend.Values {
		source, value := &p.objectives[i], &backend.Values[i]
		// The inf_eps values move to the pipeline result; backend.Close no
		// longer sees them.
		target := ObjectivePipelineValue{SourceKind: source.sourceKind, Soft: value.Soft, Arithmetic: value.Arithmetic, ModelValue: value.ArithmeticModel, ModelValid: value.ArithmeticModelValid, Interval: value.ArithmeticInterval, OpenBounds: value.ArithmeticOpenBounds}
		value.Arithmetic.Finite, value.ArithmeticModel.Finite = simplex.ZeroInfinitesimal(), simplex.ZeroInfinitesimal()
		target.Exact = numeral.NewRational()
		if value.Soft {
			if target.Exact, err = source.restore(q, &value.SoftCost); err != nil {
				return nil, errors.Join(err, q.Del(&target.Exact), target.Arithmetic.Del(q), target.ModelValue.Del(q), result.Close())
			}
		}
		result.Values[i] = target
	}
	return result, nil
}
