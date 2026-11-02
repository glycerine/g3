package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestProjectionRedundantIntegerBoundsAndOwnership(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		term, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, term)
		return term
	}
	raw := func(term *Expr) ast.ASTHandle { t.Helper(); h, err := term.Raw(); must(err); return h }
	family, err := m.ArithmeticFamilyID()
	must(err)
	num := func(text string) *Expr { return wrap(m.ArithmeticIntegerNumeralText(text)) }
	zero, one, two := num("0"), num("1"), num("2")
	sort, err := m.GetSort(raw(zero))
	must(err)
	name, err := m.Symbols.Intern("bound-x")
	must(err)
	x := wrap(m.MkConst(name, sort))
	app := func(op int32, a, b *Expr) *Expr {
		return wrap(m.MkFamilyApp(family, op, nil, []ast.ASTHandle{raw(a), raw(b)}))
	}
	lower0, lower2 := app(2, zero, x), app(3, x, two)
	upper0, upper2 := app(2, x, zero), app(2, x, two)
	negatedLower := wrap(m.MkNot(raw(app(2, x, one))))
	for _, tc := range []struct {
		name  string
		input []*Expr
		want  *Expr
	}{
		{"stronger later", []*Expr{lower0, lower2}, lower2},
		{"stronger earlier", []*Expr{lower2, lower0}, lower2},
		{"upper bound", []*Expr{upper0, upper2}, upper0},
		{"duplicate", []*Expr{lower2, lower2}, lower2},
		{"negated integer bound", []*Expr{lower0, negatedLower}, lower2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := graph.ElimRedundantBounds(tc.input, simplifier)
			must(err)
			defer func() {
				for _, term := range output {
					must(term.Close())
				}
			}()
			if len(output) != len(tc.input) {
				t.Fatal("literal positions changed")
			}
			args := make([]ast.ASTHandle, len(output))
			for i, term := range output {
				args[i] = raw(term)
			}
			conjunction := wrap(m.MkAndArgs(args))
			difference := wrap(m.MkNot(raw(wrap(m.MkIff(raw(conjunction), raw(tc.want))))))
			must(native.Push())
			must(native.Assert(raw(difference)))
			status, err := native.CheckSat(nil)
			must(err)
			must(native.Pop(1))
			if status != lbool.False {
				t.Fatal("normalized bounds differ", status)
			}
		})
	}
	baseline := m.Stats().LiveReferences
	completed := false
	for fault := int64(0); fault < 1024; fault++ {
		m.Budget().FailAfter(fault)
		output, failure := graph.ElimRedundantBounds([]*Expr{lower0, lower2}, simplifier)
		m.Budget().FailAfter(-1)
		if failure == nil {
			for _, term := range output {
				must(term.Close())
			}
			completed = true
		} else {
			if output != nil {
				t.Fatal("partial bounds", fault, failure)
			}
			retry, err := graph.ElimRedundantBounds([]*Expr{lower0, lower2}, simplifier)
			must(err)
			for _, term := range retry {
				must(term.Close())
			}
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatal("bound reference restoration", fault, got, baseline)
		}
		if completed {
			break
		}
	}
	if !completed {
		t.Fatal("bound allocation sweep never succeeded")
	}
}

func TestFourierMotzkinCombinesBounds(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	family, err := m.ArithmeticFamilyID()
	must(err)
	zero := wrap(m.ArithmeticIntegerNumeralText("0"))
	sort, err := m.GetSort(raw(zero))
	must(err)
	xn, err := m.Symbols.Intern("fourier-x")
	must(err)
	yn, err := m.Symbols.Intern("fourier-y")
	must(err)
	x := wrap(m.MkConst(xn, sort))
	y := wrap(m.MkConst(yn, sort))
	lower := wrap(m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{raw(x), raw(zero)}))
	upper := wrap(m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{raw(x), raw(y)}))
	expected := wrap(m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{raw(y), raw(zero)}))
	output, err := graph.FourierMotzkinCheap([]*Expr{y}, []*Expr{lower, upper}, simplifier)
	must(err)
	defer func() {
		for _, e := range output {
			must(e.Close())
		}
	}()
	args := make([]ast.ASTHandle, len(output))
	for i, e := range output {
		args[i] = raw(e)
	}
	combined := wrap(m.MkAndArgs(args))
	iff := wrap(m.MkIff(raw(combined), raw(expected)))
	neg := wrap(m.MkNot(raw(iff)))
	must(native.Push())
	must(native.Assert(raw(neg)))
	status, err := native.CheckSat(nil)
	must(err)
	if status != lbool.False {
		t.Fatal("Fourier projection differs", status)
	}
	must(native.Pop(1))
	two := wrap(m.ArithmeticIntegerNumeralText("2"))
	minusOne := wrap(m.ArithmeticIntegerNumeralText("-1"))
	mul := func(a, b *Expr) *Expr { return wrap(m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{raw(a), raw(b)})) }
	cmp := func(op int32, a, b *Expr) *Expr {
		return wrap(m.MkFamilyApp(family, op, nil, []ast.ASTHandle{raw(a), raw(b)}))
	}
	twiceX, twiceY := mul(two, x), mul(two, y)
	negX, negY := mul(minusOne, x), mul(minusOne, y)
	for _, tc := range []struct {
		name         string
		lower, upper *Expr
	}{
		{"positive coefficients", cmp(3, twiceX, zero), cmp(2, twiceX, twiceY)},
		{"negative coefficients", cmp(2, negX, zero), cmp(3, negX, negY)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projected, err := graph.FourierMotzkinCheap([]*Expr{y}, []*Expr{tc.lower, tc.upper}, simplifier)
			must(err)
			defer func() {
				for _, term := range projected {
					must(term.Close())
				}
			}()
			args := make([]ast.ASTHandle, len(projected))
			for i, term := range projected {
				args[i] = raw(term)
			}
			actual := wrap(m.MkAndArgs(args))
			iff := wrap(m.MkIff(raw(actual), raw(expected)))
			neg := wrap(m.MkNot(raw(iff)))
			must(native.Push())
			must(native.Assert(raw(neg)))
			status, err := native.CheckSat(nil)
			must(err)
			must(native.Pop(1))
			if status != lbool.False {
				t.Fatal("coefficient projection differs", status)
			}
		})
	}
	// Integer normalization must detect this impossible unit interval.
	one := wrap(m.ArithmeticIntegerNumeralText("1"))
	impossibleLower, impossibleUpper := cmp(3, twiceX, one), cmp(2, twiceX, one)
	must(native.Push())
	must(native.Assert(raw(impossibleLower)))
	must(native.Assert(raw(impossibleUpper)))
	status, err = native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("integer witness unexpectedly exists", status)
	}
	approximate, err := graph.FourierMotzkinCheap(nil, []*Expr{impossibleLower, impossibleUpper}, simplifier)
	must(err)
	defer func() {
		for _, term := range approximate {
			must(term.Close())
		}
	}()
	must(native.Push())
	for _, term := range approximate {
		must(native.Assert(raw(term)))
	}
	status, err = native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("impossible integer interval was lost", status)
	}

}

func TestProjectFormulaCombinesEqualityAndInequalityProjection(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	family, err := m.ArithmeticFamilyID()
	must(err)
	zero := wrap(m.ArithmeticIntegerNumeralText("0"))
	sort, err := m.GetSort(raw(zero))
	must(err)
	makeVar := func(name string) *Expr {
		sym, err := m.Symbols.Intern(name)
		must(err)
		return wrap(m.MkConst(sym, sort))
	}
	x, y, z := makeVar("project-combined-x"), makeVar("project-combined-y"), makeVar("project-combined-z")
	eq := wrap(m.MkEq(raw(x), raw(y)))
	lower := wrap(m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{raw(y), raw(zero)}))
	upper := wrap(m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{raw(y), raw(z)}))
	formula := wrap(m.MkAndArgs([]ast.ASTHandle{raw(eq), raw(lower), raw(upper)}))
	expected := wrap(m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{raw(z), raw(zero)}))
	output, err := graph.ProjectFormula([]*Expr{z}, formula, simplifier)
	must(err)
	defer output.Close()
	iff := wrap(m.MkIff(raw(output), raw(expected)))
	neg := wrap(m.MkNot(raw(iff)))
	must(native.Assert(raw(neg)))
	status, err := native.CheckSat(nil)
	must(err)
	if status != lbool.False {
		t.Fatal("combined projection", status)
	}
	baseline := m.Stats().LiveReferences
	complete := false
	for n := int64(0); n < 1024; n++ {
		m.Budget().FailAfter(n)
		trial, failure := graph.ProjectFormula([]*Expr{z}, formula, simplifier)
		m.Budget().FailAfter(-1)
		if failure == nil {
			must(trial.Close())
			complete = true
		} else {
			if trial != nil {
				t.Fatal("partial projection", n, failure)
			}
			retry, err := graph.ProjectFormula([]*Expr{z}, formula, simplifier)
			must(err)
			must(retry.Close())
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatal("projection references", n, got, baseline)
		}
		if complete {
			break
		}
	}
	if !complete {
		t.Fatal("projection allocation sweep never completed")
	}
}

func TestFourierMotzkinKeepsProtectedAndRepeatedVariables(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	family, err := m.ArithmeticFamilyID()
	must(err)
	zero := wrap(m.ArithmeticIntegerNumeralText("0"))
	one := wrap(m.ArithmeticIntegerNumeralText("1"))
	sort, err := m.GetSort(raw(zero))
	must(err)
	name, err := m.Symbols.Intern("fourier-protected-x")
	must(err)
	x := wrap(m.MkConst(name, sort))
	lower := wrap(m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{raw(x), raw(zero)}))
	upper := wrap(m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{raw(x), raw(one)}))
	fn, err := m.Symbols.Intern("fourier-protected-f")
	must(err)
	decl := wrapDeclForProjectionTest(t, m, fn, sort)
	fx := wrap(m.MkApp(decl, []ast.ASTHandle{raw(x)}))
	sum := wrap(m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{raw(x), raw(fx)}))
	repeated := wrap(m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{raw(sum), raw(zero)}))
	for _, tc := range []struct {
		name        string
		keep, terms []*Expr
	}{{"kept", []*Expr{x}, []*Expr{lower, upper}}, {"repeated", nil, []*Expr{repeated, upper}}} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := graph.FourierMotzkinCheap(tc.keep, tc.terms, simplifier)
			must(err)
			defer func() {
				for _, e := range output {
					must(e.Close())
				}
			}()
			before := make([]ast.ASTHandle, len(tc.terms))
			after := make([]ast.ASTHandle, len(output))
			for i, e := range tc.terms {
				before[i] = raw(e)
			}
			for i, e := range output {
				after[i] = raw(e)
			}
			a := wrap(m.MkAndArgs(before))
			b := wrap(m.MkAndArgs(after))
			iff := wrap(m.MkIff(raw(a), raw(b)))
			neg := wrap(m.MkNot(raw(iff)))
			must(native.Push())
			must(native.Assert(raw(neg)))
			status, err := native.CheckSat(nil)
			must(err)
			must(native.Pop(1))
			if status != lbool.False {
				t.Fatal("protected formula changed", status)
			}
		})
	}
}

func wrapDeclForProjectionTest(t *testing.T, m *ast.ASTManager, name usymbol.Symbol, sort ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := m.NewRef(decl)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.DelRef(&ref); err != nil {
			t.Fatal(err)
		}
	})
	return decl
}
