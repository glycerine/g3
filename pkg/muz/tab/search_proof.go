package tab

// Source: src/muz/tab/tab_context.cpp, imp::get_proof and resolve_rule.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// Proof replays the current successful path and returns an owning proof reference.
func (s *tabSearch) Proof() (result ast.ASTRef, err error) {
	if s == nil || s.closed || len(s.stack) == 0 || len(s.stack[len(s.stack)-1].predicates) != 0 {
		return result, ast.ErrASTDomain
	}
	m := s.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	defer func() {
		err = errors.Join(err, scope.Close())
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	converter, err := tactic.NewReplaceProofConverter(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, converter.Close()) }()
	current := s.stack[len(s.stack)-1]
	root, err := current.Formula()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&root)) }()
	for current.index != 0 {
		if current.parentIndex >= current.index || int(current.parentIndex) >= len(s.stack) {
			return result, ast.ErrASTDomain
		}
		parent := s.stack[current.parentIndex]
		selected, err := m.RefGet(parent.predicates[parent.predicateIndex])
		if err != nil {
			return result, err
		}
		decl, err := m.GetDecl(selected)
		if err != nil {
			return result, err
		}
		rule, err := s.rules.Rule(decl, int(current.parentRule))
		if err != nil {
			return result, err
		}
		if err = s.insertResolutionProof(converter, parent, rule, current); err != nil {
			return result, err
		}
		current = parent
	}
	if err = converter.Invert(); err != nil {
		return result, err
	}
	formula, err := m.RefGet(root)
	if err != nil {
		return result, err
	}
	asserted, err := m.MkProofAsserted(formula)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = converter.Convert(m, []ast.ASTHandle{asserted}, result); err != nil {
		return result, err
	}
	return result, nil
}
func (s *tabSearch) insertResolutionProof(converter interface{ Insert(ast.ASTHandle) error }, parent, rule, child *Clause) (err error) {
	m := s.manager
	unfolded, matched, err := unfoldClauseExpressionsWithBindings(parent, parent.predicateIndex, rule, true)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	defer func() { err = errors.Join(err, unfolded.Close()) }()
	replayed, matched, err := finalizeUnfoldedClause(unfolded, s.rewriter, s.lite)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	defer func() { err = errors.Join(err, replayed.Close()) }()
	sub, err := unfolded.proofSubstitutions(parent, rule)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, sub.Close()) }()
	substitutions := make([][]ast.ASTHandle, 2)
	for bank := range sub.banks {
		for _, ref := range sub.banks[bank] {
			h, e := m.RefGet(ref)
			if e != nil {
				return e
			}
			substitutions[bank] = append(substitutions[bank], h)
		}
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
	}()
	var premises []ast.ASTHandle
	for _, clause := range []*Clause{parent, rule} {
		ref, e := clause.Formula()
		if e != nil {
			return e
		}
		refs = append(refs, ref)
		h, e := m.RefGet(ref)
		if e != nil {
			return e
		}
		proof, e := m.MkProofAsserted(h)
		if e != nil {
			return e
		}
		pin, e := m.NewRef(proof)
		if e != nil {
			return e
		}
		refs = append(refs, pin)
		premises = append(premises, proof)
	}
	conclusionRef, err := child.Formula()
	if err != nil {
		return err
	}
	refs = append(refs, conclusionRef)
	conclusion, err := m.RefGet(conclusionRef)
	if err != nil {
		return err
	}
	proof, err := m.MkProofHyperResolve(premises, conclusion, []ast.ASTProofPosition{{First: parent.predicateIndex + 1, Second: 0}}, substitutions)
	if err != nil {
		return err
	}
	return converter.Insert(proof)
}
