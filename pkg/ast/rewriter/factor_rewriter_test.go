package rewriter

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type factorFixture struct {
	t      *testing.T
	m      *ast.ASTManager
	family ast.FamilyID
	sort   ast.ASTHandle
}

func newFactorFixture(t *testing.T) factorFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	astTestPin(t, m, sort)
	return factorFixture{t, m, family, sort}
}

func (f factorFixture) variable(name string) ast.ASTHandle {
	f.t.Helper()
	h, err := f.m.MkConst(astTestSymbol(f.t, f.m, name), f.sort)
	astTestMust(f.t, err)
	astTestPin(f.t, f.m, h)
	return h
}

func (f factorFixture) numeral(value int64) ast.ASTHandle {
	f.t.Helper()
	q := f.m.NumeralService()
	n, err := q.NewValue()
	astTestMust(f.t, err)
	astTestMust(f.t, q.SetInt64(&n, value))
	defer func() { astTestMust(f.t, q.Del(&n)) }()
	h, err := f.m.ArithmeticNumeral(&n, true)
	astTestMust(f.t, err)
	astTestPin(f.t, f.m, h)
	return h
}

func (f factorFixture) app(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	h, err := f.m.MkFamilyApp(f.family, op, nil, args)
	astTestMust(f.t, err)
	astTestPin(f.t, f.m, h)
	return h
}

func (f factorFixture) relation(op int32, lhs, rhs ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
	f.t.Helper()
	var h ast.ASTHandle
	var err error
	if op < 0 {
		h, err = f.m.MkEq(lhs, rhs)
	} else {
		h, err = f.m.MkFamilyApp(f.family, op, nil, []ast.ASTHandle{lhs, rhs})
	}
	astTestMust(f.t, err)
	astTestPin(f.t, f.m, h)
	d, err := f.m.GetDecl(h)
	astTestMust(f.t, err)
	return h, d
}

// factorEqualitiesInPowersOrder orders eqs (eqs[i] belongs to factors[i])
// as factor_rewriter's m_powers obj_map iterates the factors.
func factorEqualitiesInPowersOrder(t *testing.T, f factorFixture, zero ast.ASTHandle, factors, eqs []ast.ASTHandle) []ast.ASTHandle {
	t.Helper()
	table := ast.NewObjMap(f.m)
	for _, h := range factors {
		entry, e := table.InsertIfNotThere(h)
		astTestMust(t, e)
		entry.Num++
	}
	var out []ast.ASTHandle
	for _, entry := range table.Entries() {
		for i, h := range factors {
			if h == entry.Key {
				out = append(out, eqs[i])
				break
			}
		}
	}
	return out
}

func TestFactorRewriterEqualitySingleAndCommonFactors(t *testing.T) {
	f := newFactorFixture(t)
	x, y, z, zero := f.variable("factor-x"), f.variable("factor-y"), f.variable("factor-z"), f.numeral(0)
	rw, err := NewFactorRewriter(f.m)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	out := astTestPin(t, f.m, ast.ASTHandle{})

	xy := f.app(factorOpMul, x, y)
	_, decl := f.relation(-1, xy, zero)
	status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{xy, zero}), out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("single monomial refused", status)
	}
	x0, err := f.m.MkEq(x, zero)
	astTestMust(t, err)
	y0, err := f.m.MkEq(y, zero)
	astTestMust(t, err)
	want, err := f.m.MkOrArray(factorEqualitiesInPowersOrder(t, f, zero, []ast.ASTHandle{x, y}, []ast.ASTHandle{x0, y0}))
	astTestMust(t, err)
	got, err := f.m.RefGet(out)
	astTestMust(t, err)
	if got != want {
		t.Fatal("single monomial factorization", got, want)
	}

	xz := f.app(factorOpMul, x, z)
	_, decl = f.relation(-1, xy, xz)
	status, err = rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{xy, xz}), out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("common factor refused", status)
	}
	minusZ := f.app(factorOpUMinus, z)
	residual := f.app(factorOpAdd, y, minusZ)
	r0, err := f.m.MkEq(residual, zero)
	astTestMust(t, err)
	want, err = f.m.MkOrArray(factorEqualitiesInPowersOrder(t, f, zero, []ast.ASTHandle{x, residual}, []ast.ASTHandle{x0, r0}))
	astTestMust(t, err)
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	if got != want {
		t.Fatal("common-factor residual", got, want)
	}

	two := f.numeral(2)
	negativeProduct := f.app(factorOpUMinus, f.app(factorOpMul, two, x))
	_, decl = f.relation(-1, negativeProduct, zero)
	status, err = rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{negativeProduct, zero}), out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("negative monomial refused", status)
	}
	minusTwo := f.numeral(-2)
	m20, err := f.m.MkEq(minusTwo, zero)
	astTestMust(t, err)
	want, err = f.m.MkOrArray(factorEqualitiesInPowersOrder(t, f, zero, []ast.ASTHandle{minusTwo, x}, []ast.ASTHandle{m20, x0}))
	astTestMust(t, err)
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	if got != want {
		t.Fatal("negative numeral factor", got, want)
	}
}

func TestFactorRewriterInequalityParityStrictnessAndReverse(t *testing.T) {
	f := newFactorFixture(t)
	x, y, zero := f.variable("factor-sign-x"), f.variable("factor-sign-y"), f.numeral(0)
	rw, err := NewFactorRewriter(f.m)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	out := astTestPin(t, f.m, ast.ASTHandle{})
	product := f.app(factorOpMul, x, x, y)
	x0, err := f.m.MkEq(zero, x)
	astTestMust(t, err)
	y0, err := f.m.MkEq(zero, y)
	astTestMust(t, err)
	yNeg := f.app(factorOpLT, y, zero)

	for _, tc := range []struct {
		op   int32
		lhs  ast.ASTHandle
		rhs  ast.ASTHandle
		want func() (ast.ASTHandle, error)
	}{
		{factorOpLE, product, zero, func() (ast.ASTHandle, error) { return f.m.MkOrArray([]ast.ASTHandle{x0, y0, yNeg}) }},
		{factorOpGE, zero, product, func() (ast.ASTHandle, error) { return f.m.MkOrArray([]ast.ASTHandle{x0, y0, yNeg}) }},
		{factorOpLT, product, zero, func() (ast.ASTHandle, error) {
			nx, e := f.m.MkNot(x0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			ny, e := f.m.MkNot(y0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return f.m.MkAndArray([]ast.ASTHandle{nx, ny, yNeg})
		}},
	} {
		_, decl := f.relation(tc.op, tc.lhs, tc.rhs)
		status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{tc.lhs, tc.rhs}), out)
		astTestMust(t, err)
		if status != RewriteDone {
			t.Fatal("inequality refused", tc.op, status)
		}
		want, err := tc.want()
		astTestMust(t, err)
		got, err := f.m.RefGet(out)
		astTestMust(t, err)
		if got != want {
			t.Fatal("inequality factorization", tc.op, got, want)
		}
	}
}

func TestFactorRewriterRefusalZeroDifferenceTraversalAndGuards(t *testing.T) {
	f := newFactorFixture(t)
	x, y, z, zero := f.variable("factor-guard-x"), f.variable("factor-guard-y"), f.variable("factor-guard-z"), f.numeral(0)
	rw, err := NewFactorRewriter(f.m)
	astTestMust(t, err)
	out := astTestPin(t, f.m, ast.ASTHandle{})
	sentinel, err := f.m.MkTrue()
	astTestMust(t, err)
	astTestMust(t, f.m.SetRef(out, sentinel))

	xy, xz := f.app(factorOpAdd, x, y), f.app(factorOpAdd, x, z)
	_, decl := f.relation(-1, xy, xz)
	status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{xy, xz}), out)
	astTestMust(t, err)
	got, err := f.m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteFailed || got != sentinel {
		t.Fatal("refusal changed output", status, got)
	}

	_, decl = f.relation(-1, zero, zero)
	status, err = rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{zero, zero}), out)
	astTestMust(t, err)
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || !f.m.IsTrue(got) {
		t.Fatal("empty equality difference", status, got)
	}

	product := f.app(factorOpMul, x, y)
	root, _ := f.relation(-1, product, zero)
	astTestMust(t, rw.Rewrite(root, out))
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	if family, e := f.m.GetFamilyID(got); e != nil || family != f.m.BasicFamilyID() {
		t.Fatal("traversal result", got, e)
	}
	if _, err = rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{zero}), out); ast.ASTSourceDebug && !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("arity guard", err)
	}
	copyOwner := *rw
	if err = copyOwner.Rewrite(root, out); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner accepted", err)
	}
	astTestMust(t, rw.Close())
	if err = rw.Rewrite(root, out); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed owner accepted", err)
	}
	astTestMust(t, rw.Close())
}

// AUDIT.md ast_rewriter_04-F1: factor_rewriter::mk_eq walks m_powers, an
// obj_map<expr, unsigned>, in table order rather than first-occurrence
// order.
func TestFactorRewriterPowersFollowObjMapOrder(t *testing.T) {
	f := newFactorFixture(t)
	a, b, c, d, zero := f.variable("factor-a"), f.variable("factor-b"), f.variable("factor-c"), f.variable("factor-d"), f.numeral(0)
	rw, err := NewFactorRewriter(f.m)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	out := astTestPin(t, f.m, ast.ASTHandle{})
	product := f.app(factorOpMul, a, b, c, d)
	_, decl := f.relation(-1, product, zero)
	status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{product, zero}), out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("product refused", status)
	}
	table := ast.NewObjMap(f.m)
	for _, h := range []ast.ASTHandle{a, b, c, d} {
		entry, e := table.InsertIfNotThere(h)
		astTestMust(t, e)
		entry.Num++
	}
	var eqs []ast.ASTHandle
	for _, entry := range table.Entries() {
		eq, e := f.m.MkEq(entry.Key, zero)
		astTestMust(t, e)
		eqs = append(eqs, eq)
	}
	want, err := f.m.MkOrArray(eqs)
	astTestMust(t, err)
	got, err := f.m.RefGet(out)
	astTestMust(t, err)
	if got != want {
		t.Fatal("powers order", got, want)
	}
}
