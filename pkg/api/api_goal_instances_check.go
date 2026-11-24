package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// A satisfying ground model can omit required instances of the original
// universals. Only a completed UNSAT check discharges the caller's query.
func (p *orderProofState) proveGoalInstances(parent *ast.ASTManager, assertions []ast.ASTHandle, maxSteps uint32, narrowWitnesses bool) (proved bool, err error) {
	plan, witness, err := planGroundGoals(parent, assertions, narrowWitnesses)
	if err != nil {
		return false, err
	}
	if plan != nil {
		proved, err := p.checkProofPlan(parent, plan, min(maxSteps, 240_000))
		if err != nil || proved || witness == nil {
			return proved, err
		}
	}
	if witness == nil {
		witness, err = planWitnessInstances(parent, assertions)
	}
	if err != nil || witness == nil {
		return false, err
	}
	// The source can already answer the next Check cheaply after preprocessing
	// an assertion to false. Leave that decision in the incremental solver.
	if incremental, ok := p.source.solver.(interface{ HasInconsistentAssertions() bool }); ok && incremental.HasInconsistentAssertions() {
		return false, nil
	}
	if len(witness.sorts) == 1 {
		small, err := p.selectWitnessPremises(parent, witness)
		if err != nil {
			return false, err
		}
		if small != nil {
			yes, err := p.checkProofPlan(parent, small, min(maxSteps, 500_000))
			if yes || err != nil || witness.selectedOnly {
				return yes, err
			}
			// A wider, one-round plan would repeat the same tuple search
			// on the full base. Leave an inconclusive goal to Check.
			if witness.rounds == 1 && witness.width > 3 {
				return false, nil
			}
		}
	}
	if witness.selectedOnly {
		return false, nil
	}
	return p.checkProofPlan(parent, witness, min(maxSteps, 750_000))
}

type privateProofPlan interface {
	assertIn(parent, target *ast.ASTManager, child *APISimpleSolver) (bool, error)
}

func (p *orderProofState) checkProofPlan(parent *ast.ASTManager, plan privateProofPlan, maxSteps uint32) (proved bool, err error) {
	reuse := p.plans.enabled && !ast.ASTSourceDebug && !diagnostic.TraceCompiled() &&
		parent.TraceStream == nil && !parent.Budget().InjectionArmed() &&
		!parent.NumeralService().RuntimeOwner().AllocationLimited()
	aux, err := p.plans.acquire(parent, p.source, reuse)
	if err != nil {
		return false, err
	}
	defer func() {
		if reuse && err == nil && !parent.ResourceLimit().CancelFlag() {
			// Reset drops the entire solver. The next plan translates and
			// asserts its own terms into a fresh solver in this private manager.
			if err = aux.child.Reset(); err == nil {
				return
			}
		}
		err = errors.Join(err, p.plans.close())
		if err != nil {
			proved = false
		}
	}()
	m, err := aux.ctx.Manager()
	if err != nil {
		return false, err
	}
	parentLimit, limit := parent.ResourceLimit(), m.ResourceLimit()
	limit.RunSignals = parentLimit.RunSignals
	defer func() { limit.RunSignals = nil }()
	if err = limit.Push(maxSteps); err != nil {
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
	if ok, err := assertTranslatedPremises(translation, m, child, p.ground); err != nil || !ok {
		return false, err
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

func assertTranslatedPremises(translation *ast.ASTTranslation, m *ast.ASTManager, child *APISimpleSolver, roots []ast.ASTHandle) (bool, error) {
	for _, h := range roots {
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
	return true, nil
}
