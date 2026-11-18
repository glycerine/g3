package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// GetProof returns an owned proof reference, null-valued for a safe query.
// Source: muz/duality/duality_dl_interface.cpp, get_proof.
func (e *QueryEngine) GetProof() (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if e.active == nil || e.status == lbool.Undef {
		return ast.ASTRef{}, graph.ErrWrapperOwner
	}
	if e.status != lbool.True {
		m, err := e.context.Manager()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return m.NewRef(ast.ASTHandle{})
	}
	background := append([]*graph.Expr(nil), e.config.Theory...)
	background = append(background, e.active.background...)
	var proof ast.ASTRef
	err := e.context.WithProfiler(e.config.Profiler, func() error {
		var err error
		proof, err = e.active.engine.GetCounterexample().Proof(e.config.Factory, background...)
		return err
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	return proof, nil
}

func (b *Backend) GetProof() (ast.ASTRef, error) {
	if err := b.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return b.queries.GetProof()
}
