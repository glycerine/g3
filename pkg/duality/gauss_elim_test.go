package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestGaussElimCheapDependenciesKeptVariablesAndCycles(t *testing.T) {
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
	x, err := ctx.BoolConst("project-x")
	must(err)
	closeExpr(t, x)
	y, err := ctx.BoolConst("project-y")
	must(err)
	closeExpr(t, y)
	z, err := ctx.BoolConst("project-z")
	must(err)
	closeExpr(t, z)
	truth, err := ctx.BoolVal(true)
	must(err)
	closeExpr(t, truth)
	sort, err := x.Sort()
	must(err)
	defer sort.Close()
	name, err := ctx.StringSymbol("project-f")
	must(err)
	f, err := ctx.Function(name, []*Sort{sort}, sort)
	must(err)
	closeDecl(t, f)
	fx, err := f.Call(x)
	must(err)
	closeExpr(t, fx)
	fy, err := f.Call(y)
	must(err)
	closeExpr(t, fy)
	ft, err := f.Call(truth)
	must(err)
	closeExpr(t, ft)
	eq := func(a, b *Expr) *Expr { return wrap(m.MkEq(raw(a), raw(b))) }
	chain := []*Expr{eq(x, y), eq(y, z), eq(z, truth)}
	for _, tc := range []struct {
		name       string
		keep, lits []*Expr
		want       *Expr
	}{
		{"kept chain", []*Expr{x}, chain, x},
		{"eliminated chain", nil, chain, truth},
		{"dependent term", []*Expr{x}, []*Expr{eq(x, fy), eq(y, truth)}, eq(x, ft)},
		{"cyclic definition", nil, []*Expr{eq(x, fx)}, eq(x, fx)},
		{"unconstrained alias", nil, []*Expr{eq(x, y)}, truth},
		{"kept literal", []*Expr{x}, []*Expr{x}, x},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := graph.GaussElimCheap(tc.keep, tc.lits, simplifier)
			must(err)
			defer func() {
				for _, term := range output {
					must(term.Close())
				}
			}()
			formula := raw(truth)
			if len(output) == 1 {
				formula = raw(output[0])
			} else if len(output) > 1 {
				args := make([]ast.ASTHandle, len(output))
				for i, term := range output {
					args[i] = raw(term)
				}
				formula = raw(wrap(m.MkAndArgs(args)))
			}
			iff := wrap(m.MkIff(formula, raw(tc.want)))
			negated := wrap(m.MkNot(raw(iff)))
			must(native.Push())
			must(native.Assert(raw(negated)))
			status, err := native.CheckSat(nil)
			must(err)
			must(native.Pop(1))
			if status != lbool.False {
				t.Fatal("projection differs from expected formula", status)
			}
		})
	}
	baseline := m.Stats().LiveReferences
	completed := false
	for fault := int64(0); fault < 1024; fault++ {
		m.Budget().FailAfter(fault)
		output, failure := graph.GaussElimCheap([]*Expr{x}, chain, simplifier)
		m.Budget().FailAfter(-1)
		if failure == nil {
			for _, term := range output {
				must(term.Close())
			}
			completed = true
		} else {
			if output != nil {
				t.Fatal("partial Gaussian projection", fault, failure)
			}
			retry, err := graph.GaussElimCheap([]*Expr{x}, chain, simplifier)
			must(err)
			for _, term := range retry {
				must(term.Close())
			}
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatal("projection reference restoration", fault, got, baseline)
		}
		if completed {
			break
		}
	}
	if !completed {
		t.Fatal("projection allocation sweep never succeeded")
	}
}
