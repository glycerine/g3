package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"testing"
)

func TestProjectionIndexSignedBoundsAndRepeatedVariables(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
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
	family, err := m.ArithmeticFamilyID()
	must(err)
	p := &projectionArithmetic{graph: graph, simplify: simplifier, family: family}
	defer p.close()
	zero, err := p.numeral("0")
	must(err)
	one, err := p.numeral("1")
	must(err)
	sort, err := m.GetSort(zero)
	must(err)
	name, err := m.Symbols.Intern("indexed-x")
	must(err)
	variable, err := p.pin(m.MkConst(name, sort))
	must(err)
	lower, err := p.app(3, variable, zero)
	must(err)
	upper, err := p.app(2, variable, one)
	must(err)
	index := newProjectionIndex(p)
	must(index.predicate(true, lower, 0))
	must(index.predicate(true, upper, 1))
	if index.indices[1][variable] != 0 || index.indices[0][variable] != 1 || len(index.positive) != 1 {
		t.Fatal("signed occurrences", index.indices, index.positive)
	}
	must(index.predicate(true, lower, 2))
	if index.indices[0][variable] != -1 || index.indices[1][variable] != -1 {
		t.Fatal("repeated occurrence remains eligible", index.indices)
	}
}
