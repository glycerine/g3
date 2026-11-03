package fp

// Source: dl_compiler.cpp lowers a positive rule by cloning its tail,
// applying one compiler-produced interpreted condition, projecting unused
// columns, and renaming the remaining columns into head order. This typed
// slice covers one aligned exact-pair tail with unique, safe head variables.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (a *boundIntervalDatalogActivation) remapBoundIntervalCondition(condition ast.ASTHandle, variables map[uint32]uint32, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	used, err := ast.NewASTUsedVars(a.manager)
	if err != nil {
		return result, err
	}
	if err = used.Collect(condition); err != nil {
		return result, err
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		if used.Contains(variable).IsNull() {
			continue
		}
		if _, found := variables[variable]; !found {
			return result, ErrBoundIntervalDatalogFragment
		}
	}
	max := uint32(0)
	for variable := range variables {
		if variable >= max {
			max = variable + 1
		}
	}
	bindings := make([]ast.ASTHandle, max)
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, a.manager.DelRef(&roots[i]))
		}
	}()
	for variable, column := range variables {
		value, cause := a.manager.MkVar(column, sorts[column])
		if cause != nil {
			return result, cause
		}
		root, cause := a.manager.NewRef(value)
		if cause != nil {
			return result, cause
		}
		roots = append(roots, root)
		bindings[variable] = value
	}
	substitution, err := astrewriter.NewVarSubstitution(a.manager, false)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	result, err = a.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = substitution.Substitute(condition, bindings, result); err != nil {
		resultErr = errors.Join(err, a.manager.DelRef(&result))
		result = ast.ASTRef{}
	}
	return result, resultErr
}

func (a *boundIntervalDatalogActivation) positiveRuleHead(rule *base.Rule) (head ast.ASTHandle, formula ast.ASTRef, resultErr error) {
	formula, err := rule.ToFormula()
	if err != nil {
		return head, formula, err
	}
	head, err = a.manager.RefGet(formula)
	if err != nil {
		return head, formula, err
	}
	if kind, cause := a.manager.GetKind(head); cause != nil {
		return head, formula, cause
	} else if kind == ast.ASTQuantifier {
		head, err = a.manager.GetQuantifierBody(head)
		if err != nil {
			return head, formula, err
		}
	}
	if op, cause := a.manager.GetDeclKind(head); cause != nil {
		return head, formula, cause
	} else if op == ast.ASTOpImplies {
		head, err = a.manager.GetArg(head, 1)
	}
	return head, formula, err
}

func (a *boundIntervalDatalogActivation) evaluatePositiveRule(rule *base.Rule, products map[ast.ASTHandle]*rel.IntervalBoundProduct) (changed bool, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	positive, err := rule.PositiveTailSize()
	if err != nil || positive != 1 {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || uninterpreted != positive {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	tail, negated, err := rule.Tail(0)
	if err != nil || negated {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	tailDeclaration, err := a.manager.GetDecl(tail)
	if err != nil || products[tailDeclaration] == nil {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	working, err := products[tailDeclaration].Clone()
	if err != nil {
		return false, err
	}
	workingSorts := append([]ast.ASTHandle(nil), a.entries[tailDeclaration].sorts...)
	variables := make(map[uint32]uint32, len(workingSorts))
	arguments, err := a.manager.GetNumArgs(tail)
	if err != nil || int(arguments) != len(workingSorts) {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	for column := uint32(0); column < arguments; column++ {
		argument, cause := a.manager.GetArg(tail, column)
		if cause != nil {
			return false, cause
		}
		kind, cause := a.manager.GetKind(argument)
		if cause != nil || kind != ast.ASTVar {
			return false, errors.Join(cause, ErrBoundIntervalDatalogFragment)
		}
		variable, cause := a.manager.GetVarIndex(argument)
		if cause != nil {
			return false, cause
		}
		if _, repeated := variables[variable]; repeated {
			return false, ErrBoundIntervalDatalogFragment
		}
		variables[variable] = column
	}
	tailSize, err := rule.TailSize()
	if err != nil || tailSize-uninterpreted > 1 {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	var condition ast.ASTRef
	if tailSize > uninterpreted {
		raw, conditionNegated, cause := rule.Tail(uninterpreted)
		if cause != nil || conditionNegated {
			return false, errors.Join(cause, ErrBoundIntervalDatalogFragment)
		}
		condition, cause = a.remapBoundIntervalCondition(raw, variables, workingSorts)
		if cause != nil {
			return false, cause
		}
		defer func() { resultErr = errors.Join(resultErr, a.manager.DelRef(&condition)) }()
	}
	head, formula, err := a.positiveRuleHead(rule)
	if formula != (ast.ASTRef{}) {
		defer func() { resultErr = errors.Join(resultErr, a.manager.DelRef(&formula)) }()
	}
	if err != nil {
		return false, err
	}
	headDeclaration, err := a.manager.GetDecl(head)
	if err != nil || products[headDeclaration] == nil {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	headCount, err := a.manager.GetNumArgs(head)
	if err != nil || int(headCount) != len(a.entries[headDeclaration].sorts) {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	desired := make([]uint32, headCount)
	seen := make(map[uint32]bool, headCount)
	for i := uint32(0); i < headCount; i++ {
		argument, cause := a.manager.GetArg(head, i)
		if cause != nil {
			return false, cause
		}
		kind, cause := a.manager.GetKind(argument)
		if cause != nil || kind != ast.ASTVar {
			return false, errors.Join(cause, ErrBoundIntervalDatalogFragment)
		}
		variable, cause := a.manager.GetVarIndex(argument)
		column, found := variables[variable]
		if cause != nil || !found || seen[column] {
			return false, errors.Join(cause, ErrBoundIntervalDatalogFragment)
		}
		seen[column] = true
		desired[i] = column
	}
	selected := append([]uint32(nil), desired...)
	sort.Slice(selected, func(i, j int) bool { return selected[i] < selected[j] })
	removed := make([]uint32, 0, len(workingSorts)-len(selected))
	for column, at := uint32(0), 0; int(column) < len(workingSorts); column++ {
		if at < len(selected) && selected[at] == column {
			at++
		} else {
			removed = append(removed, column)
		}
	}
	if condition != (ast.ASTRef{}) {
		conditionHandle, cause := a.manager.RefGet(condition)
		if cause != nil {
			return false, cause
		}
		if len(removed) != 0 {
			working, err = working.FilterInterpretedAndProject(a.manager, conditionHandle, removed)
		} else {
			err = working.FilterInterpreted(a.manager, conditionHandle)
		}
	} else if len(removed) != 0 {
		working, err = working.Project(removed)
	}
	if err != nil {
		return false, err
	}
	current := append([]uint32(nil), selected...)
	for destination, want := range desired {
		position := destination
		for position < len(current) && current[position] != want {
			position++
		}
		if position == len(current) {
			return false, ErrBoundIntervalDatalogFragment
		}
		if position != destination {
			working, err = working.Rename([]uint32{uint32(position), uint32(destination)})
			if err != nil {
				return false, err
			}
			current[position], current[destination] = current[destination], current[position]
		}
	}
	return products[headDeclaration].Union(working, nil, false)
}
