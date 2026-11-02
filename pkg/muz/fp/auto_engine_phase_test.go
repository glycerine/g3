package fp_test

// Source: dl_context.cpp:840-859: normalized rules precede pending formulas.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type autoPhaseFixture struct {
	engine                                              *fp.PropositionalEngine
	integerFact, openIntegerFact, quantified, universal ast.ASTHandle
}

func newAutoPhaseFixture(t *testing.T) autoPhaseFixture {
	t.Helper()
	e, m := newRoutingEngine(t)
	var roots []ast.ASTRef
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		fpMust(t, err)
		r, err := m.NewRef(h)
		fpMust(t, err)
		roots = append(roots, r)
		return h
	}
	t.Cleanup(func() {
		for i := len(roots) - 1; i >= 0; i-- {
			fpMust(t, m.DelRef(&roots[i]))
		}
	})
	zero := pin(m.ArithmeticIntegerNumeralText("0"))
	integer := pin(m.GetSort(zero))
	boolean := pin(m.MkBoolSort())
	name, err := m.Symbols.Intern("auto-phase-integer")
	fpMust(t, err)
	d := pin(m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
	fpMust(t, e.RegisterPredicate(d))
	fact := pin(m.MkApp(d, []ast.ASTHandle{zero}))
	v := pin(m.MkVar(0, integer))
	open := pin(m.MkApp(d, []ast.ASTHandle{v}))
	_, head := routingPredicate(t, e, m, "auto-phase-quantified")
	quantified := pin(autoQuantifiedRule(t, m, head), nil)
	positive := pin(m.MkArithmeticGT(v, zero))
	universal := pin(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: positive}))
	universalRule := pin(m.MkImplies(universal, head))
	return autoPhaseFixture{e, fact, open, quantified, universalRule}
}

func checkAutoPhaseKind(t *testing.T, e *fp.PropositionalEngine, want fp.EngineKind) {
	t.Helper()
	fpMust(t, e.SelectEngine("auto-config"))
	kind, err := e.SelectedEngine()
	fpMust(t, err)
	if kind != want {
		t.Fatalf("source phase selection = %v, want %v", kind, want)
	}
}

func TestFixedpointAutoConfigNormalizedPrefixPrecedesPendingInputs(t *testing.T) {
	for _, quantifiedPending := range []bool{true, false} {
		name := "pending-integer"
		if quantifiedPending {
			name = "pending-quantified"
		}
		t.Run(name, func(t *testing.T) {
			f := newAutoPhaseFixture(t)
			pending, normalized := f.integerFact, f.universal
			want := fp.EngineQPDR
			if quantifiedPending {
				pending, normalized = f.quantified, f.integerFact
				want = fp.EnginePDR
			}
			fpMust(t, f.engine.AddRule(pending, "pending"))
			fpMust(t, f.engine.UpdateRule(normalized, "normalized"))
			checkAutoPhaseKind(t, f.engine, want)
		})
	}
}

func TestFixedpointAutoConfigTracksUpdatePromotionAndPop(t *testing.T) {
	for _, operation := range []string{"update", "display"} {
		t.Run(operation, func(t *testing.T) {
			f := newAutoPhaseFixture(t)
			fpMust(t, f.engine.AddRule(f.quantified, "pending-quantified"))
			if operation == "display" {
				fpMust(t, f.engine.AddRule(f.openIntegerFact, "pending-open"))
			}
			checkAutoPhaseKind(t, f.engine, fp.EngineQPDR)
			fpMust(t, f.engine.Push())
			if operation == "update" {
				fpMust(t, f.engine.UpdateRule(f.integerFact, "normalized"))
			} else {
				var out bytes.Buffer
				fpMust(t, f.engine.WriteContextSMT2(&out, nil, nil))
			}
			checkAutoPhaseKind(t, f.engine, fp.EnginePDR)
			fpMust(t, f.engine.Pop())
			checkAutoPhaseKind(t, f.engine, fp.EngineQPDR)
		})
	}
}
