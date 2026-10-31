package shell

// Source: src/test/cnf_backbones.cpp track_clauses and consequence query.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestCNFBackbonesSourceTrackedConsequences(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	sy, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	pc, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := params.NewGlobalParamsCatalog(sy, params.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := params.NewGlobalParamsService(sy, pc, params.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	input := "p cnf 4 4\n1 0\n-1 2 0\n-2 3 0\n4 -4 0\n"
	frontend, err := NewDIMACSFrontend(DIMACSFrontendConfig{Globals: globals, Input: strings.NewReader(input), Writer: &bytes.Buffer{}})
	must(err)
	defer func() {
		for _, err := range []error{frontend.Close(), globals.Close(), catalog.Close(), pc.Close(), q.Close(), sy.Close()} {
			must(err)
		}
		if budget.Used() != 0 {
			t.Fatal("tracked source fixture leaked budget", budget.Used())
		}
	}()
	must(sat.ParseDIMACS(strings.NewReader(input), frontend.solver))
	tracked, mapping, assumptions, err := frontend.trackedSolver()
	must(err)
	defer func() { must(tracked.Close()) }()
	if len(assumptions) == 0 || len(mapping) == 0 {
		t.Fatal("source clause markers were not created")
	}
	count, err := tracked.SourceNumVars()
	must(err)
	var variables []sat.BoolVar
	for v := uint32(1); v < count; v++ {
		variables = append(variables, sat.BoolVar(v))
	}
	status, rows, err := tracked.GetConsequences(assumptions, variables)
	must(err)
	if status != lbool.True {
		t.Fatal("tracked consequences", status)
	}
	seen := make(map[sat.BoolVar]bool)
	for _, row := range rows {
		if len(row) == 0 {
			t.Fatal("empty consequence row")
		}
		if !row[0].Sign() {
			seen[row[0].Variable()] = true
		}
	}
	for _, v := range []sat.BoolVar{1, 2, 3} {
		if !seen[v] {
			t.Fatal("missing tracked backbone", v, rows)
		}
	}
	if seen[4] {
		t.Fatal("free variable treated as backbone", rows)
	}
}
