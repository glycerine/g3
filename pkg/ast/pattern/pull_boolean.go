package pattern

// Source: src/ast/normal_forms/pull_quant.cpp no-proof consumer.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/normal_forms"
)

func (c *patternCandidates) pullNoProof(root ast.ASTHandle) (ast.ASTRef, error) {
	return normal_forms.PullQuantifierNoProof(c.manager, root, c.traceTags)
}

// pullWithProof is pull(new_q, new_expr, new_pr) with proofs enabled.
func (c *patternCandidates) pullWithProof(root ast.ASTHandle) (out, proof ast.ASTRef, err error) {
	return normal_forms.PullQuantifier(c.manager, root, c.traceTags)
}
