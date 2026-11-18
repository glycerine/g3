package pdr

// Source: src/muz/pdr/pdr_farkas_learner.h/.cpp, farkas_learner's m_pr and
// m_ctx: the constructor makes a private proof manager
// (ast_manager m_pr(PGM_ENABLED); reg_decl_plugins(m_pr)) and an
// smt::kernel over it (m_ctx = alloc(smt::kernel, m_pr, m_proof_params));
// ~farkas_learner deallocates the kernel and then the manager. Only
// get_lemma_guesses would use them, and prop_solver never reaches it, but
// their construction and destruction are AST events of every
// extract_theory_core.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type farkasProofManager struct {
	manager *ast.ASTManager
	kernel  *smtcore.DefaultParamsKernel
}

func newFarkasProofManager(outer *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*farkasProofManager, error) {
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: outer.Symbols, Numerals: outer.NumeralService(), Budget: outer.Budget(), ProofMode: ast.ASTProofEnabled, Params: context, Globals: globals})
	if err != nil {
		if manager != nil {
			err = errors.Join(err, manager.Close())
		}
		return nil, err
	}
	result := &farkasProofManager{manager: manager}
	if err = manager.RegisterDeclarationPlugins(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	// get_proof_params changes only arithmetic settings, which the kernel
	// constructor does not read.
	if result.kernel, err = smtcore.NewDefaultParamsKernel(manager); err != nil {
		result.kernel = nil
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// Close is the m_ctx then m_pr part of ~farkas_learner.
func (f *farkasProofManager) Close() (result error) {
	if f == nil {
		return nil
	}
	if f.kernel != nil {
		result = f.kernel.Close()
		f.kernel = nil
	}
	if f.manager != nil {
		result = errors.Join(result, f.manager.Close())
		f.manager = nil
	}
	return result
}
