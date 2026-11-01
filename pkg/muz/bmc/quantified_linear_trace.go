package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model rule tracing.

import (
	"errors"
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
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	defer func() {
		if resultErr != nil {
			for i := len(result) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, result[i].rule.Close())
			}
			result = nil
		}
	}()
	q, err := newQuantifiedLinear(m, width)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, q.close()) }()
	evaluator, err := model.NewModelEvaluator(md, services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	name, err := m.Symbols.Intern("T")
	if err != nil {
		return nil, err
	}
	index, err := q.hold(m.MkConst(name, q.index))
	if err != nil {
		return nil, err
	}
	indexRef, err := evaluator.EvaluateValue(index)
	if err != nil {
		return nil, err
	}
	value, err := m.RefGet(indexRef)
	if err != nil {
		return nil, errors.Join(err, m.DelRef(&indexRef))
	}
	num, bits, matched, err := m.BitVectorNumeralBigInt(value)
	err = errors.Join(err, m.DelRef(&indexRef))
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
		group, e := rules.PredicateRules(predicate)
		if e != nil {
			return result, e
		}
		var selected *base.Rule
		var selectedIndex uint32
		for i, rule := range group {
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
				evaluated, cause = evaluator.EvaluateValue(atom)
				if cause == nil {
					var h ast.ASTHandle
					h, cause = m.RefGet(evaluated)
					if cause == nil && m.IsTrue(h) {
						selected, cause = rule.Retain()
						selectedIndex = uint32(i)
					}
					cause = errors.Join(cause, m.DelRef(&evaluated))
				}
			}
			if cause != nil {
				e = cause
				break
			}
			if selected != nil {
				break
			}
		}
		for i := len(group) - 1; i >= 0; i-- {
			e = errors.Join(e, group[i].Close())
		}
		if e != nil {
			if selected != nil {
				e = errors.Join(e, selected.Close())
			}
			return result, e
		}
		if selected == nil {
			return result, fmt.Errorf("bmc: model has no selected quantified rule at level %d", level)
		}
		result = append(result, quantifiedLinearStep{rule: selected, index: selectedIndex, level: level})
		tails, e := selected.UninterpretedTailSize()
		if e != nil {
			return result, e
		}
		if tails == 0 {
			return result, nil
		}
		if tails != 1 || level == 0 {
			return result, ast.ErrASTDomain
		}
		predicate, e = selected.TailDeclaration(0)
		if e != nil {
			return result, e
		}
		level--
	}
}
