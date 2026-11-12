package rel

// Source: src/muz/rel/dl_relation_manager.cpp
// default_table_filter_interpreted_fn::should_remove (1405-1424) and
// src/muz/base/dl_context.h get_var_subst (m_var_subst, std_order = true).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// GroundCondition is should_remove's evaluation of an interpreted condition
// on one fact:
//
//	m_vs(m_condition, args.size(), args.c_ptr(), ground);
//	m_simp(ground);
//
// bindings use the var_subst standard order (variable i is
// bindings[len-1-i]; a zero handle leaves the variable free). The condition
// is instantiated by the context var_subst and only the ground instance is
// given to the borrowed context rewriter. The rewriter is never given
// bindings: rewriter_tpl keeps them, and its cache, after the call, so a
// later plain rewrite by the same context rewriter (for example the
// interpreted-tail simplifier of the next query) would substitute the last
// fact's values for that rule's variables.
func (m *RelationManager) GroundCondition(rewriter *astrewriter.TheoryRewriter, condition ast.ASTHandle, bindings []ast.ASTHandle, out ast.ASTRef) (result error) {
	if err := m.open(); err != nil {
		return err
	}
	if rewriter == nil {
		return ErrTable
	}
	if m.varSubst == nil {
		return ErrTable
	}
	ground, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	// The source var_subst resets its cache at its next call; release it
	// now so the instantiated terms do not outlive the evaluated fact
	// through the relation manager (frees only, no new terms).
	defer func() { result = errors.Join(result, m.manager.DelRef(&ground), m.varSubst.Reset()) }()
	if err = m.varSubst.Substitute(condition, bindings, ground); err != nil {
		return err
	}
	handle, err := m.manager.RefGet(ground)
	if err != nil {
		return err
	}
	return rewriter.Rewrite(handle, out)
}
