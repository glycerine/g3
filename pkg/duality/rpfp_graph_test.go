package duality

import (
	"errors"
	"math"
	"testing"
)

func TestRPFPGraphOwnershipCloningAndRepeatedChildren(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("graph-relation")
	must(err)
	closeExpr(t, term)
	before := m.Stats()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	other, err := NewRPFP(ctx)
	must(err)
	defer func() { must(other.Close()) }()
	parent, err := g.CreateNode(term)
	must(err)
	child, err := g.CreateNode(term)
	must(err)
	if parent.Number != 1 || child.Number != 2 || parent.RecursionBound != math.MaxUint32 {
		t.Fatal("source node defaults")
	}
	full, err := parent.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("annotation not full")
	}
	full, err = parent.Bound.IsFull()
	must(err)
	if !full {
		t.Fatal("bound not full")
	}
	empty, err := parent.Underapprox.IsEmpty()
	must(err)
	if !empty {
		t.Fatal("underapprox not empty")
	}
	parent.RecursionBound = 7
	clone, err := other.CloneNode(parent)
	must(err)
	if clone.Map != parent || clone.Number != 1 || clone.RecursionBound != math.MaxUint32 {
		t.Fatal("source clone defaults")
	}
	must(parent.Annotation.SetEmpty())
	full, err = clone.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("clone shared mutable transformer")
	}
	children := []*Node{child, child}
	edge, err := g.CreateEdge(parent, clone.Annotation, children)
	must(err)
	children[0] = parent
	if edge.Number != 1 || edge.Children[0] != child || parent.Outgoing != edge || len(child.Incoming) != 2 {
		t.Fatal("hyper-edge linkage")
	}
	if err = g.DeleteNode(child); !errors.Is(err, ErrConnectedNode) {
		t.Fatal("deleted connected child", err)
	}
	if err = g.DeleteNode(parent); !errors.Is(err, ErrConnectedNode) {
		t.Fatal("deleted connected parent", err)
	}
	must(clone.Annotation.SetEmpty())
	full, err = edge.F.IsFull()
	must(err)
	if !full {
		t.Fatal("edge did not copy transformer")
	}
	must(g.DeleteEdge(edge))
	if parent.Outgoing != nil || len(child.Incoming) != 0 {
		t.Fatal("duplicate incoming edge not removed")
	}
	lower, err := g.CreateLowerBoundEdge(parent)
	must(err)
	empty, err = lower.F.IsEmpty()
	must(err)
	if !empty || lower.Number != 2 || len(lower.Children) != 0 {
		t.Fatal("lower bound edge")
	}
	must(g.Close())
	must(g.Close())
	must(other.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("graph leaked AST ownership", before, after)
	}
	if _, err = g.CreateNode(term); err == nil {
		t.Fatal("closed graph accepted node")
	}
}

func TestRPFPGraphParameterizedNodeAndForeignOwners(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	x, err := ctx.BoolConst("graph-param")
	must(err)
	closeExpr(t, x)
	sort, err := x.Sort()
	must(err)
	defer sort.Close()
	name, err := ctx.StringSymbol("graph-predicate")
	must(err)
	decl, err := ctx.Function(name, []*Sort{sort}, sort)
	must(err)
	closeDecl(t, decl)
	term, err := decl.Call(x)
	must(err)
	closeExpr(t, term)
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	other, err := NewRPFP(ctx)
	must(err)
	defer func() { must(other.Close()) }()
	node, err := g.CreateNode(term)
	must(err)
	if len(node.Annotation.individualParams) != 1 {
		t.Fatal("missing parameter")
	}
	got, err := m.RefGet(node.Annotation.individualParams[0])
	must(err)
	want, err := x.Raw()
	must(err)
	if got != want {
		t.Fatal("wrong parameter")
	}
	foreign, err := other.CreateNode(term)
	must(err)
	if _, err = g.CreateEdge(node, node.Annotation, []*Node{foreign}); err == nil {
		t.Fatal("foreign child accepted")
	}
	if err = g.DeleteNode(foreign); err == nil {
		t.Fatal("foreign node deleted")
	}
	if len(g.edges) != 0 || len(foreign.Incoming) != 0 || node.Outgoing != nil {
		t.Fatal("failed edge changed graph")
	}
}
