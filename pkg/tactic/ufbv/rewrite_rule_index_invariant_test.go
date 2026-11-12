package ufbv

import "github.com/glycerine/g3/pkg/ast"

// ruleIndexConsistent checks both directions of the Go rule/set ownership
// relationship. It is test-only; the source's check_fwd_idx_consistency has
// no production caller.
func ruleIndexConsistent(r *rewriteRuleIndex) bool {
	if r.check() != nil {
		return false
	}
	seen := make(map[ast.ASTHandle]ast.ASTHandle)
	for f, set := range r.byFunc {
		if set == nil {
			return false
		}
		for _, q := range objSetKeys(set.values) {
			rule, found := r.rules[q]
			if !found || rule.removing || rule.function != f {
				return false
			}
			if _, duplicate := seen[q]; duplicate {
				return false
			}
			seen[q] = f
		}
	}
	for q, rule := range r.rules {
		if f, found := seen[q]; !found || f != rule.function {
			return false
		}
	}
	return true
}
