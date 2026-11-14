package bv

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (r *bvArrayRewriter) isBVSort(s ast.ASTHandle) bool {
	_, e := r.owner.manager.BitVectorSize(s)
	return e == nil
}
func (r *bvArrayRewriter) isBVArraySort(s ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	f, e := m.GetFamilyID(s)
	if e != nil {
		return false, e
	}
	k, e := m.GetDeclKind(s)
	if e != nil {
		return false, e
	}
	if f != r.owner.arrayFamily || k != 0 {
		return false, nil
	}
	n, e := m.GetSortNumParameters(s)
	if e != nil || n < 2 {
		return false, e
	}
	for i := uint32(0); i+1 < n; i++ {
		v, e := m.ArrayDomain(s, i)
		if e != nil || !r.isBVSort(v) {
			return false, e
		}
	}
	v, e := m.ArrayRange(s)
	return e == nil && r.isBVSort(v), e
}
func (r *bvArrayRewriter) isBVArray(h ast.ASTHandle) (bool, error) {
	s, e := r.owner.manager.GetSort(h)
	if e != nil {
		return false, e
	}
	return r.isBVArraySort(s)
}
func (r *bvArrayRewriter) indexSort(s ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	n, e := m.GetSortNumParameters(s)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	var width uint32
	for i := uint32(0); i+1 < n; i++ {
		d, e := m.ArrayDomain(s, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		w, e := m.BitVectorSize(d)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		width += w
	}
	return m.BitVectorSort(width)
}

// freshFunction is mk_fresh_func_decl("f_t", "", 1, &domain, range) for the
// index and value sorts of e. The empty suffix is a non-null symbol, so the
// name is f_t!!<id>.
func (r *bvArrayRewriter) freshFunction(e ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	s, err := m.GetSort(e)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain, err := r.indexSort(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := m.ArrayRange(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	prefix, err := m.Symbols.Intern("f_t")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffix, err := m.Symbols.Intern("")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkFreshFuncDecl(prefix, suffix, []ast.ASTHandle{domain}, value)
}

// register is the model-converter bookkeeping and m_arrays_fs.insert(e, bv_f)
// with its two inc_refs.
func (r *bvArrayRewriter) register(e, f ast.ASTHandle) error {
	m := r.owner.manager
	if e2 := m.IncRef(f); e2 != nil {
		return e2
	}
	constant, yes, x := bvArrayConstant(m, e)
	if x != nil {
		_ = m.DecRef(f)
		return x
	}
	if yes {
		if r.extension != nil {
			array, x := m.MkAsArray(f)
			if x != nil {
				_ = m.DecRef(f)
				return x
			}
			if x = r.extension.Insert(constant, array); x != nil {
				_ = m.DecRef(f)
				return x
			}
		}
	} else if r.filter != nil {
		if x = r.filter.Insert(f); x != nil {
			_ = m.DecRef(f)
			return x
		}
	}
	if x = r.arrays.Insert(e, [2]ast.ASTHandle{f}); x != nil {
		_ = m.DecRef(f)
		return x
	}
	if x = m.IncRef(e); x != nil {
		return x
	}
	return nil
}

// mkUFForArray is bvarray2uf_rewriter_cfg::mk_uf_for_array.
func (r *bvArrayRewriter) mkUFForArray(e ast.ASTHandle) (ast.ASTHandle, error) {
	m := r.owner.manager
	if f, err := m.AsArrayFunction(e); err == nil {
		return f, nil
	}
	value, found, err := r.arrays.Find(e)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if found {
		r.owner.tracePair("for ", e, " found ", value[0], 129)
		return value[0], nil
	}
	f, err := r.freshFunction(e)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r.owner.tracePair("for ", e, " new func_decl is ", f, 116)
	if err = r.register(e, f); err != nil {
		return ast.ASTHandle{}, err
	}
	return f, nil
}

func bvArrayConstant(m *ast.ASTManager, h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return ast.ASTHandle{}, false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil || n != 0 {
		return ast.ASTHandle{}, false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil {
		return ast.ASTHandle{}, false, e
	}
	if f != ast.NullFamilyID {
		return ast.ASTHandle{}, false, nil
	}
	d, e := m.GetDecl(h)
	return d, e == nil, e
}
