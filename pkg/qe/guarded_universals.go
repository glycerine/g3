package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// A counterexample to A(x) => (B(x) => C(x)) must satisfy both A and B.
// If either guard requires x=t, checking the original body at t is equivalent
// to checking it for every x. Keep the other guards and the conclusion intact.
func (c *definitionRewriter) reduceGuardedUniversal(q, body ast.ASTHandle, out ast.ASTRef) (bool, error) {
	m := c.m
	n, err := m.GetNumDeclarations(q)
	if err != nil || n != 1 {
		return false, err
	}
	nested, err := m.HasQuantifiers(body)
	if err != nil || nested {
		return false, err
	}
	for current := body; c.spend(1); {
		h, err := m.NodeHeader(current)
		if err != nil || h.Family != m.BasicFamilyID() || h.DeclKind != ast.ASTOpImplies || h.NumArgs != 2 {
			return false, err
		}
		guard, err := m.GetArg(current, 0)
		if err != nil {
			return false, err
		}
		value, err := c.guardBinding(guard)
		if err != nil || c.stopped {
			return false, err
		}
		if !value.IsNull() {
			// A child rewrite may already have changed the quantifier body.
			// InstantiateQuantifier also shifts references to outer binders.
			target, err := m.UpdateQuantifierBody(q, body)
			if err != nil {
				return false, err
			}
			held, err := m.NewRef(target)
			if err != nil {
				return false, err
			}
			err = rewriter.InstantiateQuantifier(m, target, []ast.ASTHandle{value}, out)
			if err = errors.Join(err, m.DelRef(&held)); err != nil {
				return false, err
			}
			c.changed = true
			return true, nil
		}
		current, err = m.GetArg(current, 1)
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

// Only equality in a required conjunct fixes the variable. Disjunctions,
// predicate arguments and negated equalities cannot supply a binding.
func (c *definitionRewriter) guardBinding(guard ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.m
	leaves, found, err := c.definingConjuncts(guard, 1)
	if err != nil || !found {
		return ast.ASTHandle{}, err
	}
	for _, term := range leaves {
		h, err := m.NodeHeader(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if h.Family != m.BasicFamilyID() || h.DeclKind != ast.ASTOpEq || h.NumArgs != 2 {
			continue
		}
		for i := uint32(0); i < 2; i++ {
			x, err := m.GetArg(term, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			kind, _, index, _, err := m.SubstitutionShape(x)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if kind != ast.ASTVar || index != 0 {
				continue
			}
			value, err := m.GetArg(term, 1-i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			ground, err := m.IsGround(value)
			if err != nil || ground {
				return value, err
			}
		}
	}
	return ast.ASTHandle{}, nil
}
