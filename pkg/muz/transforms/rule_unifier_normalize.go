package transforms

// Source: src/muz/transforms/dl_mk_rule_inliner.cpp, rule_unifier with its
// default m_normalize = true, as datalog::rule_unifier unif(dctx) is used by
// model_search::get_proof_trace (src/muz/pdr/pdr_context.cpp) and by
// mk_slice::slice_proof_converter (src/muz/transforms/dl_mk_slice.cpp).

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// NormalizingRuleUnifier is a stand-alone datalog::rule_unifier. Its
// normalization stage (rule_manager::fix_unbound_vars and the member
// mk_interp_tail_simplifier) uses the services configured on the borrowed
// inliner plugin, which the unifier owns and closes.
type NormalizingRuleUnifier struct {
	plugin *RuleInlinerPlugin
}

// NewNormalizingRuleUnifier takes ownership of plugin, which supplies the
// rule manager, parameter services and the rule unifier's substitution.
func NewNormalizingRuleUnifier(plugin *RuleInlinerPlugin) (*NormalizingRuleUnifier, error) {
	if plugin == nil || plugin.closed || plugin.closing || plugin.manager == nil || plugin.rules == nil {
		return nil, ast.ErrASTHandle
	}
	return &NormalizingRuleUnifier{plugin: plugin}, nil
}

// RuleManager returns the borrowed rule manager the unifier builds rules in.
func (u *NormalizingRuleUnifier) RuleManager() *base.RuleManager {
	if u == nil || u.plugin == nil {
		return nil
	}
	return u.plugin.rules
}

// UnifyRules is rule_unifier::unify_rules(tgt, tgt_idx, src).
func (u *NormalizingRuleUnifier) UnifyRules(target *base.Rule, tailIndex int, source *base.Rule) (bool, error) {
	if u == nil || u.plugin == nil || u.plugin.closed || u.plugin.closing {
		return false, ast.ErrASTHandle
	}
	p := u.plugin
	if p.unifier == nil {
		unifier, err := NewRuleUnifier(p.manager)
		if err != nil {
			return false, err
		}
		p.unifier = unifier
	}
	return p.unifier.UnifyRules(target, tailIndex, source)
}

// RuleSubstitution is rule_unifier::get_rule_subst(r, is_tgt). Each returned
// reference belongs to the caller.
func (u *NormalizingRuleUnifier) RuleSubstitution(rule *base.Rule, isTarget bool) ([]ast.ASTRef, error) {
	if u == nil || u.plugin == nil || u.plugin.unifier == nil {
		return nil, ast.ErrASTHandle
	}
	return u.plugin.unifier.RuleSubstitution(rule, isTarget)
}

// Apply is rule_unifier::apply(tgt, tail_index, src, res) with m_normalize
// set: the resolvent is built normalized, its unbound variables are fixed
// and the interpreted tail is simplified. It returns false (and no rule)
// when the simplified interpreted tail is unsatisfiable.
func (u *NormalizingRuleUnifier) Apply(target *base.Rule, tailIndex int, source *base.Rule) (*base.Rule, bool, error) {
	if u == nil || u.plugin == nil || u.plugin.unifier == nil {
		return nil, false, ast.ErrASTHandle
	}
	p := u.plugin
	resolved, err := p.unifier.ResolveRaw(p.rules, target, tailIndex, source)
	if err != nil {
		return nil, false, err
	}
	normalized, err := p.normalizeResolvedRule(resolved)
	if err != nil {
		return nil, false, err
	}
	if normalized == nil {
		return nil, false, nil
	}
	return normalized, true, nil
}

// Close releases the unifier and the plugin that supplied its services.
func (u *NormalizingRuleUnifier) Close() error {
	if u == nil {
		return ast.ErrASTHandle
	}
	if u.plugin == nil {
		return nil
	}
	if err := u.plugin.Close(); err != nil {
		return err
	}
	u.plugin = nil
	return nil
}
