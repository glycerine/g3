package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (r *bvArrayRewriter) forall(sort, body ast.ASTHandle) (ast.ASTHandle, error) {
	name, e := r.owner.manager.Symbols.Intern("x")
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return r.owner.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{name}, Body: body})
}
func (r *bvArrayRewriter) apply(f, index ast.ASTHandle) (ast.ASTHandle, error) {
	return r.owner.manager.MkApp(f, []ast.ASTHandle{index})
}

// hold keeps a local expr_ref/app_ref alive until release.
func (r *bvArrayRewriter) hold(h ast.ASTHandle) error {
	if e := r.owner.manager.IncRef(h); e != nil {
		return e
	}
	r.scratch = append(r.scratch, h)
	return nil
}
func (r *bvArrayRewriter) release(limit int) (err error) {
	for len(r.scratch) > limit {
		i := len(r.scratch) - 1
		if e := r.owner.manager.DecRef(r.scratch[i]); e != nil && err == nil {
			err = e
		}
		r.scratch = r.scratch[:i]
	}
	return err
}

// indexVar is sorts[0] = get_index_sort(get_sort(e)) and x = mk_var(0, sorts[0]).
func (r *bvArrayRewriter) indexVar(e ast.ASTHandle) (sort, x ast.ASTHandle, err error) {
	s, err := r.owner.manager.GetSort(e)
	if err != nil {
		return sort, x, err
	}
	return r.indexVarOfSort(s)
}
func (r *bvArrayRewriter) indexVarOfSort(s ast.ASTHandle) (sort, x ast.ASTHandle, err error) {
	if sort, err = r.indexSort(s); err != nil {
		return sort, x, err
	}
	x, err = r.owner.manager.MkVar(0, sort)
	return sort, x, err
}
