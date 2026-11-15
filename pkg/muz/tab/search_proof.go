package tab

// Source: src/muz/tab/tab_context.cpp, imp::get_proof and resolve_rule.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Proof replays the current successful path and returns an owning proof reference.
func (s *tabSearch) Proof() (result ast.ASTRef, err error) {
	if s == nil || s.closed || len(s.stack) == 0 || len(s.stack[len(s.stack)-1].predicates) != 0 {
		return result, ast.ErrASTDomain
	}
	m := s.manager
	attempt := &tabProofAttempt{manager: m}
	defer func() { result, err = finishTabProofAttempt(attempt, result, err) }()
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	attempt.scope = scope
	converter, err := tactic.NewReplaceProofConverter(m)
	if err != nil {
		return result, err
	}
	attempt.converter = converter
	current := s.stack[len(s.stack)-1]
	root, err := current.Formula()
	if err != nil {
		return result, err
	}
	attempt.root = root
	// substs is get_proof's vector<expr_ref_vector> of each step's
	// get_rule_subst(true), kept for display_body_insts (g3trace only).
	var substs [][]ast.ASTRef
	defer func() {
		for i := range substs {
			for j := range substs[i] {
				err = errors.Join(err, m.DelRef(&substs[i][j]))
			}
		}
	}()
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
		s1, err := s.insertResolutionProofSubst(converter, parent, rule, current)
		if err != nil {
			return result, err
		}
		substs = append(substs, s1)
		current = parent
	}
	// IF_VERBOSE(1, display_body_insts(substs, *clause, verbose_stream()))
	// (AUDIT.md FUZZ-F5; g3trace builds only).
	if diagnostic.VerboseProgress {
		if err = s.verboseBodyInsts(substs, current); err != nil {
			return result, err
		}
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
	s1, err := s.insertResolutionProofSubst(converter, parent, rule, child)
	for i := range s1 {
		err = errors.Join(err, s.manager.DelRef(&s1[i]))
	}
	return err
}

// insertResolutionProofSubst is resolve_rule; it also returns new
// references to s1 (get_rule_subst(true)) when VerboseProgress is set.
func (s *tabSearch) insertResolutionProofSubst(converter interface{ Insert(ast.ASTHandle) error }, parent, rule, child *Clause) (s1 []ast.ASTRef, err error) {
	m := s.manager
	attempt := &tabResolutionProofAttempt{manager: m}
	defer func() { err = finishTabResolutionProofAttempt(attempt, err) }()
	unfolded, matched, err := unfoldClauseExpressionsWith(parent, parent.predicateIndex, rule, true, s.unifierSlot())
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ast.ErrASTDomain
	}
	attempt.unfolded = unfolded
	replayed, matched, err := finalizeUnfoldedClause(unfolded, s.rewriter, s.lite)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ast.ErrASTDomain
	}
	attempt.replayed = replayed
	sub, err := unfolded.proofSubstitutions(parent, rule)
	if err != nil {
		return nil, err
	}
	if unfolded.members == nil {
		attempt.sub = sub
	}
	substitutions := make([][]ast.ASTHandle, 2)
	for bank := range sub.banks {
		for _, ref := range sub.banks[bank] {
			h, e := m.RefGet(ref)
			if e != nil {
				return nil, e
			}
			substitutions[bank] = append(substitutions[bank], h)
		}
	}
	if diagnostic.VerboseProgress {
		for _, h := range substitutions[0] {
			ref, e := m.NewRef(h)
			if e != nil {
				return s1, e
			}
			s1 = append(s1, ref)
		}
	}
	var premises []ast.ASTHandle
	for _, clause := range []*Clause{parent, rule} {
		ref, e := clause.Formula()
		if e != nil {
			return nil, e
		}
		attempt.refs = append(attempt.refs, ref)
		h, e := m.RefGet(ref)
		if e != nil {
			return nil, e
		}
		proof, e := m.MkProofAsserted(h)
		if e != nil {
			return nil, e
		}
		pin, e := m.NewRef(proof)
		if e != nil {
			return nil, e
		}
		attempt.refs = append(attempt.refs, pin)
		premises = append(premises, proof)
	}
	conclusionRef, err := child.Formula()
	if err != nil {
		return nil, err
	}
	attempt.refs = append(attempt.refs, conclusionRef)
	conclusion, err := m.RefGet(conclusionRef)
	if err != nil {
		return nil, err
	}
	proof, err := m.MkProofHyperResolve(premises, conclusion, []ast.ASTProofPosition{{First: parent.predicateIndex + 1, Second: 0}}, substitutions)
	if err != nil {
		return nil, err
	}
	return s1, converter.Insert(proof)
}
