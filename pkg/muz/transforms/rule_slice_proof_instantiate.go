package transforms

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// instantiateAsserted translates ground sliced substitutions into the original
// rule's variable bank, leaving removed variables universally quantified.
func (c *SliceProofConverter) instantiateAsserted(original *base.Rule, originalProof ast.ASTRef, variables []uint32, input []ast.ASTHandle) (rule *base.Rule, proof ast.ASTRef, resultErr error) {
	m := c.manager
	defer func() {
		if resultErr != nil {
			if rule != nil {
				resultErr = errors.Join(resultErr, rule.Close())
				rule = nil
			}
			if proof != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&proof))
			}
		}
	}()
	sorts, err := original.VariableSorts()
	if err != nil {
		return nil, proof, err
	}
	if len(input) > len(variables) {
		return nil, proof, fmt.Errorf("slice proof: substitution exceeds sliced variables")
	}
	bindings := make([]ast.ASTHandle, len(sorts))
	for i, term := range input {
		if int(variables[i]) >= len(bindings) || term.IsNull() {
			return nil, proof, ast.ErrASTDomain
		}
		isValue, e := m.IsValue(term)
		if e != nil {
			return nil, proof, e
		}
		if !isValue {
			return nil, proof, fmt.Errorf("slice proof: non-value substitution is not implemented")
		}
		sort, e := m.GetSort(term)
		if e != nil {
			return nil, proof, e
		}
		if sort != sorts[variables[i]] {
			return nil, proof, ast.ErrASTDomain
		}
		bindings[variables[i]] = term
	}
	temporary, err := c.rules.SubstituteRule(original, bindings)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, temporary.Close()) }()
	used, err := temporary.UsedVariables()
	if err != nil {
		return nil, proof, err
	}
	var refs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, refs)) }()
	var next uint32
	for i := range bindings {
		if !bindings[i].IsNull() {
			continue
		}
		sort := used.Contains(uint32(i))
		if sort.IsNull() {
			return nil, proof, fmt.Errorf("slice proof: missing original variable")
		}
		variable, e := m.MkVar(next, sort)
		if e != nil {
			return nil, proof, e
		}
		ref, e := m.NewRef(variable)
		if e != nil {
			return nil, proof, e
		}
		refs = append(refs, ref)
		bindings[i] = variable
		next++
	}
	head, err := temporary.Head()
	if err != nil {
		return nil, proof, err
	}
	count, err := temporary.TailSize()
	if err != nil {
		return nil, proof, err
	}
	tails, negative := make([]ast.ASTHandle, count), make([]bool, count)
	for i := range tails {
		tails[i], negative[i], err = temporary.Tail(i)
		if err != nil {
			return nil, proof, err
		}
	}
	name, err := temporary.Name()
	if err != nil {
		return nil, proof, err
	}
	rule, err = c.rules.NewRule(head, tails, negative, name)
	if err != nil {
		return nil, proof, err
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return rule, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&formula)) }()
	fact, err := m.RefGet(formula)
	if err != nil {
		return rule, proof, err
	}
	parent, err := m.RefGet(originalProof)
	if err != nil {
		return rule, proof, err
	}
	node, err := m.MkProofHyperResolve([]ast.ASTHandle{parent}, fact, nil, [][]ast.ASTHandle{bindings})
	if err != nil {
		return rule, proof, err
	}
	proof, err = m.NewRef(node)
	return rule, proof, err
}
