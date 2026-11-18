package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::normalize theory and NNF passes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type exprQETreeSimplifier interface {
	Simplify(ast.ASTRef, *rewriter.TheoryRewriter) (bool, error)
}

func normalizeExprQETree(node *exprQETree, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (err error) {
	if node == nil || node.manager == nil || rw == nil || nnf == nil || node.formula == (ast.ASTRef{}) {
		return ast.ErrASTHandle
	}
	m := node.manager
	root, err := m.RefGet(node.formula)
	if err != nil {
		return err
	}
	if err = rw.Rewrite(root, node.formula); err != nil {
		return err
	}
	// while (simplified): the first plugin, in family order, that simplifies
	// restarts the scan.
	for simplified := true; simplified; {
		simplified = false
		for _, simplifier := range node.simplifiers {
			changed, cause := simplifier.Simplify(node.formula, rw)
			if cause != nil {
				return cause
			}
			if changed {
				simplified = true
				break
			}
		}
	}
	atoms, err := nnf.Normalize(node.formula)
	if err != nil {
		return err
	}
	// TRACE("qe", tout << "nnf: " << mk_ismt2_pp(result.get(), m) << "\n";)
	// (AUDIT.md qe_05-F6).
	if err = qeTrace(m, node.config, "normalize", "src/qe/qe.cpp", 1809, "nnf: ", node.formula, "\n"); err != nil {
		return errors.Join(err, atoms.Close())
	}
	if node.atoms != nil {
		if err = node.atoms.Close(); err != nil {
			return errors.Join(err, atoms.Close())
		}
	}
	node.atoms = atoms
	return nil
}
