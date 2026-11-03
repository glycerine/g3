package fp

// Source: dl_compiler.cpp compile_preds_init/compile_dependent_rules and
// dl_instruction.cpp instr_union/while_loop. This first widening consumer
// admits singleton recursive strata with one positive self-tail per rule.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type boundIntervalWideningRule struct {
	index int
	head  ast.ASTHandle
	tails []ast.ASTHandle
}

type boundIntervalWideningStratum struct {
	head      ast.ASTHandle
	initial   []int
	recursive []int
	all       []int
}

func (a *boundIntervalDatalogActivation) wideningRule(index int, rule *base.Rule) (_ boundIntervalWideningRule, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	plan := boundIntervalWideningRule{index: index}
	var err error
	plan.head, err = rule.HeadDeclaration()
	if err != nil || a.entries[plan.head] == nil {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	positive, err := rule.PositiveTailSize()
	if err != nil || positive > 2 {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || uninterpreted != positive {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	total, err := rule.TailSize()
	if err != nil || positive == 0 && total != 0 {
		return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	for i := 0; i < positive; i++ {
		tail, err := rule.TailDeclaration(i)
		if err != nil || a.entries[tail] == nil {
			return plan, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
		plan.tails = append(plan.tails, tail)
	}
	return plan, nil
}

func (a *boundIntervalDatalogActivation) wideningPlan() ([]boundIntervalWideningStratum, error) {
	count, err := a.rules.Len()
	if err != nil {
		return nil, err
	}
	byHead := make(map[ast.ASTHandle][]boundIntervalWideningRule)
	for i := 0; i < count; i++ {
		if err = a.checkpoint(); err != nil {
			return nil, err
		}
		rule, err := a.rules.Rule(i)
		if err != nil {
			return nil, err
		}
		plan, err := a.wideningRule(i, rule)
		if err != nil {
			return nil, err
		}
		byHead[plan.head] = append(byHead[plan.head], plan)
	}
	// Source saturation closes the program before reading its dependency-first
	// strata. The activation borrows this same engine-owned rule set.
	finalized, err := a.rules.IsFinalized()
	if err != nil {
		return nil, err
	}
	if !finalized {
		valid, err := a.rules.Finalize()
		if err != nil || !valid {
			return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
	}
	strata, err := a.rules.Strata()
	if err != nil {
		return nil, err
	}
	result := make([]boundIntervalWideningStratum, 0, len(strata))
	for _, predicates := range strata {
		if len(predicates) == 0 {
			continue
		}
		if len(predicates) != 1 {
			return nil, ErrBoundIntervalDatalogFragment
		}
		head := predicates[0]
		plan := boundIntervalWideningStratum{head: head}
		for _, rule := range byHead[head] {
			plan.all = append(plan.all, rule.index)
			if len(rule.tails) == 0 {
				continue // Ground rules have already initialized their targets.
			}
			recursive := false
			for _, tail := range rule.tails {
				recursive = recursive || tail == head
			}
			if recursive {
				if len(rule.tails) != 1 {
					return nil, ErrBoundIntervalDatalogFragment
				}
				plan.recursive = append(plan.recursive, rule.index)
			} else {
				plan.initial = append(plan.initial, rule.index)
			}
		}
		result = append(result, plan)
	}
	return result, nil
}

// All-or-nothing compilation runs every head rule in each loop, including
// ground rules. Construct the ground relation independently before widening
// it into the target, just as the compiler assembles a fresh head register.
func (a *boundIntervalDatalogActivation) evaluateWideningRule(index int, head ast.ASTHandle, inputs, products map[ast.ASTHandle]*rel.IntervalBoundProduct, delta *rel.IntervalBoundProduct) error {
	rule, err := a.rules.Rule(index)
	if err != nil {
		return err
	}
	tails, err := rule.TailSize()
	if err != nil {
		return errors.Join(err, rule.Close())
	}
	if tails != 0 {
		_, err = a.evaluatePositiveRuleInto(rule, inputs, products, delta, true)
		return err
	}
	entry := a.entries[head]
	source, selected, err := rel.NewSelectedIntervalBoundProduct(a.manager, entry.sorts, entry.kinds, true)
	if err != nil || !selected {
		return errors.Join(err, ErrBoundIntervalDatalogFragment, rule.Close())
	}
	if err = a.applyFactRule(rule, map[ast.ASTHandle]*rel.IntervalBoundProduct{head: source}); err != nil {
		return err
	}
	_, err = products[head].Union(source, delta, true)
	return err
}

func (a *boundIntervalDatalogActivation) saturateWithWidening(products map[ast.ASTHandle]*rel.IntervalBoundProduct, allOrNothing bool) error {
	plan, err := a.wideningPlan()
	if err != nil {
		return err
	}
	for _, stratum := range plan {
		for _, index := range stratum.initial {
			if err = a.checkpoint(); err != nil {
				return err
			}
			rule, err := a.rules.Rule(index)
			if err != nil {
				return err
			}
			if _, err = a.evaluatePositiveRule(rule, products); err != nil {
				return err
			}
		}
		if len(stratum.recursive) == 0 {
			continue
		}
		current, err := products[stratum.head].Clone()
		if err != nil {
			return err
		}
		loop := stratum.recursive
		if allOrNothing {
			loop = stratum.all
		}
		for current != nil && !current.Empty() {
			var next *rel.IntervalBoundProduct
			inputs := products
			if !allOrNothing {
				inputs = map[ast.ASTHandle]*rel.IntervalBoundProduct{stratum.head: current}
			}
			for _, index := range loop {
				if err = a.checkpoint(); err != nil {
					return err
				}
				if next == nil {
					entry := a.entries[stratum.head]
					var selected bool
					next, selected, err = rel.NewSelectedIntervalBoundProduct(a.manager, entry.sorts, entry.kinds, true)
					if err != nil || !selected {
						return errors.Join(err, ErrBoundIntervalDatalogFragment)
					}
				}
				if err = a.evaluateWideningRule(index, stratum.head, inputs, products, next); err != nil {
					return err
				}
				// instr_union clears an empty delta register after EACH rule.
				// Its hidden nonempty component must not join a later rule's
				// different component and spuriously keep the loop running.
				if next.Empty() {
					next = nil
				}
			}
			current = next
		}
	}
	return nil
}
