package rel

// Source: dl_compiler.cpp order_rules/compile_strats, specialized to positive
// acyclic rules over the registered finite/explanation product. Recursive
// dependency components still require source delta/fixpoint lowering.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// CompileExplanationRulesWithGuard compiles an acyclic transformed explanation
// rule set, including its finite output bridges, into dependency-ordered
// execution and saturation-marking termination blocks. Both blocks transfer to
// the caller. Unsupported rules or cycles publish neither block.
func CompileExplanationRulesWithGuard(relations *RelationManager, set *base.RuleSet, guard *rewriter.TheoryRewriter) (execution, termination *InstructionBlock, resultErr error) {
	if err := relations.open(); err != nil {
		return nil, nil, err
	}
	if set == nil {
		return nil, nil, ErrCompilerUnsupported
	}
	count, err := set.Len()
	if err != nil {
		return nil, nil, err
	}
	var rules []*base.Rule
	defer func() {
		for _, rule := range rules {
			resultErr = errors.Join(resultErr, rule.Close())
		}
		if resultErr != nil {
			if execution != nil {
				resultErr = errors.Join(resultErr, execution.Close())
			}
			if termination != nil {
				resultErr = errors.Join(resultErr, termination.Close())
			}
			execution, termination = nil, nil
		}
	}()
	heads := make([]ast.ASTHandle, 0)
	byHead := make(map[ast.ASTHandle][]int)
	dependencies := make(map[ast.ASTHandle][]ast.ASTHandle)
	for index := 0; index < count; index++ {
		rule, err := set.Rule(index)
		if err != nil {
			return nil, nil, err
		}
		rules = append(rules, rule)
		plan, err := planExplanationRule(relations, rule)
		if err != nil {
			return nil, nil, err
		}
		if len(plan.conditions) != 0 && guard == nil {
			return nil, nil, ErrCompilerUnsupported
		}
		if _, found := byHead[plan.head]; !found {
			heads = append(heads, plan.head)
		}
		byHead[plan.head] = append(byHead[plan.head], index)
		if !plan.tail.IsNull() {
			dependencies[plan.head] = append(dependencies[plan.head], plan.tail)
		}
		for _, join := range plan.joins {
			dependencies[plan.head] = append(dependencies[plan.head], join.predicate)
		}
	}
	states := make(map[ast.ASTHandle]uint8)
	ordered := make([]ast.ASTHandle, 0, len(heads))
	var visit func(ast.ASTHandle) error
	visit = func(head ast.ASTHandle) error {
		if states[head] == 1 {
			return ErrCompilerUnsupported
		}
		if states[head] == 2 {
			return nil
		}
		states[head] = 1
		for _, tail := range dependencies[head] {
			if _, defined := byHead[tail]; defined {
				if err := visit(tail); err != nil {
					return err
				}
			}
		}
		states[head] = 2
		ordered = append(ordered, head)
		return nil
	}
	for _, head := range heads {
		if err := visit(head); err != nil {
			return nil, nil, err
		}
	}
	execution, termination = NewInstructionBlock(), NewInstructionBlock()
	for _, head := range ordered {
		for _, index := range byHead[head] {
			block, err := CompileExplanationRuleWithGuard(relations, rules[index], guard)
			if err != nil {
				return execution, termination, err
			}
			// Each rule stores its head and empties its temporaries, so the next
			// rule can reuse its local register numbers in the same context.
			execution.instructions = append(execution.instructions, block.instructions...)
			block.instructions = nil
			if err := block.Close(); err != nil {
				return execution, termination, err
			}
		}
		mark, err := NewMarkSaturatedInstruction(relations, head)
		if err != nil {
			return execution, termination, err
		}
		if err := termination.Append(mark); err != nil {
			return execution, termination, errors.Join(err, mark.Close())
		}
	}
	return execution, termination, nil
}
