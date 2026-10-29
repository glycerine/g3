package sls

// Source: src/tactic/sls/sls_params.pyg and generated sls_params.hpp.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func slsParamsFixture(t *testing.T) (*uparams.ParamsContext, *uparams.GlobalParamsService, *uparams.ParamsRef, *arena.Budget) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 24)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget, PageSize: 64, DescriptorPageSize: 2})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{PageSize: 2, Budget: budget},
		Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}},
	}})
	must(err)
	context, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{
		Budget: budget, PageEntries: 2,
		Text: arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 2, Budget: budget}},
	})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{
		Budget:       budget,
		Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}},
		Text:         arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 2, Budget: budget}},
	})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{
		Budget: budget, Catalog: catalog, DescriptorPageSize: 2, DescriptorWordsPageSize: 16,
	})
	must(err)
	must(globals.InitializeSource())
	local, err := context.NewValue()
	must(err)
	t.Cleanup(func() {
		budget.FailAfter(-1)
		must(context.Del(&local))
		must(globals.Close())
		must(catalog.Close())
		must(context.Close())
		must(numbers.Close())
		must(symbols.Close())
		if budget.Used() != 0 {
			t.Error("retained SLS parameter owners", budget.Used())
		}
	})
	return context, globals, &local, budget
}

func TestSLSParamsGeneratedDefaults(t *testing.T) {
	context, globals, _, _ := slsParamsFixture(t)
	p, err := NewParams(context, globals, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	bools := []struct {
		name string
		call func() (bool, error)
		want bool
	}{
		{"walksat", p.Walksat, true}, {"walksat_ucb", p.WalksatUCB, true},
		{"walksat_ucb_init", p.WalksatUCBInit, false}, {"walksat_repick", p.WalksatRepick, true},
		{"vns_repick", p.VNSRepick, false}, {"restart_init", p.RestartInit, false},
		{"early_prune", p.EarlyPrune, true}, {"random_offset", p.RandomOffset, true},
		{"rescore", p.Rescore, true}, {"track_unsat", p.TrackUnsat, false},
	}
	for _, tc := range bools {
		got, err := tc.call()
		if err != nil || got != tc.want {
			t.Fatalf("%s = %v, want %v: %v", tc.name, got, tc.want, err)
		}
	}
	uints := []struct {
		name string
		call func() (uint32, error)
		want uint32
	}{
		{"max_memory", p.MaxMemory, ^uint32(0)}, {"max_restarts", p.MaxRestarts, ^uint32(0)},
		{"paws_init", p.PawsInit, 40}, {"paws_sp", p.PawsSP, 52}, {"wp", p.WP, 100},
		{"vns_mc", p.VNSMC, 0}, {"restart_base", p.RestartBase, 100}, {"random_seed", p.RandomSeed, 0},
	}
	for _, tc := range uints {
		got, err := tc.call()
		if err != nil || got != tc.want {
			t.Fatalf("%s = %d, want %d: %v", tc.name, got, tc.want, err)
		}
	}
	floats := []struct {
		name string
		call func() (float64, error)
		want float64
	}{
		{"walksat_ucb_constant", p.WalksatUCBConstant, 20.0},
		{"walksat_ucb_forget", p.WalksatUCBForget, 1.0},
		{"walksat_ucb_noise", p.WalksatUCBNoise, 0.0002},
		{"scale_unsat", p.ScaleUnsat, 0.5},
	}
	for _, tc := range floats {
		got, err := tc.call()
		if err != nil || got != tc.want {
			t.Fatalf("%s = %g, want %g: %v", tc.name, got, tc.want, err)
		}
	}
}

func TestSLSParamsLocalPrecedenceSnapshotAndOwnerGuards(t *testing.T) {
	context, globals, local, _ := slsParamsFixture(t)
	moduleName, err := context.SymbolsField.Intern("sls")
	if err != nil {
		t.Fatal(err)
	}
	module, err := globals.Current.GetParams(moduleName)
	if err != nil {
		t.Fatal(err)
	}
	walksat, paws, noise := "walksat", "paws_init", "walksat_ucb_noise"
	if err = context.SetBoolName(module, &walksat, false); err != nil {
		t.Fatal(err)
	}
	if err = context.SetUIntName(module, &paws, 77); err != nil {
		t.Fatal(err)
	}
	if err = context.SetDoubleName(module, &noise, 0.25); err != nil {
		t.Fatal(err)
	}
	p, err := NewParams(context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	if err = context.SetBoolName(local, &walksat, true); err != nil {
		t.Fatal(err)
	}
	if err = context.SetUIntName(local, &paws, 88); err != nil {
		t.Fatal(err)
	}
	if err = context.SetDoubleName(local, &noise, 0.5); err != nil {
		t.Fatal(err)
	}
	if got, err := p.Walksat(); err != nil || !got {
		t.Fatal("local bool precedence", got, err)
	}
	if got, err := p.PawsInit(); err != nil || got != 88 {
		t.Fatal("local uint precedence", got, err)
	}
	if got, err := p.WalksatUCBNoise(); err != nil || got != 0.5 {
		t.Fatal("local double precedence", got, err)
	}
	if err = context.Reset(local); err != nil {
		t.Fatal(err)
	}
	if got, err := p.Walksat(); err != nil || got {
		t.Fatal("captured global bool", got, err)
	}
	if got, err := p.PawsInit(); err != nil || got != 77 {
		t.Fatal("captured global uint", got, err)
	}
	if got, err := p.WalksatUCBNoise(); err != nil || got != 0.25 {
		t.Fatal("captured global double", got, err)
	}
	if err = globals.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	if got, err := p.PawsInit(); err != nil || got != 77 {
		t.Fatal("constructor snapshot changed", got, err)
	}
	copyOwner := *p
	if _, err = copyOwner.RandomSeed(); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("shallow copy accepted", err)
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	if err = p.Close(); err != nil {
		t.Fatal("idempotent close", err)
	}
	if _, err = p.MaxMemory(); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("closed facade accepted", err)
	}
}
