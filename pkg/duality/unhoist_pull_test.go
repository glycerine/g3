package duality

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDualityUnhoistPullTransitiveDefinitionsAndSharedParameters(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	sort, err := m.GetSort(raw(one))
	must(err)
	constant := func(text string) *Expr {
		name, err := m.Symbols.Intern(text)
		must(err)
		return wrap(m.MkConst(name, sort))
	}
	x, y, z, u, q, existing := constant("pull-x"), constant("pull-y"), constant("pull-z"), constant("pull-u"), constant("pull-q"), constant("pull-existing")
	add := func(a, b *Expr) *Expr { return wrap(m.MkArithmeticAdd(raw(a), raw(b))) }
	eq := func(a, b *Expr) *Expr { return wrap(m.MkEq(raw(a), raw(b))) }
	yz, up, uq := add(y, z), add(u, one), add(u, q)
	formula := wrap(m.MkAndArgs([]ast.ASTHandle{raw(eq(x, yz)), raw(eq(y, up)), raw(eq(z, uq))}))
	defs, err := GetDefs(formula)
	must(err)
	defer func() {
		for _, d := range defs {
			must(d.Close())
		}
	}()
	pull := &unhoistPull{context: ctx, definitions: defs, inverse: map[ast.ASTHandle]ast.ASTHandle{raw(q): raw(existing)}, constantParams: map[ast.ASTHandle]ast.ASTHandle{raw(existing): raw(q)}}
	defer pull.close()
	result, err := pull.pull(raw(x))
	must(err)
	if len(pull.newParams) != 1 {
		t.Fatal("shared undefined input generated duplicate parameters")
	}
	fresh := pull.newParams[0]
	if pull.inverse[raw(u)] != fresh || pull.constantParams[fresh] != raw(u) {
		t.Fatal("new parameter maps disagree")
	}
	if pull.inverse[raw(q)] != raw(existing) {
		t.Fatal("existing loop parameter replaced")
	}
	freshExpr, err := NewExpr(ctx, fresh)
	must(err)
	defer freshExpr.Close()
	expected := add(add(freshExpr, one), add(freshExpr, existing))
	if result != raw(expected) {
		t.Fatal("transitive initialization substitution mismatch")
	}
	repeated, err := pull.pull(raw(x))
	must(err)
	if repeated != result || len(pull.newParams) != 1 {
		t.Fatal("memoized pull generated extra parameters")
	}
	old, err := pull.pull(raw(q))
	must(err)
	if old != raw(existing) {
		t.Fatal("existing parameter not reused")
	}
	output, err := NewExpr(ctx, result)
	must(err)
	defer output.Close()
	must(pull.close())
	must(pull.close())
	if raw(output) != raw(expected) {
		t.Fatal("memo closure destroyed caller result")
	}
	// The C++ recursion has no result on cyclic definitions; diagnose this
	// explicitly instead of overflowing the Go stack.
	cycleFormula := wrap(m.MkAnd(raw(eq(x, y)), raw(eq(y, x))))
	cycles, err := GetDefs(cycleFormula)
	must(err)
	defer func() {
		for _, d := range cycles {
			must(d.Close())
		}
	}()
	cyclic := &unhoistPull{context: ctx, definitions: cycles}
	defer cyclic.close()
	if _, err := cyclic.pull(raw(x)); !errors.Is(err, ErrCyclicInitialization) {
		t.Fatalf("cyclic definitions: %v", err)
	}
	if len(cyclic.visiting) != 0 {
		t.Fatal("failed recursion left active markers")
	}
}
