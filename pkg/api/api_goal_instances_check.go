package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// A satisfying ground model can omit required instances of the original
// universals. Only a completed UNSAT check discharges the caller's query.
func (p *orderProofState) proveGoalInstances(parent *ast.ASTManager, assertions []ast.ASTHandle, maxSteps uint32) (proved bool, err error) {
	plan, err := planGoalInstances(parent, assertions)
	if err != nil || plan == nil {
		return false, err
	}
	aux := &orderProofState{source: p.source}
	defer func() {
		if e := aux.release(); e != nil {
			p.pendingCondition = aux
			proved, err = false, errors.Join(err, e)
		}
	}()
	if err = aux.start(parent); err != nil {
		return false, err
	}
	m, err := aux.ctx.Manager()
	if err != nil {
		return false, err
	}
	parentLimit, limit := parent.ResourceLimit(), m.ResourceLimit()
	limit.RunSignals = parentLimit.RunSignals
	defer func() { limit.RunSignals = nil }()
	if err = limit.Push(min(maxSteps, 240_000)); err != nil {
		return false, err
	}
	defer limit.Pop()
	if err = parentLimit.PushChild(limit); err != nil {
		return false, err
	}
	defer parentLimit.PopChild()
	if parentLimit.CancelFlag() {
		return false, nil
	}
	complete, err := plan.assertIn(parent, m, aux.child)
	if !limit.IncOffset(0) || parentLimit.CancelFlag() {
		return false, nil
	}
	if err != nil || !complete {
		return false, err
	}
	answer, err := aux.child.Check()
	if !limit.IncOffset(0) || parentLimit.CancelFlag() {
		return false, nil
	}
	return err == nil && answer == lbool.False, err
}

func (p *goalInstancePlan) assertIn(parent, m *ast.ASTManager, child *APISimpleSolver) (complete bool, err error) {
	translation, err := ast.NewASTTranslation(parent, m, true)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, translation.Close()) }()
	goal, err := translation.Translate(p.goal)
	if err != nil {
		return false, err
	}
	// This reference also owns every translated witness while the ground
	// premises are installed. Assert the goal last to retain premise order.
	if err = m.IncRef(goal); err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DecRef(goal)) }()
	for _, h := range p.ground {
		if !m.ResourceLimit().Inc() {
			return false, nil
		}
		value, err := translation.Translate(h)
		if err != nil {
			return false, err
		}
		if err = m.IncRef(value); err != nil {
			return false, err
		}
		err = child.Assert(value)
		if err = errors.Join(err, m.DecRef(value)); err != nil {
			return false, err
		}
	}

	sub, err := rewriter.NewVarSubstitution(m, true)
	if sub != nil {
		defer func() { err = errors.Join(err, sub.Close()) }()
	}
	if err != nil {
		return false, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	translated := make(map[ast.ASTHandle]ast.ASTHandle)
	var args []ast.ASTHandle
	start := 0
	for _, rule := range p.rules {
		if !m.ResourceLimit().Inc() {
			return false, nil
		}
		args = args[:0]
		for _, witness := range p.bindings[start:rule.end] {
			value, ok := translated[witness]
			if !ok {
				value, err = translation.Translate(witness)
				if err != nil {
					return false, err
				}
				translated[witness] = value
			}
			args = append(args, value)
		}
		start = rule.end
		body, err := translation.Translate(rule.body)
		if err != nil {
			return false, err
		}
		if err = m.IncRef(body); err != nil {
			return false, err
		}
		err = sub.Substitute(body, args, out)
		if err = errors.Join(err, m.DecRef(body)); err != nil {
			return false, err
		}
		formula, err := m.RefGet(out)
		if err != nil {
			return false, err
		}
		ground, err := m.IsGround(formula)
		if err != nil {
			return false, err
		}
		if ground {
			if err = child.Assert(formula); err != nil {
				return false, err
			}
		}
	}
	err = child.Assert(goal)
	return err == nil, err
}
