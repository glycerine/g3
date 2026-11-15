package qe

// Source: src/muz/spacer/spacer_legacy_mbp.cpp Boolean substitution branch.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

// substituteSkolemBooleans is the Boolean branch of qe_project(..., map):
// each Boolean variable's completed model value goes into an
// expr_substitution (with proof pr = mk_asserted(true)), and
// mk_expr_simp_replacer applies it, simplifying as it rewrites. The caller
// runs rw(fml) afterwards.
func (p *LegacyProjection) substituteSkolemBooleans(md *model.ModelCore, formula ast.ASTRef, variables []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := p.manager
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.config.Context, Globals: p.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace,
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return ast.ASTRef{}, err
	}
	// expr_substitution sub(m); proof_ref pr(m.mk_asserted(m.mk_true()), m);
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelExprSubstitution(&sub)) }()
	truth, err := m.MkTrue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	asserted, err := m.MkProofAsserted(truth)
	if err != nil {
		return ast.ASTRef{}, err
	}
	pr, err := m.NewRef(asserted)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&pr)) }()
	values := make([]ast.ASTRef, 0, len(variables))
	defer func() { resultErr = errors.Join(resultErr, truncateLiteVector(m, &values, 0)) }()
	for _, variable := range variables {
		value, x := evaluator.EvaluateValue(variable)
		if x != nil {
			return ast.ASTRef{}, x
		}
		values = append(values, value)
		handle, x := m.RefGet(value)
		if x == nil {
			x = m.ExprSubstitutionInsert(sub, variable, handle, asserted, ast.ASTDependency{})
		}
		if x != nil {
			return ast.ASTRef{}, x
		}
	}
	// scoped_ptr<expr_replacer> rep = mk_expr_simp_replacer(m);
	// rep->set_substitution(&sub); (*rep)(fml);
	replacer, err := rewriter.NewSimplifyingExprReplacer(m, p.config.Context, p.config.Globals, nil)
	if err != nil {
		if replacer != nil {
			err = errors.Join(err, replacer.Close())
		}
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, replacer.SetSubstitution(nil), replacer.Close()) }()
	if err = replacer.SetSubstitution(&sub); err != nil {
		return ast.ASTRef{}, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	body, err := m.RefGet(formula)
	if err == nil {
		err = replacer.Rewrite(body, result)
	}
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}
