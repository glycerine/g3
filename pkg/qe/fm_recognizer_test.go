package qe

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fmFixture struct {
	t                      *testing.T
	runtime                *memory.Runtime
	r                      fmRecognizer
	integer, real, boolean ast.ASTHandle
}

func fmMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFMFixture(t *testing.T) *fmFixture {
	return newFMFixtureProofs(t, false)
}

func newFMFixtureProofs(t *testing.T, proofs bool) *fmFixture {
	return newFMFixtureTransport(t, proofs, nil)
}

func newFMFixtureTransport(t *testing.T, proofs bool, transport func(memory.Failure) error) *fmFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 30)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	fmMust(t, err)
	integers := numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}
	var runtime *memory.Runtime
	if transport != nil {
		runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: transport})
		fmMust(t, err)
		integers.Memory = runtime
		integers.Profile = memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	}
	numerals, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: integers})
	fmMust(t, err)
	mode := ast.ASTProofDisabled
	if proofs {
		mode = ast.ASTProofEnabled
	}
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget, ProofMode: mode})
	fmMust(t, err)
	p, err := params.NewParamsContext(symbols, numerals, params.ParamsConfig{Budget: budget})
	fmMust(t, err)
	catalog, err := params.NewGlobalParamsCatalog(symbols, params.GlobalParamsCatalogConfig{Budget: budget})
	fmMust(t, err)
	g, err := params.NewGlobalParamsService(symbols, p, params.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	fmMust(t, err)
	t.Cleanup(func() {
		fmMust(t, m.Close())
		fmMust(t, g.Close())
		fmMust(t, catalog.Close())
		fmMust(t, p.Close())
		fmMust(t, numerals.Close())
		fmMust(t, symbols.Close())
		if runtime != nil {
			fmMust(t, runtime.Close())
		}
		if n := budget.Used(); n != 0 {
			t.Errorf("FM fixture retained %d bytes", n)
		}
	})
	fmMust(t, g.InitializeSource())
	previous, err := params.BindProcessGlobalParamsService(g)
	fmMust(t, err)
	t.Cleanup(func() { _, err := params.BindProcessGlobalParamsService(previous); fmMust(t, err) })
	coop, err := cooperate.NewCooperation(m.Budget())
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, coop.Close()) })
	v, err := NewVariableSetTest(m, []uint32{0, 1, 2, 3, 4, 5, 6, 7}, true)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, v.Close()) })
	family, err := m.RegisterArithmeticPlugin()
	fmMust(t, err)
	f := &fmFixture{t: t, runtime: runtime, r: fmRecognizer{manager: m, selection: v, config: Config{Context: g.Context, Globals: g, Cooperation: coop}, arithmetic: family, occ: true}}
	f.integer, err = m.MkFamilySort(family, 1, nil)
	fmMust(t, err)
	f.real, err = m.MkFamilySort(family, 0, nil)
	fmMust(t, err)
	f.boolean, err = m.MkBoolSort()
	fmMust(t, err)
	return f
}

func (f *fmFixture) own(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	fmMust(f.t, err)
	r, err := f.r.manager.NewRef(h)
	fmMust(f.t, err)
	f.t.Cleanup(func() { fmMust(f.t, f.r.manager.DelRef(&r)) })
	return h
}
func (f *fmFixture) variable(index uint32, sort ast.ASTHandle) ast.ASTHandle {
	return f.own(f.r.manager.MkVar(index, sort))
}
func (f *fmFixture) number(text string) ast.ASTHandle {
	return f.own(f.r.manager.ArithmeticIntegerNumeralText(text))
}
func (f *fmFixture) constant(name string, sort ast.ASTHandle) ast.ASTHandle {
	n, err := f.r.manager.Symbols.Intern(name)
	fmMust(f.t, err)
	return f.own(f.r.manager.MkConst(n, sort))
}
func (f *fmFixture) predicate(name string, args ...ast.ASTHandle) ast.ASTHandle {
	var sorts []ast.ASTHandle
	for _, arg := range args {
		s, err := f.r.manager.GetSort(arg)
		fmMust(f.t, err)
		sorts = append(sorts, s)
	}
	n, err := f.r.manager.Symbols.Intern(name)
	fmMust(f.t, err)
	d, err := f.r.manager.MkFuncDecl(n, sorts, f.boolean, ast.ASTFuncDeclInfo{})
	fmMust(f.t, err)
	return f.own(f.r.manager.MkApp(d, args))
}

func TestFMRecognitionSourceNormalForms(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x, y := f.variable(0, f.integer), f.variable(1, f.integer)
	zero, two := f.number("0"), f.number("2")
	p, q := f.constant("fm-p", f.boolean), f.constant("fm-q", f.boolean)
	linear := f.own(m.MkArithmeticLE(x, two))
	ge := f.own(m.MkArithmeticGE(x, two))
	neg := f.own(m.MkNot(linear))
	mul := f.own(m.MkArithmeticMul(two, x))
	reverseMul := f.own(m.MkArithmeticMul(x, two))
	add := f.own(m.MkArithmeticAdd(mul, y))
	duplicate := f.own(m.MkArithmeticAdd(mul, x))
	nested := f.own(m.MkArithmeticAdd(add, x))
	coerced := f.own(m.MkFamilyApp(f.r.arithmetic, 14, nil, []ast.ASTHandle{x}))
	double := f.own(m.MkArithmeticAdd(coerced, coerced))
	boundBool := f.variable(3, f.boolean)
	cases := []struct {
		name string
		h    ast.ASTHandle
		want bool
	}{
		{"le", linear, true}, {"ge", ge, true}, {"not-le", neg, true},
		{"coefficient-first", f.own(m.MkArithmeticLE(mul, two)), true},
		{"coefficient-last", f.own(m.MkArithmeticLE(reverseMul, two)), false},
		{"sum", f.own(m.MkArithmeticLE(add, two)), true},
		{"duplicate", f.own(m.MkArithmeticLE(duplicate, two)), false},
		{"nested-add", f.own(m.MkArithmeticLE(nested, two)), false},
		{"to-real", f.own(m.MkArithmeticLE(coerced, f.own(m.ArithmeticRealNumeralText("2")))), true},
		{"coercion-duplicate", f.own(m.MkArithmeticLE(double, f.own(m.ArithmeticRealNumeralText("2")))), false},
		{"constant-left", f.own(m.MkArithmeticLE(zero, two)), false},
		{"variable-right", f.own(m.MkArithmeticLE(x, y)), false},
		{"strict-lt", f.own(m.MkFamilyApp(f.r.arithmetic, 4, nil, []ast.ASTHandle{x, two})), false},
		{"unselected-index", f.own(m.MkArithmeticLE(f.variable(8, f.integer), two)), false},
		{"clause", f.own(m.MkOrArray([]ast.ASTHandle{p, neg, f.own(m.MkNot(q))})), true},
		{"two-constraints", f.own(m.MkOrArray([]ast.ASTHandle{p, linear, ge})), false},
		{"only-literals", f.own(m.MkOrArray([]ast.ASTHandle{p, q})), false},
		{"bound-bool-literal", f.own(m.MkOrArray([]ast.ASTHandle{boundBool, linear})), false},
		{"uf-predicate", f.own(m.MkOrArray([]ast.ASTHandle{f.predicate("fm-P", x), linear})), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.r.clause(tc.h)
			fmMust(t, err)
			if got != tc.want {
				t.Fatalf("recognition=%v, want %v", got, tc.want)
			}
		})
	}
	f.r.occ = false
	got, err := f.r.clause(cases[14].h)
	fmMust(t, err)
	if got {
		t.Fatal("disabled OCC accepted disjunction")
	}
}

func TestFMForbiddenSetInputOrderAndAnnotations(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x, y := f.variable(0, f.integer), f.variable(1, f.integer)
	p := f.predicate("fm-nonlinear", x)
	ineq := f.own(m.MkArithmeticLE(x, f.number("2")))
	mixed := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(x, y)), f.number("3")))
	fmMust(t, f.r.initForbidden([]ast.ASTHandle{ineq, p, mixed}))
	if !f.r.forbidden[0] || f.r.forbidden[1] {
		t.Fatal("unrelated variable forbidden", f.r.forbidden)
	}
	yes, err := f.r.clause(ineq)
	fmMust(t, err)
	if yes {
		t.Fatal("all-forbidden inequality accepted")
	}
	yes, err = f.r.clause(mixed)
	fmMust(t, err)
	if !yes {
		t.Fatal("mixed forbidden/eliminable inequality rejected")
	}
	// Patterns and no-patterns participate in quick_for_each_expr, with no
	// binder shift in this source pass. Boolean and unselected vars do not.
	pattern := f.own(m.MkPattern([]ast.ASTHandle{f.predicate("fm-trigger", f.variable(2, f.integer))}))
	name, err := m.Symbols.Intern("fm-bound")
	fmMust(t, err)
	truth := f.own(m.MkTrue())
	quant := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: truth, Patterns: []ast.ASTHandle{pattern}}))
	noPattern := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: truth, NoPatterns: []ast.ASTHandle{f.predicate("fm-no-trigger", f.variable(4, f.integer))}}))
	fmMust(t, f.r.initForbidden([]ast.ASTHandle{quant, noPattern, f.predicate("fm-bool", f.variable(5, f.boolean)), f.predicate("fm-outside", f.variable(8, f.integer))}))
	if len(f.r.forbidden) != 2 || !f.r.forbidden[2] || !f.r.forbidden[4] {
		t.Fatal("annotation traversal or selector", f.r.forbidden)
	}
	f.r.realOnly = true
	f.r.forbidden = nil
	yes, err = f.r.clause(ineq)
	fmMust(t, err)
	if yes {
		t.Fatal("integer accepted in real-only mode")
	}
	real := f.variable(6, f.real)
	yes, err = f.r.clause(f.own(m.MkArithmeticLE(real, f.own(m.ArithmeticRealNumeralText("3")))))
	fmMust(t, err)
	if !yes {
		t.Fatal("real variable rejected")
	}
}

func TestFMRecognitionSourceTraceAndClosedStream(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(map[bool]string{false: "trace", true: "closed"}[closed], func(t *testing.T) {
			f := newFMFixture(t)
			var out bytes.Buffer
			var writer io.Writer = &out
			if closed {
				writer = fmClosedTrace{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("qe_lite")
			t.Cleanup(tags.Finalize)
			f.r.config.Trace = tags
			x := f.variable(0, f.integer)
			ineq := f.own(f.r.manager.MkArithmeticLE(x, f.number("2")))
			neg := f.own(f.r.manager.MkNot(ineq))
			fmMust(t, f.r.initForbidden([]ast.ASTHandle{neg, f.predicate("fm-trace", x)}))
			if !f.r.forbidden[0] {
				t.Fatal("trace changed source traversal")
			}
			if !closed && tags.ShouldTrace("qe_lite") {
				for _, text := range []string{"src/qe/qe_lite.cpp:1183", " true\n", "src/qe/qe_lite.cpp:1523", "OCC: ", "src/qe/qe_lite.cpp:1526", "not OCC:\n"} {
					if !strings.Contains(out.String(), text) {
						t.Fatal("missing source trace", text, out.String())
					}
				}
			}
		})
	}
}

type fmClosedTrace struct{}

func (fmClosedTrace) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
