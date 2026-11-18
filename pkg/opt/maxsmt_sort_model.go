package opt

// Source: src/opt/sortmax.cpp update_assignment/is_true.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// updateResult is maxsmt_solver_base::init / sortmax::update_assignment:
// both evaluate with m_model->eval, which does not complete the model.
func (e *sortMaxEngine) updateResult(result *ArithmeticResult) error {
	cost, satisfied, assignment, err := e.solver.evaluateSoftAssignment(e.objective, result.Model, false)
	if err == nil {
		err = setSoftObjectiveValue(result, e.index, &cost, &satisfied, assignment)
	}
	q := e.solver.rationalManager()
	return errors.Join(err, q.Del(&cost), q.Del(&satisfied))
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
			count += int(e.terms[i].units)
		}
	}
	return count, nil
}

// outputIsTrue is sortmax::is_true: model::eval without completion.
func (e *sortMaxEngine) outputIsTrue(md *model.ModelCore, output ast.ASTHandle) (bool, error) {
	return wmaxModelIsTrue(e.solver, md, output)
}

func (e *sortMaxEngine) setBounds(result *ArithmeticResult, lower, upper *numeral.Rational) error {
	o := newRatOps(e.solver.manager)
	value := &result.Values[e.index]
	o.set(&value.SoftCost, lower)
	o.set(&value.SoftLower, lower)
	o.set(&value.SoftUpper, upper)
	value.SoftBoundsValid = true
	total := o.clone(&e.objective.softOffset)
	for i := range e.objective.softWeights {
		o.add(&total, &total, &e.objective.softWeights[i])
	}
	o.sub(&value.SoftSatisfied, &total, lower)
	o.del(&total)
	value.Soft = true
	return o.err
}
