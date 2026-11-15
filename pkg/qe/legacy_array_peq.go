package qe

// Source: src/muz/spacer/spacer_qe_project.cpp:44-186 (anonymous namespace
// class peq).

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// legacyPeqName is peq::PARTIAL_EQ. A partial equality (a ==I b) is denoted
// f(a,b,i0,i1,...) where f is the uninterpreted predicate partial_eq.
const legacyPeqName = "partial_eq"

// legacyPeq is the source peq: lhs, rhs and the diff indices of a partial
// equality application.
type legacyPeq struct {
	lhs, rhs ast.ASTHandle
	indices  []ast.ASTHandle
}

// legacyPeqOf is peq::peq(app* p, ast_manager& m).
func (p *legacyArrayProjection) legacyPeqOf(application ast.ASTHandle) (legacyPeq, error) {
	m := p.syntax.manager
	decl, err := m.GetDecl(application)
	if err != nil {
		return legacyPeq{}, err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return legacyPeq{}, err
	}
	partialEq, err := m.Symbols.Intern(legacyPeqName)
	if err != nil {
		return legacyPeq{}, err
	}
	if name != partialEq {
		return legacyPeq{}, diagnostic.BorrowProcessDebugContext().VerifyFailure("src/muz/spacer/spacer_qe_project.cpp", 101, "is_partial_eq (p)")
	}
	count, err := m.GetNumArgs(application)
	if err != nil {
		return legacyPeq{}, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		if args[i], err = m.GetArg(application, uint32(i)); err != nil {
			return legacyPeq{}, err
		}
	}
	return legacyPeq{lhs: args[0], rhs: args[1], indices: args[2:]}, nil
}

// mkPeq is peq::peq(lhs, rhs, num_indices, diff_indices, m) followed by
// peq::mk_peq.
func (p *legacyArrayProjection) mkPeq(lhs, rhs ast.ASTHandle, indices []ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.syntax.manager
	sorts := make([]ast.ASTHandle, 0, len(indices)+2)
	for _, term := range append([]ast.ASTHandle{lhs, rhs}, indices...) {
		sort, err := m.GetSort(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sorts = append(sorts, sort)
	}
	name, err := m.Symbols.Intern(legacyPeqName)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, 0, len(indices)+2)
	args = append(args, lhs, rhs)
	args = append(args, indices...)
	return m.MkApp(decl, args)
}

// mkEq is peq::mk_eq: the equivalent equality
// lhs = (...(store (store rhs i0 v0) i1 v1)...) with fresh diff constants vi.
func (p *legacyArrayProjection) mkPeqEq(q legacyPeq, storesOnRHS bool) (eq ast.ASTHandle, auxConsts []ast.ASTHandle, err error) {
	m := p.syntax.manager
	lhs, rhs := q.lhs, q.rhs
	if !storesOnRHS {
		lhs, rhs = rhs, lhs
	}
	sort, err := m.GetSort(lhs)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	valSort, err := m.ArrayRange(sort)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	for _, index := range q.indices {
		val, err := m.MkFreshConst("diff", valSort)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		rhs, err = m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{rhs, index, val})
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		auxConsts = append(auxConsts, val)
	}
	eq, err = m.MkEq(lhs, rhs)
	return eq, auxConsts, err
}
