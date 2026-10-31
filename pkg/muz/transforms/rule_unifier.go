package transforms

// Source: src/muz/transforms/dl_mk_rule_inliner.cpp/.h, rule_unifier.
// The same rule unifier is used by BMC's proof reconstruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
)

// RuleUnifier keeps the two variable banks produced by the most recent rule
// unification. Rule resolution and normalization are separate source stages.
type RuleUnifier struct {
	manager   *ast.ASTManager
	subst     *substitution.Substitution
	unifier   *substitution.Unifier
	variables uint32
	ready     bool
}

func NewRuleUnifier(manager *ast.ASTManager) (*RuleUnifier, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	subst, err := substitution.New(manager)
	if err != nil {
		return nil, err
	}
	unifier, err := substitution.NewUnifier(manager)
	if err != nil {
		return nil, errors.Join(err, subst.Close())
	}
	return &RuleUnifier{manager: manager, subst: subst, unifier: unifier}, nil
}

// UnifyRules unifies the selected target tail with the source head, using
// distinct variable banks for the two rules.
func (u *RuleUnifier) UnifyRules(target *base.Rule, tailIndex int, source *base.Rule) (bool, error) {
	if u == nil || u.manager == nil || target == nil || source == nil {
		return false, ast.ErrASTHandle
	}
	u.ready = false
	tail, _, err := target.Tail(tailIndex)
	if err != nil {
		return false, err
	}
	head, err := source.Head()
	if err != nil {
		return false, err
	}
	left, err := ruleVariableCount(u.manager, target)
	if err != nil {
		return false, err
	}
	right, err := ruleVariableCount(u.manager, source)
	if err != nil {
		return false, err
	}
	if right > left {
		left = right
	}
	replacement, err := substitution.New(u.manager)
	if err != nil {
		return false, err
	}
	if err = u.subst.Close(); err != nil {
		return false, errors.Join(err, replacement.Close())
	}
	u.subst = replacement
	if err = u.subst.Reserve(2, left); err != nil {
		return false, err
	}
	unified, err := u.unifier.UnifyPair(tail, head, u.subst, true)
	if err != nil || !unified {
		return false, err
	}
	u.variables, u.ready = left, true
	return true, nil
}

// RuleSubstitution returns owning AST references for the selected rule's
// variable bank. The caller releases each reference through the AST manager.
func (u *RuleUnifier) RuleSubstitution(rule *base.Rule, isTarget bool) ([]ast.ASTRef, error) {
	if u == nil || u.manager == nil || rule == nil || !u.ready {
		return nil, ast.ErrASTHandle
	}
	var offset uint32
	if !isTarget {
		offset = 1
	}
	_, refs, err := ruleProofSubstitution(u.manager, rule, offset, u.variables, u.subst)
	return refs, err
}

func (u *RuleUnifier) Close() error {
	if u == nil || u.manager == nil {
		return ast.ErrASTHandle
	}
	err := errors.Join(u.unifier.Close(), u.subst.Close())
	u.manager, u.subst, u.unifier = nil, nil, nil
	u.ready = false
	return err
}
