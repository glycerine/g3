package bv

// Source: src/tactic/bv/bvarray2uf_rewriter.cpp reduce_app. GCC evaluates
// call arguments right to left, so nested constructor calls build their
// last argument first.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// ReduceApp is bvarray2uf_rewriter_cfg::reduce_app.
func (r *bvArrayRewriter) ReduceApp(f ast.ASTHandle, arguments ast.AppArguments, out, _ ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	m := r.owner.manager
	// The source's local expr_refs die when reduce_app returns.
	start := len(r.scratch)
	defer func() {
		if e := r.release(start); e != nil && err == nil {
			err = e
		}
	}()
	num := arguments.Count()
	args := make([]ast.ASTHandle, num)
	for i := range args {
		a, e := arguments.At(uint32(i))
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		args[i] = a
	}
	family, e := m.GetFamilyID(f)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	op, e := m.GetDeclKind(f)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	rangeSort, e := m.GetRange(f)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	basic := family == m.BasicFamilyID()
	domainArray := false
	if basic && (op == ast.ASTOpEq || op == ast.ASTOpDistinct) {
		arity, e := m.GetArity(f)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if arity == 0 {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		d0, e := m.GetDomain(f, 0)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if domainArray, e = r.isBVArraySort(d0); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	done := func(h ast.ASTHandle, e error) (rewriter.RewriteStatus, error) {
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(out, h); e != nil {
			return rewriter.RewriteFailed, e
		}
		return rewriter.RewriteDone, nil
	}
	if basic && op == ast.ASTOpEq && domainArray {
		// From [1]: replace t = s, where t and s are arrays, with
		// \forall x . f_t(x) = f_s(x).
		same, e := m.AreEqual(args[0], args[1])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if same {
			return done(m.MkTrue())
		}
		ft, e := r.mkUFForArray(args[0])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		fs, e := r.mkUFForArray(args[1])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		sort, x, e := r.indexVar(args[0])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		// mk_eq(mk_app(f_t, x), mk_app(f_s, x))
		right, e := r.apply(fs, x)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		left, e := r.apply(ft, x)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		body, e := m.MkEq(left, right)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		return done(r.forall(sort, body))
	}
	if basic && op == ast.ASTOpDistinct && domainArray {
		expanded, e := m.MkDistinctExpanded(args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(out, expanded); e != nil {
			return rewriter.RewriteFailed, e
		}
		return rewriter.Rewrite1, nil
	}
	rangeArray, e := r.isBVArraySort(rangeSort)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	// is_term_ite(f): an ite whose range is not Bool.
	if basic && op == ast.ASTOpIte && rangeArray {
		c := args[0]
		ft, e := r.mkUFForArray(args[1])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ff, e := r.mkUFForArray(args[2])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		sort, x, e := r.indexVar(args[1])
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		pin := func(h ast.ASTHandle, e error) (ast.ASTHandle, error) {
			if e != nil {
				return h, e
			}
			return h, r.hold(h)
		}
		fa, e := pin(m.MkApp(f, args))
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		fta, e := pin(r.apply(ft, x))
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ffa, e := pin(r.apply(ff, x))
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ite, e := pin(m.MkIte(c, fta, ffa))
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		var bvf ast.ASTHandle
		value, found, e := r.arrays.Find(fa)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if found {
			bvf = value[0]
		} else {
			// The source looks f_a up but inserts under e.
			if bvf, e = r.freshFunction(args[1]); e != nil {
				return rewriter.RewriteFailed, e
			}
			if e = r.register(ite, bvf); e != nil {
				return rewriter.RewriteFailed, e
			}
		}
		// body = mk_eq(mk_app(bv_f, x), e)
		at, e := r.apply(bvf, x)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		body, e := m.MkEq(at, ite)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		q, e := r.forall(sort, body)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.addExtra(q); e != nil {
			return rewriter.RewriteFailed, e
		}
		return done(m.MkAsArray(bvf))
	}
	if basic && rangeArray {
		// NOT_IMPLEMENTED_YET() in the source.
		return rewriter.RewriteFailed, tactic.NewTacticException("bvarray2uf: basic array-valued application is not implemented")
	}
	if family == ast.NullFamilyID {
		hasArrays := false
		for _, a := range args {
			yes, e := r.isBVArray(a)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			hasArrays = hasArrays || yes
		}
		t, e := m.MkApp(f, args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if rangeArray {
			// From [1]: every array term t gets a fresh uninterpreted f_t.
			ft, e := r.mkUFForArray(t)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			return done(m.MkAsArray(ft))
		}
		if hasArrays {
			return done(t, nil)
		}
		return rewriter.RewriteFailed, nil
	}
	if family != r.owner.arrayFamily {
		return rewriter.RewriteFailed, nil
	}
	if op == ast.ASTArrayOpSelect {
		if num < 2 {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		t, i := args[0], args[1]
		yes, e := r.isBVArray(t)
		if e != nil || !yes {
			return rewriter.RewriteFailed, e
		}
		// From [1]: replace select(t, i) with f_t(i).
		ft, e := r.mkUFForArray(t)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		return done(r.apply(ft, i))
	}
	if !rangeArray {
		return rewriter.RewriteFailed, nil
	}
	switch op {
	case ast.ASTArrayOpConst:
		if num != 1 {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		t, e := m.MkApp(f, args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.hold(t); e != nil {
			return rewriter.RewriteFailed, e
		}
		v := args[0]
		ft, e := r.mkUFForArray(t)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		result, e := m.MkAsArray(ft)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(out, result); e != nil {
			return rewriter.RewriteFailed, e
		}
		// Add \forall x . f_t(x) = v
		sort, x, e := r.indexVarOfSort(rangeSort)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		at, e := r.apply(ft, x)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		body, e := m.MkEq(at, v)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		q, e := r.forall(sort, body)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.addExtra(q); e != nil {
			return rewriter.RewriteFailed, e
		}
		return rewriter.RewriteDone, nil
	case ast.ASTArrayOpAsArray:
		return rewriter.RewriteFailed, nil
	case ast.ASTArrayOpMap:
		t, e := m.MkApp(f, args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.hold(t); e != nil {
			return rewriter.RewriteFailed, e
		}
		ft, e := r.mkUFForArray(t)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		mapf, e := m.ArrayMapFunction(f)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ss := make([]ast.ASTHandle, num)
		for i, a := range args {
			if ss[i], e = r.mkUFForArray(a); e != nil {
				return rewriter.RewriteFailed, e
			}
		}
		// Add \forall x . f_t(x) = map_f(a[x], b[x], ...)
		sort, x, e := r.indexVarOfSort(rangeSort)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		newArgs := make([]ast.ASTHandle, num)
		for i := range ss {
			if newArgs[i], e = r.apply(ss[i], x); e != nil {
				return rewriter.RewriteFailed, e
			}
			if e = r.hold(newArgs[i]); e != nil {
				return rewriter.RewriteFailed, e
			}
		}
		// mk_eq(mk_app(f_t, x), mk_app(map_f, num, new_args))
		mapped, e := m.MkApp(mapf, newArgs)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.hold(mapped); e != nil {
			return rewriter.RewriteFailed, e
		}
		at, e := r.apply(ft, x)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		body, e := m.MkEq(at, mapped)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		q, e := r.forall(sort, body)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.addExtra(q); e != nil {
			return rewriter.RewriteFailed, e
		}
		return done(m.MkAsArray(ft))
	case ast.ASTArrayOpStore:
		if num < 3 {
			return rewriter.RewriteFailed, ast.ErrASTDomain
		}
		s, i, v := args[0], args[1], args[2]
		yes, e := r.isBVArray(s)
		if e != nil || !yes {
			return rewriter.RewriteFailed, e
		}
		t, e := m.MkApp(f, args)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.hold(t); e != nil {
			return rewriter.RewriteFailed, e
		}
		// From [1]: for every term t of the form store(s, i, v), add
		// \forall x . x = i \vee f_t(x) = f_s(x), and the atom f_t(i) = v.
		fs, e := r.mkUFForArray(s)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ft, e := r.mkUFForArray(t)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		result, e := m.MkAsArray(ft)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(out, result); e != nil {
			return rewriter.RewriteFailed, e
		}
		sort, x, e := r.indexVarOfSort(rangeSort)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		// mk_or(mk_eq(x, i), mk_eq(mk_app(f_t, x), mk_app(f_s, x)))
		oldAt, e := r.apply(fs, x)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		newAt, e := r.apply(ft, x)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		unchanged, e := m.MkEq(newAt, oldAt)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.hold(unchanged); e != nil {
			return rewriter.RewriteFailed, e
		}
		same, e := m.MkEq(x, i)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		body, e := m.MkOr(same, unchanged)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		q, e := r.forall(sort, body)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.addExtra(q); e != nil {
			return rewriter.RewriteFailed, e
		}
		// ground_atom = mk_eq(mk_app(f_t, i), v)
		at, e := r.apply(ft, i)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		ground, e := m.MkEq(at, v)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = r.addExtra(ground); e != nil {
			return rewriter.RewriteFailed, e
		}
		return rewriter.RewriteDone, nil
	}
	// NOT_IMPLEMENTED_YET() in the source.
	return rewriter.RewriteFailed, tactic.NewTacticException("bvarray2uf: array operation is not implemented")
}
