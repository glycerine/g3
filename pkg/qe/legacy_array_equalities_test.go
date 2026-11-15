package qe

// Source: src/muz/spacer/spacer_qe_project.cpp array_project_eqs_util and
// array_project. The expected texts are what a harness linked against the
// reference build's libz3.a printed for qe::array_project_eqs and
// qe::array_project on the same terms and models (AUDIT.md qe_03-F4).

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type legacyArrayEqsCase struct {
	name      string
	eqs, full string
	build     func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle)
}

type legacyArrayEqsEnv struct {
	t  *testing.T
	f  *fmFixture
	m  *ast.ASTManager
	p  *legacyArrayProjection
	md *model.Model
	A  ast.ASTHandle
}

func (e *legacyArrayEqsEnv) arr(name string) ast.ASTHandle { return e.f.constant(name, e.A) }
func (e *legacyArrayEqsEnv) in(name string) ast.ASTHandle  { return e.f.constant(name, e.f.integer) }
func (e *legacyArrayEqsEnv) num(text string) ast.ASTHandle { return e.f.number(text) }
func (e *legacyArrayEqsEnv) k(text string) ast.ASTHandle {
	return e.f.own(e.m.MkConstArray(e.A, e.num(text)))
}
func (e *legacyArrayEqsEnv) st(a, i, v ast.ASTHandle) ast.ASTHandle {
	return e.f.own(e.m.MkFamilyApp(e.p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, v}))
}
func (e *legacyArrayEqsEnv) sel(a, i ast.ASTHandle) ast.ASTHandle {
	return e.f.own(e.m.MkFamilyApp(e.p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
}
func (e *legacyArrayEqsEnv) eq(a, b ast.ASTHandle) ast.ASTHandle { return e.f.own(e.m.MkEq(a, b)) }
func (e *legacyArrayEqsEnv) and(a, b ast.ASTHandle) ast.ASTHandle {
	return e.f.own(e.m.MkAnd(a, b))
}
func (e *legacyArrayEqsEnv) set(c, v ast.ASTHandle) {
	decl, err := e.m.GetDecl(c)
	fmMust(e.t, err)
	fmMust(e.t, e.md.Core().RegisterConstant(decl, v))
}

func (e *legacyArrayEqsEnv) render(formula ast.ASTHandle, remaining, aux []ast.ASTHandle) string {
	var b strings.Builder
	b.WriteString("fml: ")
	fmMust(e.t, writeExpression(e.m, e.p.syntax.config, &b, formula))
	b.WriteString("\nremaining:")
	for _, v := range remaining {
		b.WriteString(" ")
		fmMust(e.t, writeExpression(e.m, e.p.syntax.config, &b, v))
	}
	b.WriteString("\naux:")
	for _, v := range aux {
		b.WriteString(" ")
		fmMust(e.t, writeExpression(e.m, e.p.syntax.config, &b, v))
		decl, err := e.m.GetDecl(v)
		fmMust(e.t, err)
		value, err := e.md.Core().ConstantInterpretation(decl)
		fmMust(e.t, err)
		b.WriteString("=")
		fmMust(e.t, writeExpression(e.m, e.p.syntax.config, &b, value))
	}
	b.WriteString("\n")
	return b.String()
}

var legacyArrayEqsCases = []legacyArrayEqsCase{
	{
		// The LIFO traversal finds (= v t2) first; C++ substitutes t2.
		name: "two-direct",
		eqs:  "fml: (= t2 t1)\nremaining:\naux:\n",
		full: "fml: (and (= t2 t1))\nremaining:\naux:\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, t1, t2 := e.arr("v"), e.arr("t1"), e.arr("t2")
			e.set(v, e.k("0"))
			e.set(t1, e.k("0"))
			e.set(t2, e.k("0"))
			return e.and(e.eq(v, t1), e.eq(v, t2)), []ast.ASTHandle{v}
		},
	},
	{
		// A trivial peq yields no substitution: v remains, with the aux
		// literal and no diff constant.
		name: "store-of-self",
		eqs:  "fml: (= (select v i) x)\nremaining: v\naux:\n",
		full: "fml: (and (= sel!0 x))\nremaining:\naux: sel!0=0\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, i, x := e.arr("v"), e.in("i"), e.in("x")
			e.set(v, e.k("0"))
			e.set(i, e.num("1"))
			e.set(x, e.num("0"))
			return e.eq(e.st(v, i, x), v), []ast.ASTHandle{v}
		},
	},
	{
		name: "store-equality",
		eqs:  "fml: (= (select b i) 7)\nremaining:\naux: diff!0=0\n",
		full: "fml: (and (= (select b i) 7))\nremaining:\naux: diff!0=0\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, b, i := e.arr("v"), e.arr("b"), e.in("i")
			e.set(v, e.k("0"))
			e.set(b, e.st(e.k("0"), e.num("1"), e.num("7")))
			e.set(i, e.num("1"))
			return e.eq(e.st(v, i, e.num("7")), b), []ast.ASTHandle{v}
		},
	},
	{
		name: "select-index",
		eqs:  "fml: (let ((a!1 (= (select b (select (store b sel!0 diff!1) k)) 7)))\n  (and a!1 (= sel!0 (select (store b sel!0 diff!1) k))))\nremaining:\naux: sel!0=0 diff!1=0\n",
		full: "fml: (let ((a!1 (= (select b (select (store b sel!0 diff!1) k)) 7)))\n  (and a!1 (= sel!0 (select (store b sel!0 diff!1) k))))\nremaining:\naux: sel!0=0 diff!1=0\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, b, k := e.arr("v"), e.arr("b"), e.in("k")
			e.set(v, e.k("0"))
			e.set(b, e.st(e.k("0"), e.num("0"), e.num("7")))
			e.set(k, e.num("1"))
			return e.eq(e.st(v, e.sel(v, k), e.num("7")), b), []ast.ASTHandle{v}
		},
	},
	{
		// The model-false (= v c) is substituted, not replaced by false.
		name: "store-and-false-equality",
		eqs:  "fml: (and (= (select b i) 7) (or (= (store b i diff!0) c) q))\nremaining:\naux: diff!0=0\n",
		full: "fml: (and (= (select b i) 7) (or (= (store b i diff!0) c) q))\nremaining:\naux: diff!0=0\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, b, c, i := e.arr("v"), e.arr("b"), e.arr("c"), e.in("i")
			q := e.f.constant("q", e.f.boolean)
			e.set(v, e.k("0"))
			e.set(b, e.st(e.k("0"), e.num("1"), e.num("7")))
			e.set(c, e.k("1"))
			e.set(i, e.num("1"))
			e.set(q, e.f.own(e.m.MkTrue()))
			return e.and(e.eq(e.st(v, i, e.num("7")), b), e.f.own(e.m.MkOr(e.eq(v, c), q))), []ast.ASTHandle{v}
		},
	},
	{
		name: "false-equality",
		eqs:  "fml: true\nremaining: v\naux:\n",
		full: "fml: (and true)\nremaining:\naux:\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, b, i := e.arr("v"), e.arr("b"), e.in("i")
			e.set(v, e.k("0"))
			e.set(b, e.k("1"))
			e.set(i, e.num("0"))
			return e.f.own(e.m.MkOr(e.eq(v, b), e.eq(i, i))), []ast.ASTHandle{v}
		},
	},
	{
		// Depth 1 (= v u) sorts before the depth 3 store chain.
		name: "nesting-depth-order",
		eqs:  "fml: (= (store (store u i x) j y) w)\nremaining:\naux:\n",
		full: "fml: (and (= (store (store u i x) j y) w))\nremaining:\naux:\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, w, u := e.arr("v"), e.arr("w"), e.arr("u")
			i, j, x, y := e.in("i"), e.in("j"), e.in("x"), e.in("y")
			e.set(v, e.k("0"))
			e.set(u, e.k("0"))
			e.set(i, e.num("1"))
			e.set(j, e.num("2"))
			e.set(x, e.num("3"))
			e.set(y, e.num("4"))
			e.set(w, e.st(e.st(e.k("0"), e.num("1"), e.num("3")), e.num("2"), e.num("4")))
			return e.and(e.eq(e.st(e.st(v, i, x), j, y), w), e.eq(v, u)), []ast.ASTHandle{v}
		},
	},
	{
		name: "two-variables",
		eqs:  "fml: (and (= (select w i) x) (= (select w j) 3))\nremaining: w\naux: diff!0=3\n",
		full: "fml: (and (< i j) (= sel!2 x) (= sel!1 3))\nremaining:\naux: diff!0=3 sel!1=3 sel!2=5\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, w, i, j, x := e.arr("v"), e.arr("w"), e.in("i"), e.in("j"), e.in("x")
			e.set(v, e.k("3"))
			e.set(w, e.st(e.k("3"), e.num("1"), e.num("5")))
			e.set(i, e.num("1"))
			e.set(j, e.num("2"))
			e.set(x, e.num("5"))
			return e.and(e.eq(e.st(v, i, x), w), e.eq(e.sel(w, j), e.num("3"))), []ast.ASTHandle{v, w}
		},
	},
	{
		// The outer store index equals a diff index in the model.
		name: "index-in-diff-indices",
		eqs:  "fml: (and (= i j) (= (select t j) y))\nremaining:\naux: diff!0=0\n",
		full: "fml: (and (= i j) (= (select t j) y))\nremaining:\naux: diff!0=0\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, t, i, j, x, y := e.arr("v"), e.arr("t"), e.in("i"), e.in("j"), e.in("x"), e.in("y")
			e.set(v, e.k("0"))
			e.set(i, e.num("1"))
			e.set(j, e.num("1"))
			e.set(x, e.num("3"))
			e.set(y, e.num("4"))
			e.set(t, e.st(e.k("0"), e.num("1"), e.num("4")))
			return e.eq(e.st(e.st(v, i, x), j, y), t), []ast.ASTHandle{v}
		},
	},
	{
		// v on both sides: depth 0, a trivial peq and no substitution.
		name: "both-sides",
		eqs:  "fml: (and (not (= j i)) (= (select (store v j y) i) x) (= (select v j) y))\nremaining: v\naux:\n",
		full: "fml: (and (< i j) (not (= j i)) (= sel!1 x) (= sel!0 y))\nremaining:\naux: sel!0=0 sel!1=0\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, i, j, x, y := e.arr("v"), e.in("i"), e.in("j"), e.in("x"), e.in("y")
			e.set(v, e.k("0"))
			e.set(i, e.num("1"))
			e.set(j, e.num("2"))
			e.set(x, e.num("0"))
			e.set(y, e.num("0"))
			return e.eq(e.st(v, i, x), e.st(v, j, y)), []ast.ASTHandle{v}
		},
	},
	{
		name: "stores-on-rhs",
		eqs:  "fml: (and (= (select t i) x) (not (= (store t i diff!0) t2)))\nremaining:\naux: diff!0=0\n",
		full: "fml: (and (= (select t i) x) (not (= (store t i diff!0) t2)))\nremaining:\naux: diff!0=0\n",
		build: func(e *legacyArrayEqsEnv) (ast.ASTHandle, []ast.ASTHandle) {
			v, t, t2, i, x := e.arr("v"), e.arr("t"), e.arr("t2"), e.in("i"), e.in("x")
			e.set(v, e.k("0"))
			e.set(i, e.num("1"))
			e.set(x, e.num("2"))
			e.set(t, e.st(e.k("0"), e.num("1"), e.num("2")))
			e.set(t2, e.k("5"))
			return e.and(e.eq(t, e.st(v, i, x)), e.f.own(e.m.MkNot(e.eq(v, t2)))), []ast.ASTHandle{v}
		},
	},
}

func newLegacyArrayEqsEnv(t *testing.T) *legacyArrayEqsEnv {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, p.Close()) })
	md, err := model.NewModel(m)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, md.Close()) })
	e := &legacyArrayEqsEnv{t: t, f: f, m: m, p: p, md: md}
	e.A = f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	return e
}

func TestLegacyArrayProjectEqsMatchesSource(t *testing.T) {
	for _, c := range legacyArrayEqsCases {
		t.Run(c.name, func(t *testing.T) {
			e := newLegacyArrayEqsEnv(t)
			formula, variables := c.build(e)
			current, err := e.m.NewRef(formula)
			fmMust(t, err)
			defer func() { fmMust(t, e.m.DelRef(&current)) }()
			result := &legacyArrayProjectionResult{manager: e.m}
			defer func() { fmMust(t, result.Close()) }()
			remaining, err := e.p.projectEqualities(e.md.Core(), variables, current, result)
			fmMust(t, err)
			handle, err := e.m.RefGet(current)
			fmMust(t, err)
			var aux []ast.ASTHandle
			for _, ref := range result.variables {
				h, err := e.m.RefGet(ref)
				fmMust(t, err)
				aux = append(aux, h)
			}
			if got := e.render(handle, remaining, aux); got != c.eqs {
				t.Fatalf("array_project_eqs:\n got: %q\nwant: %q", got, c.eqs)
			}
		})
	}
}

func TestLegacyArrayProjectionMatchesSource(t *testing.T) {
	for _, c := range legacyArrayEqsCases {
		t.Run(c.name, func(t *testing.T) {
			e := newLegacyArrayEqsEnv(t)
			formula, variables := c.build(e)
			result, err := e.p.projectFormula(e.md.Core(), variables, formula, false)
			fmMust(t, err)
			defer func() { fmMust(t, result.Close()) }()
			handle, err := e.m.RefGet(result.formula)
			fmMust(t, err)
			var aux []ast.ASTHandle
			for _, ref := range result.variables {
				h, err := e.m.RefGet(ref)
				fmMust(t, err)
				aux = append(aux, h)
			}
			if got := e.render(handle, nil, aux); got != c.full {
				t.Fatalf("array_project:\n got: %q\nwant: %q", got, c.full)
			}
		})
	}
}

// find_arr_eqs pops the last pushed argument first, so sibling equalities
// come out in reverse order; selects block store propagation.
func TestLegacyArrayFindArrEqsOrderAndMarks(t *testing.T) {
	e := newLegacyArrayEqsEnv(t)
	a, b := e.arr("a"), e.arr("b")
	zero, one := e.num("0"), e.num("1")
	stored := e.st(a, zero, one)
	storeEquality := e.eq(stored, b)
	directEquality := e.eq(a, b)
	selected := e.sel(stored, zero)
	selectEquality := e.eq(selected, one)
	formula := e.f.own(e.m.MkAndArray([]ast.ASTHandle{storeEquality, directEquality, selectEquality}))
	u, err := newLegacyArrayProjectEqs(e.p)
	fmMust(t, err)
	defer func() { fmMust(t, u.Close()) }()
	u.v = a
	refs := &legacyArrayRefs{manager: e.m}
	defer func() { fmMust(t, refs.close()) }()
	eqs, err := u.findArrEqs(formula, refs)
	fmMust(t, err)
	if len(eqs) != 2 || eqs[0] != directEquality || eqs[1] != storeEquality {
		t.Fatal("find_arr_eqs order", eqs)
	}
	if !u.hasStores[stored] || !u.hasStores[storeEquality] || u.hasStores[selected] || u.hasStores[selectEquality] {
		t.Fatal("store propagation marks", u.hasStores[stored], u.hasStores[storeEquality], u.hasStores[selected], u.hasStores[selectEquality])
	}
}

func TestLegacyPeqApplicationAndExpansion(t *testing.T) {
	e := newLegacyArrayEqsEnv(t)
	a, b, i, j := e.arr("a"), e.arr("b"), e.in("i"), e.in("j")
	application, err := e.p.mkPeq(a, b, []ast.ASTHandle{i, j})
	fmMust(t, err)
	application = e.f.own(application, nil)
	q, err := e.p.legacyPeqOf(application)
	fmMust(t, err)
	if q.lhs != a || q.rhs != b || len(q.indices) != 2 || q.indices[0] != i || q.indices[1] != j {
		t.Fatal("partial equality shape")
	}
	for _, storesOnRHS := range []bool{true, false} {
		equality, constants, err := e.p.mkPeqEq(q, storesOnRHS)
		fmMust(t, err)
		equality = e.f.own(equality, nil)
		if len(constants) != 2 {
			t.Fatal("partial equality diff constant count", len(constants))
		}
		lhs, base := a, b
		if !storesOnRHS {
			lhs, base = b, a
		}
		args, err := (&legacyArrayProjectEqs{m: e.m}).args(equality)
		fmMust(t, err)
		if args[0] != lhs {
			t.Fatal("partial equality expansion lhs")
		}
		depth, current := 0, args[1]
		for {
			storeArgs, store, err := e.p.arrayOp(current, ast.ASTArrayOpStore)
			fmMust(t, err)
			if !store {
				break
			}
			depth++
			current = storeArgs[0]
		}
		if depth != 2 || current != base {
			t.Fatal("partial equality expansion store chain", depth)
		}
	}
}
