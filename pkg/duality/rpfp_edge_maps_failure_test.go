package duality

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestRPFPLocalizeRebuildsInterruptedEdgeMaps(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	const width = 16
	arguments := make([]*Expr, width)
	for i := range arguments {
		var err error
		arguments[i], err = ctx.BoolConst(fmt.Sprintf("map-retry-%d", i))
		must(err)
		closeExpr(t, arguments[i])
	}
	sort, err := arguments[0].Sort()
	must(err)
	defer func() { must(sort.Close()) }()
	sorts := make([]*Sort, width)
	for i := range sorts {
		sorts[i] = sort
	}
	name, err := ctx.StringSymbol("map-retry-relation")
	must(err)
	decl, err := ctx.Function(name, sorts, sort)
	must(err)
	closeDecl(t, decl)
	term, err := decl.Call(arguments...)
	must(err)
	closeExpr(t, term)
	formula, err := NewTransformer(ctx, nil, arguments, arguments[width-1])
	must(err)
	defer func() { must(formula.Close()) }()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	node, err := g.CreateNode(term)
	must(err)
	edge, err := g.CreateEdge(node, formula, nil)
	must(err)
	// Keep the indexed vocabulary alive so the refused allocation belongs to
	// map ownership, rather than first-time symbol or expression construction.
	activation, indexed, err := g.RedVars(node)
	must(err)
	defer func() {
		must(activation.Close())
		for _, variable := range indexed {
			must(variable.Close())
		}
	}()
	h, err := term.Raw()
	must(err)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			must(m.DelRef(&pins[i]))
		}
	}()
	for i := 0; ; i++ {
		if i == 4096 {
			t.Fatal("reference allocation frontier not reached")
		}
		m.Budget().FailAfter(0)
		pin, err := m.NewRef(h)
		m.Budget().FailAfter(-1)
		if errors.Is(err, arena.ErrInjectedFailure) {
			if len(pins) >= width+8 {
				break
			}
			// The fixture may start near a page boundary. Allocate the next
			// page before selecting a frontier with enough releasable slots.
			pin, err = m.NewRef(h)
		}
		must(err)
		pins = append(pins, pin)
	}
	// Leave enough slots for RedVars and a nonempty map prefix, but not the
	// entire second owning copy of the indexed parameter vector.
	for i := 0; i < width+8; i++ {
		last := len(pins) - 1
		if last < 0 {
			t.Fatal("insufficient slots before reference frontier")
		}
		must(m.DelRef(&pins[last]))
		pins = pins[:last]
	}
	before := m.Stats()
	m.Budget().FailAfter(0)
	err = g.SetEdgeMaps(edge)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || len(edge.varMap) == 0 || len(edge.varMap) >= width {
		t.Fatal("fixture missed partial edge-map allocation", err, len(edge.varMap))
	}
	for i, argument := range arguments {
		localized, err := g.Localize(edge, argument)
		must(err)
		got, err := localized.Raw()
		must(err)
		want, err := indexed[i].Raw()
		must(err)
		must(localized.Close())
		if got != want {
			t.Fatal("retry used incomplete parameter map", i, got, want)
		}
	}
	if len(edge.varMap) != width {
		t.Fatal("retry did not rebuild all parameter mappings", len(edge.varMap))
	}
	must(edge.clearMaps())
	if after := m.Stats(); after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes {
		t.Fatal("map recovery left temporary owners", before, after)
	}
}
