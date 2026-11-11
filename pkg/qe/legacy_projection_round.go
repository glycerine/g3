package qe

// Source: src/muz/spacer/spacer_util.cpp:507-581, qe_project's
// repeated qe_lite and theory-rewriter stages for array auxiliaries.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (p *LegacyProjection) projectArithmeticRound(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle) (result *legacyFormulaProjection, resultErr error) {
	scope, err := ast.NewScopedNoProof(p.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != nil {
			resultErr = closeTransferredLegacyOuterArithmetic(result, resultErr)
			result = nil
		}
	}()
	return p.arithmetic.projectFormula(md, variables, formula)
}

// qe_project constructs its array projection and following theory rewrite
// inside scoped_no_proof, then restores the caller's proof-generation mode.
func (p *LegacyProjection) projectArrayRound(md *model.ModelCore, variables []ast.ASTHandle, formula ast.ASTHandle, reduceAllSelects bool) (result *legacyArrayProjectionResult, resultErr error) {
	scope, err := ast.NewScopedNoProof(p.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != nil {
			resultErr = closeTransferredLegacyOuterArray(result, resultErr)
			result = nil
		}
	}()
	// th_rewriter srw(m): a local rewriter aware of the no-proof mode,
	// destroyed before _sp.
	srw, err := rewriter.NewTheoryRewriter(p.manager, p.config.Context, p.config.Globals, nil)
	if err != nil {
		if srw != nil {
			err = errors.Join(err, srw.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, srw.Close()) }()
	result, resultErr = p.array.projectFormula(md, variables, formula, reduceAllSelects)
	if resultErr == nil {
		resultErr = rewriteProjectionCurrentWith(p.manager, srw, result.formula)
	}
	return result, resultErr
}

// rewriteProjectionCurrentWith is rw(fml) for an expr_ref fml with the given
// rewriter.
func rewriteProjectionCurrentWith(m *ast.ASTManager, rw *rewriter.TheoryRewriter, current ast.ASTRef) error {
	root, err := m.RefGet(current)
	if err != nil {
		return err
	}
	rewritten, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = rw.Rewrite(root, rewritten); err != nil {
		return closeLegacyProjectionRewriteResult(m, rewritten, err)
	}
	if root, err = m.RefGet(rewritten); err != nil {
		return closeLegacyProjectionRewriteResult(m, rewritten, err)
	}
	err = m.SetRef(current, root)
	return closeLegacyProjectionRewriteResult(m, rewritten, err)
}

func (p *LegacyProjection) rewriteProjectionCurrent(current ast.ASTRef) (resultErr error) {
	root, err := p.manager.RefGet(current)
	if err != nil {
		return err
	}
	rewritten, err := p.arithmetic.rewriteLegacy(root)
	if err != nil {
		return err
	}
	root, err = p.manager.RefGet(rewritten)
	if err != nil {
		return closeLegacyProjectionRewriteResult(p.manager, rewritten, err)
	}
	err = p.manager.SetRef(current, root)
	return closeLegacyProjectionRewriteResult(p.manager, rewritten, err)
}

// prepareProjectionRound is the start of one qe_project round: qe_lite qe is
// constructed, applied and followed by rw(fml). The returned Lite stays
// open, as qe lives to the end of the round (its rewriter caches hold the
// eliminated formula's parts until then); the caller closes it.
func (p *LegacyProjection) prepareProjectionRound(variables []ast.ASTHandle, current ast.ASTRef) (refs []ast.ASTRef, lite *Lite, resultErr error) {
	defer func() {
		if resultErr != nil {
			if lite != nil {
				resultErr = errors.Join(resultErr, lite.Close())
				lite = nil
			}
			resultErr = errors.Join(resultErr, truncateLiteVector(p.manager, &refs, 0))
		}
	}()
	for _, variable := range variables {
		ref, err := p.manager.NewRef(variable)
		if err != nil {
			return refs, nil, err
		}
		refs = append(refs, ref)
	}
	lite, err := NewLite(p.manager, nil, false, p.config)
	if err != nil {
		return refs, nil, err
	}
	if err = lite.EliminateConstants(&refs, current); err != nil {
		return refs, lite, err
	}
	if err = p.rewriteProjectionCurrent(current); err != nil {
		return refs, lite, err
	}
	return refs, lite, nil
}
