package pdr

// Source: datalog::rule_unifier and model_search::get_proof_trace rule
// reduction in src/muz/transforms/dl_mk_rule_inliner.cpp and
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astsubst "github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
)

func proofRuleVariables(manager *ast.ASTManager, rule *base.Rule) ([]ast.ASTHandle, error) {
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	if err = used.Process(head); err != nil {
		return nil, err
	}
	count, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		tail, _, x := rule.Tail(index)
		if x != nil {
			return nil, x
		}
		if x = used.Process(tail); x != nil {
			return nil, x
		}
	}
	variables := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	for index := range variables {
		variables[index] = used.Contains(uint32(index))
	}
	return variables, nil
}

func (c *Context) resolveProofRules(rules *base.RuleManager, target, source *base.Rule, selected int, tailMap *[]int) (resolved *base.Rule, targetSubstitution, sourceSubstitution []ast.ASTHandle, temporaries []ast.ASTRef, resultErr error) {
	tail, negated, err := target.Tail(selected)
	if err != nil || negated {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, nil, nil, nil, err
	}
	sourceHead, err := source.Head()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	targetVariables, err := proofRuleVariables(c.manager, target)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sourceVariables, err := proofRuleVariables(c.manager, source)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	width := len(targetVariables)
	if len(sourceVariables) > width {
		width = len(sourceVariables)
	}
	substitution, err := astsubst.New(c.manager)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	unifier, err := astsubst.NewUnifier(c.manager)
	if err != nil {
		return nil, nil, nil, nil, errors.Join(err, substitution.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, unifier.Close(), substitution.Close()) }()
	if err = substitution.Reserve(2, uint32(width)); err != nil {
		return nil, nil, nil, nil, err
	}
	unified, err := unifier.UnifyPair(tail, sourceHead, substitution, true)
	if err != nil || !unified {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, nil, nil, nil, err
	}
	deltas := []uint32{0, uint32(width)}
	apply := func(expression ast.ASTHandle, bank uint32) (ast.ASTHandle, error) {
		reference, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return ast.ASTHandle{}, x
		}
		temporaries = append(temporaries, reference)
		if x = substitution.Apply(2, deltas, astsubst.NewExprOffset(expression, bank), reference); x != nil {
			return ast.ASTHandle{}, x
		}
		return c.manager.RefGet(reference)
	}
	buildSubstitution := func(sorts []ast.ASTHandle, bank uint32) ([]ast.ASTHandle, error) {
		result := make([]ast.ASTHandle, len(sorts))
		for index, sort := range sorts {
			if sort.IsNull() {
				continue
			}
			variable, x := c.manager.MkVar(uint32(index), sort)
			if x != nil {
				return nil, x
			}
			result[index], x = apply(variable, bank)
			if x != nil {
				return nil, x
			}
		}
		return result, nil
	}
	targetSubstitution, err = buildSubstitution(targetVariables, 0)
	if err != nil {
		return nil, nil, nil, temporaries, err
	}
	sourceSubstitution, err = buildSubstitution(sourceVariables, 1)
	if err != nil {
		return nil, nil, nil, temporaries, err
	}
	targetHead, err := target.Head()
	if err != nil {
		return nil, nil, nil, temporaries, err
	}
	newHead, err := apply(targetHead, 0)
	if err != nil {
		return nil, nil, nil, temporaries, err
	}
	tails := make([]ast.ASTHandle, 0)
	negatives := make([]bool, 0)
	appendRule := func(rule *base.Rule, bank uint32, skip int) error {
		count, x := rule.TailSize()
		if x != nil {
			return x
		}
		for index := 0; index < count; index++ {
			if index == skip {
				continue
			}
			expression, negative, y := rule.Tail(index)
			if y != nil {
				return y
			}
			expression, y = apply(expression, bank)
			if y != nil {
				return y
			}
			duplicate := false
			for prior := range tails {
				if tails[prior] == expression && negatives[prior] == negative {
					duplicate = true
					break
				}
			}
			if !duplicate {
				tails = append(tails, expression)
				negatives = append(negatives, negative)
			}
		}
		return nil
	}
	if err = appendRule(target, 0, selected); err != nil {
		return nil, nil, nil, temporaries, err
	}
	if err = appendRule(source, 1, -1); err != nil {
		return nil, nil, nil, temporaries, err
	}
	targetName, err := target.Name()
	if err != nil {
		return nil, nil, nil, temporaries, err
	}
	sourceName, err := source.Name()
	if err != nil {
		return nil, nil, nil, temporaries, err
	}
	resolved, err = rules.NewRule(newHead, tails, negatives, targetName+";"+sourceName)
	if err != nil {
		return nil, nil, nil, temporaries, fmt.Errorf("pdr: construct reduced proof rule: %w", err)
	}
	count, err := target.UninterpretedTailSize()
	if err != nil {
		return resolved, targetSubstitution, sourceSubstitution, temporaries, err
	}
	mapping := make([]int, count)
	for i := range mapping {
		mapping[i] = -1
		if i == selected {
			continue
		}
		term, _, x := target.Tail(i)
		if x == nil {
			term, x = apply(term, 0)
		}
		if x != nil {
			return resolved, targetSubstitution, sourceSubstitution, temporaries, x
		}
		resolvedTail, _, x := target.Tail(selected)
		if x == nil {
			resolvedTail, x = apply(resolvedTail, 0)
		}
		if x != nil {
			return resolved, targetSubstitution, sourceSubstitution, temporaries, x
		}
		if term == resolvedTail {
			continue
		}
		for j, candidate := range tails {
			if candidate == term {
				mapping[i] = j
				break
			}
		}
		if mapping[i] < 0 {
			return resolved, targetSubstitution, sourceSubstitution, temporaries, fmt.Errorf("pdr: missing reduced antecedent: %w", ast.ErrASTDomain)
		}
	}
	*tailMap = mapping
	return resolved, targetSubstitution, sourceSubstitution, temporaries, resultErr
}

type reducedProofEntry struct {
	rule  *base.Rule
	proof ast.ASTHandle
}

func (c *Context) reducedProofTrace(root *ModelNode) (result ast.ASTRef, resultErr error) {
	rules, err := base.NewRuleManager(c.manager)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, rules.Close()) }()
	for _, declaration := range c.order {
		if err = rules.RegisterPredicate(declaration); err != nil {
			return result, err
		}
	}
	references := make([]ast.ASTRef, 0)
	generated := make([]*base.Rule, 0)
	defer func() {
		resultErr = errors.Join(resultErr, closeASTRefs(c.manager, references))
		for index := len(generated) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, generated[index].Close())
		}
	}()
	cache := make(map[*ModelNode]reducedProofEntry)
	var reduce func(*ModelNode) (reducedProofEntry, error)
	reduce = func(node *ModelNode) (reducedProofEntry, error) {
		if prior, found := cache[node]; found {
			return prior, nil
		}
		selected, x := c.selectedTraceRule(node)
		if x != nil {
			return reducedProofEntry{}, x
		}
		formula, x := selected.rule.ToFormula()
		if x != nil {
			return reducedProofEntry{}, x
		}
		references = append(references, formula)
		formulaHandle, x := c.manager.RefGet(formula)
		if x != nil {
			return reducedProofEntry{}, x
		}
		currentProof, x := c.manager.MkProofAsserted(formulaHandle)
		if x != nil {
			return reducedProofEntry{}, x
		}
		proofReference, x := c.manager.NewRef(currentProof)
		if x != nil {
			return reducedProofEntry{}, x
		}
		references = append(references, proofReference)
		currentRule := selected.rule
		children, x := node.Children()
		if x != nil {
			return reducedProofEntry{}, x
		}
		uninterpreted, x := currentRule.UninterpretedTailSize()
		if x != nil || len(children) != uninterpreted {
			if x == nil {
				x = fmt.Errorf("pdr: trace has %d children for %d predicate tails: %w", len(children), uninterpreted, ast.ErrASTDomain)
			}
			return reducedProofEntry{}, x
		}
		positions := make([]int, len(children))
		for i := range positions {
			positions[i] = i
		}
		for childIndex, child := range children {
			selected := positions[childIndex]
			if selected < 0 {
				// An earlier substitution merged this antecedent with one already resolved.
				continue
			}
			childEntry, y := reduce(child)
			if y != nil {
				return reducedProofEntry{}, fmt.Errorf("pdr: resolve trace child %d: %w", childIndex, y)
			}
			var tailMap []int
			resolved, targetSubst, sourceSubst, temporaryRefs, y := c.resolveProofRules(rules, currentRule, childEntry.rule, selected, &tailMap)
			references = append(references, temporaryRefs...)
			if resolved != nil {
				generated = append(generated, resolved)
			}
			if y != nil {
				return reducedProofEntry{}, fmt.Errorf("pdr: unify trace child %d: %w", childIndex, y)
			}
			conclusion, y := resolved.ToFormula()
			if y != nil {
				return reducedProofEntry{}, y
			}
			references = append(references, conclusion)
			conclusionHandle, y := c.manager.RefGet(conclusion)
			if y != nil {
				return reducedProofEntry{}, y
			}
			currentProof, y = c.manager.MkProofHyperResolve(
				[]ast.ASTHandle{currentProof, childEntry.proof}, conclusionHandle,
				[]ast.ASTProofPosition{{First: uint32(selected + 1), Second: 0}},
				[][]ast.ASTHandle{targetSubst, sourceSubst},
			)
			if y != nil {
				return reducedProofEntry{}, y
			}
			proofReference, y = c.manager.NewRef(currentProof)
			if y != nil {
				return reducedProofEntry{}, y
			}
			references = append(references, proofReference)
			for i, position := range positions {
				if position >= 0 {
					positions[i] = tailMap[position]
				}
			}
			currentRule = resolved
		}
		entry := reducedProofEntry{rule: currentRule, proof: currentProof}
		cache[node] = entry
		return entry, nil
	}
	entry, err := reduce(root)
	if err != nil {
		return result, fmt.Errorf("pdr: reduce proof trace: %w", err)
	}
	return c.manager.NewRef(entry.proof)
}
