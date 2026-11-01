package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// ComputeProofCore lazily owns the asserted leaves of the current native proof.
// Source: duality_rpfp.cpp:1151-1163 and duality_wrapper.cpp:629-650.
func (g *RPFP) ComputeProofCore() (result error) {
	if err := g.solverReady(); err != nil {
		return err
	}
	if g.proofCore != nil {
		return nil
	}
	proof, err := g.solver.Proof()
	if err != nil {
		return err
	}
	if proof.IsNull() {
		return ast.ErrASTHandle
	}
	manager, err := interp.NewManager(g.context.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, manager.Close()) }()
	assumptions, err := interp.ProofAssumptions(manager, proof)
	if err != nil {
		return err
	}
	g.proofCore = make(map[ast.ASTHandle]ast.ASTRef)
	for _, term := range assumptions {
		if _, exists := g.proofCore[term]; exists {
			continue
		}
		ref, err := g.context.manager.NewRef(term)
		if err != nil {
			return errors.Join(err, g.ClearProofCore())
		}
		g.proofCore[term] = ref
	}
	return nil
}

// EdgeUsedInProof tests the edge formula and its localized constraints against
// asserted proof leaves. Cached assumption-literal translation is separate.
// Source: duality_rpfp.cpp:4227-4236.
func (g *RPFP) EdgeUsedInProof(edge *Edge) (bool, error) {
	if err := g.open(); err != nil {
		return false, err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return false, ErrWrapperOwner
	}
	if err := g.ComputeProofCore(); err != nil {
		return false, err
	}
	refs := append([]ast.ASTRef{edge.dual}, edge.constraints...)
	for _, ref := range refs {
		if ref == (ast.ASTRef{}) {
			continue
		}
		term, err := g.context.manager.RefGet(ref)
		if err != nil {
			return false, err
		}
		if _, found := g.proofCore[term]; found {
			return true, nil
		}
	}
	return false, nil
}
