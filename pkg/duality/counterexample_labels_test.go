package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"reflect"
	"testing"
	"time"
)

func TestCounterexampleLabelsBranchOrderPolarityAndSharing(t *testing.T) {
	ctx, m, solver, globals := dualityNativeSolverServicesFixture(t)
	g, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if err := g.BindSolver(solver); err != nil {
		t.Fatal(err)
	}
	if err := g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)); err != nil {
		t.Fatal(err)
	}
	if status, err := g.Check(nil, nil, nil); err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	var pins []*Expr
	defer func() {
		for _, p := range pins {
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}()
	keep := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		p, e := NewExpr(ctx, h)
		if e != nil {
			t.Fatal(e)
		}
		pins = append(pins, p)
		return h
	}
	yes := keep(m.MkTrue())
	no := keep(m.MkFalse())
	first, err := m.Symbols.Intern("first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Symbols.Intern("second")
	if err != nil {
		t.Fatal(err)
	}
	a := keep(m.MkLabel(true, []usymbol.Symbol{first}, yes))
	b := keep(m.MkLabel(true, []usymbol.Symbol{second}, yes))
	negative := keep(m.MkLabel(false, []usymbol.Symbol{second}, no))
	and := keep(m.MkAnd(a, b))
	or := keep(m.MkOr(a, b))
	shared := keep(m.MkAnd(a, a))
	not := keep(m.MkNot(negative))
	imp := keep(m.MkImplies(negative, a))
	iff := keep(m.MkIff(no, negative))
	for _, tc := range []struct {
		name  string
		h     ast.ASTHandle
		want  []usymbol.Symbol
		timed int
	}{
		{"conjunction", and, []usymbol.Symbol{first, second}, 0},
		{"first disjunct", or, []usymbol.Symbol{first}, 1},
		{"shared once", shared, []usymbol.Symbol{first}, 0},
		{"negative label", not, []usymbol.Symbol{second}, 0},
		{"implication consequent first", imp, []usymbol.Symbol{first}, 1},
		{"equivalence polarity", iff, []usymbol.Symbol{second}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfiler(nil)
			var tick int64
			profile.now = func() time.Time { tick++; return time.Unix(tick, 0) }
			ctx.activeProfiler = profile
			defer func() { ctx.activeProfiler = nil }()
			got, err := g.labelsForFormula(tc.h)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("labels %v want %v", got, tc.want)
			}
			child := profile.top.children["SubtermTruth"]
			if tc.timed == 0 && child != nil || tc.timed != 0 && (child == nil || child.elapsed != time.Duration(tc.timed)*time.Second) {
				t.Fatalf("SubtermTruth scope: got %v, want %d samples", child, tc.timed)
			}
			if profile.current != profile.top {
				t.Fatal("timer scope not restored")
			}
		})
	}
}
