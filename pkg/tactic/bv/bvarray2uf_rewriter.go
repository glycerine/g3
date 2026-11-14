package bv

// Source: src/tactic/bv/bvarray2uf_rewriter.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// bvArrayRewriter is bvarray2uf_rewriter: rewriter_tpl<bvarray2uf_rewriter_cfg>
// together with its configuration.
type bvArrayRewriter struct {
	rewriter.DefaultConfiguration
	owner      *BVArray2UFTactic
	traversal  *rewriter.Traversal
	arrays     *ast.ObjMap     // m_arrays_fs: expr -> func_decl (Value[0])
	bindings   []ast.ASTHandle // m_bindings
	extra      []ast.ASTHandle // extra_assertions
	scratch    []ast.ASTHandle // local expr_refs of the running reduce_app
	extension  *tactic.ExtensionModelConverter
	filter     *tactic.FilterModelConverter
	out, proof ast.ASTRef
	proofGen   bool
}

func newBVArrayRewriter(t *BVArray2UFTactic) (r *bvArrayRewriter, err error) {
	m := t.manager
	r = &bvArrayRewriter{owner: t, arrays: ast.NewObjMap(m)}
	// rewriter_tpl<bvarray2uf_rewriter_cfg>(m, m.proofs_enabled(), m_cfg)
	if r.proofGen, err = m.ProofsEnabled(); err != nil {
		return r, err
	}
	if r.proofGen {
		r.traversal, err = rewriter.NewProofTraversal(m, r)
	} else {
		r.traversal, err = rewriter.NewTraversal(m, r)
	}
	if err != nil {
		return r, err
	}
	if r.out, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return r, err
	}
	r.proof, err = m.NewRef(ast.ASTHandle{})
	return r, err
}

// close is ~bvarray2uf_rewriter: the configuration member goes first
// (dec_ref_map_key_values over m_arrays_fs in table order, then
// extra_assertions and m_bindings), then the rewriter_tpl base.
func (r *bvArrayRewriter) close() (err error) {
	m := r.owner.manager
	for _, ref := range []*ast.ASTRef{&r.proof, &r.out} {
		if *ref != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(ref))
			*ref = ast.ASTRef{}
		}
	}
	if r.arrays != nil {
		for _, kv := range r.arrays.Entries() {
			err = errors.Join(err, m.DecRef(kv.Key))
			err = errors.Join(err, m.DecRef(kv.Value[0]))
		}
		r.arrays = nil
	}
	for _, h := range r.extra {
		err = errors.Join(err, m.DecRef(h))
	}
	r.extra = nil
	for _, h := range r.bindings {
		err = errors.Join(err, m.DecRef(h))
	}
	r.bindings = nil
	if r.traversal != nil {
		err = errors.Join(err, r.traversal.Close())
		r.traversal = nil
	}
	return err
}
func (r *bvArrayRewriter) addExtra(h ast.ASTHandle) error {
	if e := r.owner.manager.IncRef(h); e != nil {
		return e
	}
	r.extra = append(r.extra, h)
	return nil
}

// rewrite is m_rw(curr, new_curr, new_pr).
func (r *bvArrayRewriter) rewrite(root ast.ASTHandle) (value, proof ast.ASTHandle, err error) {
	m := r.owner.manager
	if r.proofGen {
		err = r.traversal.RewriteWithProof(root, r.out, r.proof)
	} else {
		if err = m.ResetRef(r.proof); err == nil {
			err = r.traversal.Rewrite(root, r.out)
		}
	}
	if err != nil {
		return value, proof, err
	}
	if value, err = m.RefGet(r.out); err != nil {
		return value, proof, err
	}
	proof, err = m.RefGet(r.proof)
	return value, proof, err
}

// PreVisit is bvarray2uf_rewriter_cfg::pre_visit.
func (r *bvArrayRewriter) PreVisit(t ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	k, e := m.GetKind(t)
	if e != nil {
		return false, e
	}
	if k == ast.ASTQuantifier {
		n, e := m.GetNumDeclarations(t)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			s, e := m.GetDeclarationSort(t, i)
			if e != nil {
				return false, e
			}
			if e = m.IncRef(s); e != nil {
				return false, e
			}
			r.bindings = append(r.bindings, s)
		}
	}
	return true, nil
}

// ReduceQuantifier is NOT_IMPLEMENTED_YET() in the source.
func (r *bvArrayRewriter) ReduceQuantifier(ast.ASTHandle, ast.ASTHandle, ast.AppArguments, ast.AppArguments, ast.ASTRef, ast.ASTRef) (bool, error) {
	return false, tactic.NewTacticException("bvarray2uf: quantified bit-vector arrays are not implemented")
}

// ReduceVar is bvarray2uf_rewriter_cfg::reduce_var.
func (r *bvArrayRewriter) ReduceVar(v ast.ASTHandle, _, _ ast.ASTRef) (bool, error) {
	idx, e := r.owner.manager.GetVarIndex(v)
	if e != nil {
		return false, e
	}
	if uint64(idx) >= uint64(len(r.bindings)) {
		return false, nil
	}
	return false, tactic.NewTacticException("bvarray2uf: quantified bit-vector arrays are not implemented")
}
