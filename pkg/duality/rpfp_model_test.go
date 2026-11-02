package duality

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func annotationModelFixture(t *testing.T) (*RPFP, *Node, *Node, ast.ASTHandle) {
	t.Helper()
	ctx, _, m := dualityContextFixture(t)
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
	x, err := ctx.BoolConst("model-x")
	must(err)
	closeExpr(t, x)
	y, err := ctx.BoolConst("model-y")
	must(err)
	closeExpr(t, y)
	xh, err := x.Raw()
	must(err)
	yh, err := y.Raw()
	must(err)
	notY := wrap(m.MkNot(yh))
	nyh, err := notY.Raw()
	must(err)
	formula := wrap(m.MkAnd(xh, nyh))
	sort, err := x.Sort()
	must(err)
	closeSort(t, sort)
	name, err := ctx.StringSymbol("annotation-P")
	must(err)
	decl, err := ctx.Function(name, []*Sort{sort, sort}, sort)
	must(err)
	closeDecl(t, decl)
	app, err := decl.Call(x, y)
	must(err)
	closeExpr(t, app)
	g, err := NewRPFP(ctx)
	must(err)
	t.Cleanup(func() { must(g.Close()) })
	node, err := g.CreateNode(app)
	must(err)
	must(g.SetAnnotation(node, formula))
	constant, err := ctx.BoolConst("annotation-Q")
	must(err)
	closeExpr(t, constant)
	zero, err := g.CreateNode(constant)
	must(err)
	must(zero.Annotation.SetEmpty())
	d, err := decl.Raw()
	must(err)
	return g, node, zero, d
}

func TestRPFPAnnotationModelParameterOrderAndIndependentLifetime(t *testing.T) {
	g, node, zero, decl := annotationModelFixture(t)
	m := g.context.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	md, err := g.AnnotationModel()
	must(err)
	defer func() { must(md.Close()) }()
	fi, err := md.Core().FunctionInterpretation(decl)
	must(err)
	if fi == nil || fi.Arity() != 2 || fi.NumEntries() != 0 {
		t.Fatal("annotation must be the complete binary predicate fallback")
	}
	body, err := fi.Else()
	must(err)
	// P(x,y) = x && !y must become var(0) && !var(1).
	first, err := m.GetArg(body, 0)
	must(err)
	negated, err := m.GetArg(body, 1)
	must(err)
	second, err := m.GetArg(negated, 0)
	must(err)
	i, err := m.GetVarIndex(first)
	must(err)
	j, err := m.GetVarIndex(second)
	must(err)
	if i != 0 || j != 1 {
		t.Fatal("predicate arguments reversed", i, j)
	}
	qd, err := m.RefGet(zero.Name)
	must(err)
	oldConstant, err := md.Core().ConstantInterpretation(qd)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	falsity, err := m.MkFalse()
	must(err)
	if oldConstant != falsity {
		t.Fatal("zero-arity annotation lost")
	}
	must(node.Annotation.SetFull())
	must(zero.Annotation.SetFull())
	next, err := g.AnnotationModel()
	must(err)
	defer func() { must(next.Close()) }()
	nextFI, err := next.Core().FunctionInterpretation(decl)
	must(err)
	nextBody, err := nextFI.Else()
	must(err)
	if nextBody != truth {
		t.Fatal("new snapshot did not observe changed annotation")
	}
	must(g.Close())
	still, err := fi.Else()
	must(err)
	if still != body {
		t.Fatal("earlier model changed with graph annotations")
	}
	if _, err := g.AnnotationModel(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("closed graph accepted model request", err)
	}
}

func TestRPFPAnnotationModelEveryAllocationPrefix(t *testing.T) {
	completed := false
	for fault := int64(0); fault < 128 && !completed; fault++ {
		if !t.Run(strconv.FormatInt(fault, 10), func(t *testing.T) {
			g, _, _, _ := annotationModelFixture(t)
			m := g.context.manager
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(fault)
			md, err := g.AnnotationModel()
			m.Budget().FailAfter(-1)
			if err == nil {
				completed = true
				if err := md.Close(); err != nil {
					t.Fatal(err)
				}
			} else if md != nil || !errors.Is(err, arena.ErrInjectedFailure) && !errors.Is(err, arena.ErrBudget) {
				t.Fatal("unexpected model construction refusal", md, err)
			}
			if after := m.Stats().LiveReferences; after != before {
				t.Fatal("model construction leaked references", before, after)
			}
		}) {
			return
		}
	}
	if !completed {
		t.Fatal("model construction never reached success")
	}
}
