package rel

// relationFormulaChecker's grounding, substitution, projection and
// equivalence checks (shared by the bound relation formula checks).

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var ErrRelationVerification = errors.New("muz/rel: operation was not verified")

// ground replaces each column variable with the source numeric-symbol constant.
func (r *relationFormulaChecker) ground(formula ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	return r.groundWithNames(formula, false)
}

func (r *relationFormulaChecker) groundWithNames(formula ast.ASTHandle, named bool) (result ast.ASTRef, resultErr error) {
	var refs []ast.ASTRef
	owned := false
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	vars := make([]ast.ASTHandle, len(r.sorts))
	for i, sort := range r.sorts {
		name := symbol.Numerical(uint32(i))
		if named {
			var err error
			name, err = r.manager.Symbols.Intern(fmt.Sprintf("x%d", i))
			if err != nil {
				return result, err
			}
		}
		value, err := r.manager.MkConst(name, sort)
		if err != nil {
			return result, err
		}
		ref, err := r.manager.NewRef(value)
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
		vars[i] = value
	}
	subst, err := rewriter.NewVarSubstitution(r.manager, false)
	if subst != nil {
		defer func() { resultErr = errors.Join(resultErr, subst.Close()) }()
	}
	if err != nil {
		return result, err
	}
	result, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	owned = true
	resultErr = subst.Substitute(formula, vars, result)
	return result, resultErr
}

func (r *relationFormulaChecker) checkEquivalent(left, right ast.ASTHandle) (resultErr error) {
	return VerifyRelationFormulasEquivalent(r.manager, r.config, left, right)
}

func (r *relationFormulaChecker) factFormula(values []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := r.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	conjuncts := make([]ast.ASTHandle, 0, len(values))
	for i, value := range values {
		variable, err := m.MkVar(uint32(i), r.sorts[i])
		if err != nil {
			return result, err
		}
		varRef, err := m.NewRef(variable)
		if err != nil {
			return result, err
		}
		refs = append(refs, varRef)
		eq, err := m.MkEq(variable, value)
		if err != nil {
			return result, err
		}
		eqRef, err := m.NewRef(eq)
		if err != nil {
			return result, err
		}
		refs = append(refs, eqRef)
		conjuncts = append(conjuncts, eq)
	}
	expression, err := m.MkAndArray(conjuncts)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(expression)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (r *relationFormulaChecker) substitute(formula ast.ASTHandle, bindings []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	subst, err := rewriter.NewVarSubstitution(r.manager, false)
	if subst != nil {
		defer func() {
			resultErr = errors.Join(resultErr, subst.Close())
			if resultErr != nil && result != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
				result = ast.ASTRef{}
			}
		}()
	}
	if err != nil {
		return result, err
	}
	result, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	resultErr = subst.Substitute(formula, bindings, result)
	return result, resultErr
}

func (r *relationFormulaChecker) projectFormulaForSorts(formula ast.ASTHandle, sorts []ast.ASTHandle, removed []int) (result ast.ASTRef, resultErr error) {
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	bindings := make([]ast.ASTHandle, len(sorts))
	var bound []ast.ASTHandle
	var names []symbol.Symbol
	removedIndex, remainingIndex := 0, 0
	for i, sort := range sorts {
		index := remainingIndex + len(removed)
		if removedIndex < len(removed) && removed[removedIndex] == i {
			index = removedIndex
			bound = append(bound, sort)
			name, err := r.manager.Symbols.Intern(fmt.Sprintf("x%d", removedIndex))
			if err != nil {
				return result, err
			}
			names = append(names, name)
			removedIndex++
		} else {
			remainingIndex++
		}
		variable, err := r.manager.MkVar(uint32(index), sort)
		if err != nil {
			return result, err
		}
		ref, err := r.manager.NewRef(variable)
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
		bindings[i] = variable
	}
	if removedIndex != len(removed) {
		return result, ast.ErrASTDomain
	}
	body, err := r.substitute(formula, bindings)
	if err != nil {
		return result, err
	}
	refs = append(refs, body)
	bodyHandle, err := r.manager.RefGet(body)
	if err != nil {
		return result, err
	}
	for i, j := 0, len(bound)-1; i < j; i, j = i+1, j-1 {
		bound[i], bound[j] = bound[j], bound[i]
	}
	quantifier, err := r.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: bound, Names: names, Body: bodyHandle})
	if err != nil {
		return result, err
	}
	return r.manager.NewRef(quantifier)
}

func (r *relationFormulaChecker) verifyUnion(old, source, actual, deltaOld, deltaActual ast.ASTHandle, hasDelta bool) (resultErr error) {
	m := r.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return value, nil
	}
	ground := func(value ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := r.groundWithNames(value, true)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return m.RefGet(ref)
	}
	contains := func(container, subset ast.ASTHandle) error {
		conjunction, err := hold(m.MkAndArgs([]ast.ASTHandle{container, subset}))
		if err != nil {
			return err
		}
		return r.checkEquivalent(conjunction, subset)
	}
	expected, err := hold(m.MkOrArgs([]ast.ASTHandle{source, old}))
	if err != nil {
		return err
	}
	expected, err = ground(expected)
	if err != nil {
		return err
	}
	actual, err = ground(actual)
	if err != nil {
		return err
	}
	if err = r.checkEquivalent(expected, actual); err != nil {
		return fmt.Errorf("union: %w", err)
	}
	if !hasDelta {
		return nil
	}
	notOld, err := hold(m.MkNot(old))
	if err != nil {
		return err
	}
	newDifference, err := hold(m.MkAndArgs([]ast.ASTHandle{actual, notOld}))
	if err != nil {
		return err
	}
	newDifference, err = ground(newDifference)
	if err != nil {
		return err
	}
	delta, err := ground(deltaActual)
	if err != nil {
		return err
	}
	if err = contains(delta, newDifference); err != nil {
		return fmt.Errorf("union delta low: %w", err)
	}
	previousDelta, err := ground(deltaOld)
	if err != nil {
		return err
	}
	if err = contains(delta, previousDelta); err != nil {
		return fmt.Errorf("union delta0: %w", err)
	}
	upperLeft, err := hold(m.MkOrArgs([]ast.ASTHandle{actual, deltaOld}))
	if err != nil {
		return err
	}
	upperRight, err := hold(m.MkOrArgs([]ast.ASTHandle{delta, old}))
	if err != nil {
		return err
	}
	upperLeft, err = ground(upperLeft)
	if err != nil {
		return err
	}
	upperRight, err = ground(upperRight)
	if err != nil {
		return err
	}
	if err = r.checkEquivalent(upperLeft, upperRight); err != nil {
		return fmt.Errorf("union no overflow: %w", err)
	}
	return nil
}
