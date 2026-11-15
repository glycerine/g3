package opt

// Source: src/opt/sortmax.cpp update_assignment/is_true.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// updateResult is maxsmt_solver_base::init / sortmax::update_assignment:
// both evaluate with m_model->eval, which does not complete the model.
func (e *sortMaxEngine) updateResult(result *ArithmeticResult) error {
	cost, satisfied, assignment, err := e.solver.evaluateSoftAssignment(e.objective, result.Model, false)
	if err != nil {
		return err
	}
	setSoftObjectiveValue(result, e.index, cost, satisfied, assignment)
	return nil
}

// countSatisfiedInputs is the seeding loop of sortmax::operator(): each
// soft key that m_model->eval (no model completion) makes true asserts one
// network output per unit of its weight.
func (e *sortMaxEngine) countSatisfiedInputs(md *model.ModelCore) (int, error) {
	count := 0
	for i := range e.terms {
		isTrue, err := wmaxModelIsTrue(e.solver, md, e.terms[i].expression)
		if err != nil {
			return 0, err
		}
		if isTrue {
			count += int(e.terms[i].weight.Int64())
		}
	}
	return count, nil
}

// outputIsTrue is sortmax::is_true: model::eval without completion.
func (e *sortMaxEngine) outputIsTrue(md *model.ModelCore, output ast.ASTHandle) (bool, error) {
	return wmaxModelIsTrue(e.solver, md, output)
}

func (e *sortMaxEngine) setBounds(result *ArithmeticResult, lower, upper *big.Rat) {
	value := &result.Values[e.index]
	value.SoftCost.Set(lower)
	value.SoftLower.Set(lower)
	value.SoftUpper.Set(upper)
	value.SoftBoundsValid = true
	total := new(big.Rat).Set(&e.objective.softOffset)
	for _, weight := range e.objective.softWeights {
		total.Add(total, weight)
	}
	value.SoftSatisfied.Sub(total, lower)
	value.Soft = true
}
