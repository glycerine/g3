package opt

// Source: src/opt/opt_params.hpp; src/opt/opt_params.pyg.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func optParamsFixture(t *testing.T) (*uparams.ParamsContext, *uparams.GlobalParamsService, *uparams.ParamsRef, *arena.Budget) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	b := arena.NewBudget(1 << 24)
	sy, err := symbol.NewManager(symbol.Config{Budget: b, PageSize: 64, DescriptorPageSize: 2})
	must(err)
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{PageSize: 2, Budget: b},
		Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}},
	}})
	must(err)
	c, err := uparams.NewParamsContext(sy, q, uparams.ParamsConfig{
		Budget: b, PageEntries: 2,
		Text: arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 2, Budget: b}},
	})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(sy, uparams.GlobalParamsCatalogConfig{
		Budget:       b,
		Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}},
		Text:         arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 2, Budget: b}},
	})
	must(err)
	g, err := uparams.NewGlobalParamsService(sy, c, uparams.GlobalParamsConfig{
		Budget: b, Catalog: catalog, DescriptorPageSize: 2, DescriptorWordsPageSize: 16,
	})
	must(err)
	must(g.InitializeSource())
	local, err := c.NewValue()
	must(err)
	t.Cleanup(func() {
		b.FailAfter(-1)
		must(c.Del(&local))
		must(g.Close())
		must(catalog.Close())
		must(c.Close())
		must(q.Close())
		must(sy.Close())
		if b.Used() != 0 {
			t.Error("retained parameter owners", b.Used())
		}
	})
	return c, g, &local, b
}

func TestOptParamsGeneratedDefaults(t *testing.T) {
	c, g, _, _ := optParamsFixture(t)
	p, err := NewParams(c, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	for name, getter := range map[string]struct {
		call func() (bool, error)
		want bool
	}{
		"dump_benchmarks":                {p.DumpBenchmarks, false},
		"enable_sls":                     {p.EnableSLS, false},
		"enable_sat":                     {p.EnableSAT, true},
		"elim_01":                        {p.Elim01, true},
		"pp.neat":                        {p.PPNeat, true},
		"pb.compile_equality":            {p.PBCompileEquality, false},
		"maxres.hill_climb":              {p.MaxResHillClimb, true},
		"maxres.add_upper_bound_block":   {p.MaxResAddUpperBoundBlock, false},
		"maxres.maximize_assignment":     {p.MaxResMaximizeAssignment, false},
		"maxres.wmax":                    {p.MaxResWMax, false},
		"maxres.pivot_on_correction_set": {p.MaxResPivotOnCorrectionSet, true},
	} {
		got, err := getter.call()
		if err != nil || got != getter.want {
			t.Fatalf("%s: got %v, want %v: %v", name, got, getter.want, err)
		}
	}
	for name, getter := range map[string]struct {
		call func() (uint32, error)
		want uint32
	}{
		"timeout":                        {p.Timeout, ^uint32(0)},
		"rlimit":                         {p.RLimit, 0},
		"maxres.max_num_cores":           {p.MaxResMaxNumCores, ^uint32(0)},
		"maxres.max_core_size":           {p.MaxResMaxCoreSize, 3},
		"maxres.max_correction_set_size": {p.MaxResMaxCorrectionSetSize, 3},
	} {
		got, err := getter.call()
		if err != nil || got != getter.want {
			t.Fatalf("%s: got %d, want %d: %v", name, got, getter.want, err)
		}
	}
	for name, tc := range map[string]struct {
		call func() (symbol.Symbol, error)
		want string
	}{
		"optsmt_engine":   {p.OptSMTEngine, "basic"},
		"maxsat_engine":   {p.MaxSATEngine, "maxres"},
		"priority":        {p.Priority, "lex"},
		"solution_prefix": {p.SolutionPrefix, ""},
	} {
		got, err := tc.call()
		if err != nil {
			t.Fatal(name, err)
		}
		text, _, err := c.SymbolsField.BareString(got)
		if err != nil || text != tc.want {
			t.Fatalf("%s: got %q, want %q: %v", name, text, tc.want, err)
		}
	}
}

func TestOptParamsLocalPrecedenceGlobalSnapshotAndOwnerGuards(t *testing.T) {
	c, g, local, _ := optParamsFixture(t)
	moduleName, err := c.SymbolsField.Intern("opt")
	if err != nil {
		t.Fatal(err)
	}
	module, err := g.Current.GetParams(moduleName)
	if err != nil {
		t.Fatal(err)
	}
	priority := "priority"
	pareto, err := c.SymbolsField.Intern("pareto")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SetSymbolName(module, &priority, pareto); err != nil {
		t.Fatal(err)
	}
	size := "maxres.max_core_size"
	if err = c.SetUIntName(module, &size, 9); err != nil {
		t.Fatal(err)
	}
	p, err := NewParams(c, g, local)
	if err != nil {
		t.Fatal(err)
	}
	box, err := c.SymbolsField.Intern("box")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SetSymbolName(local, &priority, box); err != nil {
		t.Fatal(err)
	}
	if err = c.SetUIntName(local, &size, 4); err != nil {
		t.Fatal(err)
	}
	gotPriority, err := p.Priority()
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := c.SymbolsField.BareString(gotPriority)
	if err != nil || text != "box" {
		t.Fatal("local symbol precedence", text, err)
	}
	gotSize, err := p.MaxResMaxCoreSize()
	if err != nil || gotSize != 4 {
		t.Fatal("local uint precedence", gotSize, err)
	}
	if err = c.Reset(local); err != nil {
		t.Fatal(err)
	}
	gotPriority, err = p.Priority()
	if err != nil {
		t.Fatal(err)
	}
	text, _, err = c.SymbolsField.BareString(gotPriority)
	if err != nil || text != "pareto" {
		t.Fatal("captured global module", text, err)
	}
	gotSize, err = p.MaxResMaxCoreSize()
	if err != nil || gotSize != 9 {
		t.Fatal("captured global uint", gotSize, err)
	}
	// Reinitialization replaces the process registry, but not this source
	// constructor's owned module snapshot.
	if err = g.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	gotSize, err = p.MaxResMaxCoreSize()
	if err != nil || gotSize != 9 {
		t.Fatal("module snapshot changed", gotSize, err)
	}
	copyOwner := *p
	if _, err = copyOwner.Timeout(); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("shallow copy accepted", err)
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	if err = p.Close(); err != nil {
		t.Fatal("idempotent close", err)
	}
	if _, err = p.EnableSAT(); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("closed facade accepted", err)
	}
}
