package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model rule tracing.

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

// QuantifiedLinearTrace follows model-selected rules from the query to a fact.
// The caller owns each returned rule reference.
func (c *Compiler) QuantifiedLinearTrace(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result []*base.Rule, resultErr error) {
	steps, err := c.quantifiedLinearSteps(rules, query, width, md, services)
	if err != nil {
		return nil, err
	}
	result = make([]*base.Rule, len(steps))
	for i := range steps {
		result[i] = steps[i].rule
	}
	return result, nil
}

type quantifiedLinearStep struct {
	rule  *base.Rule
	index uint32
	level uint32
}

func (c *Compiler) quantifiedLinearSteps(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result []quantifiedLinearStep, resultErr error) {
	return c.quantifiedLinearStepsVisit(rules, query, width, md, services, nil)
}

func (c *Compiler) quantifiedLinearStepsVisit(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig, visit func(quantifiedLinearStep) error) (result []quantifiedLinearStep, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	owner := &quantifiedLinearTraceOwner{manager: m}
	defer func() {
		result, resultErr = finishQuantifiedLinearTrace(owner, resultErr)
	}()
	q, err := newQuantifiedLinear(m, width)
	owner.terms = q
	if err != nil {
		return nil, err
	}
	// model::eval constructs a fresh evaluator for every call and explicitly
	// disables completion, independently of model_evaluator parameters.
	evaluate := func(term ast.ASTHandle) (ast.ASTRef, error) {
		evaluator, err := model.NewModelEvaluator(md, services)
		owner.evaluator = evaluator
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = evaluator.SetModelCompletion(false); err != nil {
			return ast.ASTRef{}, err
		}
		ref, err := evaluator.EvaluateValue(term)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = evaluator.Close(); err != nil {
			return ast.ASTRef{}, disposeASTRefs(m, []ast.ASTRef{ref}, err)
		}
		owner.evaluator = nil
		return ref, nil
	}
	name, err := m.Symbols.Intern("T")
	if err != nil {
		return nil, err
	}
	index, err := q.hold(m.MkConst(name, q.index))
	if err != nil {
		return nil, err
	}
	indexRef, err := evaluate(index)
	if err != nil {
		return nil, err
	}
	value, err := m.RefGet(indexRef)
	if err != nil {
		return nil, disposeASTRefs(m, []ast.ASTRef{indexRef}, err)
	}
	num, bits, matched, err := m.BitVectorNumeralBigInt(value)
	err = disposeASTRefs(m, []ast.ASTRef{indexRef}, err)
	if err != nil {
		return nil, err
	}
	if !matched || bits != width || !num.IsUint64() || num.Uint64() > uint64(^uint32(0)) {
		return nil, ast.ErrASTDomain
	}
	level := uint32(num.Uint64())
	predicate := query
	for {
		canceled, e := m.Canceled()
		if e != nil {
			return result, e
		}
		if canceled {
			return result, fmt.Errorf("bmc: %s", m.ResourceLimit().CancelMessage())
		}
		levelExpr, e := q.hold(m.BitVectorNumeralRat(new(big.Rat).SetUint64(uint64(level)), width))
		if e != nil {
			return result, e
		}
		owner.group, e = rules.PredicateRules(predicate)
		if e != nil {
			return result, e
		}
		var selectedIndex uint32
		for i, rule := range owner.group {
			atom, cause := q.rule(predicate, uint32(i))
			if cause == nil {
				var decl ast.ASTHandle
				decl, cause = m.GetDecl(atom)
				if cause == nil {
					atom, cause = q.hold(m.MkApp(decl, []ast.ASTHandle{levelExpr}))
				}
			}
			if cause == nil {
				var evaluated ast.ASTRef
				evaluated, cause = evaluate(atom)
				if cause == nil {
					var h ast.ASTHandle
					h, cause = m.RefGet(evaluated)
					if cause == nil && m.IsTrue(h) {
						owner.selected, cause = rule.Retain()
						selectedIndex = uint32(i)
					}
					cause = disposeASTRefs(m, []ast.ASTRef{evaluated}, cause)
				}
			}
			if cause != nil {
				e = cause
				break
			}
			if owner.selected != nil {
				break
			}
		}
		if e != nil {
			return result, e
		}
		if e = owner.closeGroup(); e != nil {
			return result, e
		}
		if owner.selected == nil {
			return result, fmt.Errorf("bmc: model has no selected quantified rule at level %d", level)
		}
		selected := owner.selected
		owner.steps = append(owner.steps, quantifiedLinearStep{rule: selected, index: selectedIndex, level: level})
		owner.selected = nil
		if visit != nil {
			if e = visit(owner.steps[len(owner.steps)-1]); e != nil {
				return result, e
			}
		}
		tails, e := selected.UninterpretedTailSize()
		if e != nil {
			return result, e
		}
		if level == 0 {
			if tails != 0 {
				return result, ast.ErrASTDomain
			}
			return result, nil
		}
		if tails != 1 {
			return result, ast.ErrASTDomain
		}
		predicate, e = selected.TailDeclaration(0)
		if e != nil {
			return result, e
		}
		level--
	}
}
