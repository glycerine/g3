package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type rcfParamsFixture struct {
	t       *testing.T
	budget  *arena.Budget
	symbols *symbol.Manager
	numbers *numeral.RationalService
	context *params.ParamsContext
	catalog *params.GlobalParamsCatalog
	globals *params.GlobalParamsService
	local   params.ParamsRef
}

func newRCFParamsFixture(t *testing.T) *rcfParamsFixture {
	t.Helper()
	f := &rcfParamsFixture{t: t, budget: arena.NewBudget(1 << 27)}
	var err error
	f.symbols, err = symbol.NewManager(symbol.Config{Budget: f.budget})
	if err != nil {
		t.Fatal(err)
	}
	f.numbers, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: f.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: f.budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.context, err = params.NewParamsContext(f.symbols, f.numbers, params.ParamsConfig{Budget: f.budget})
	if err != nil {
		t.Fatal(err)
	}
	f.catalog, err = params.NewGlobalParamsCatalog(f.symbols, params.GlobalParamsCatalogConfig{
		Budget: f.budget, Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{Budget: f.budget}},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.globals, err = params.NewGlobalParamsService(f.symbols, f.context, params.GlobalParamsConfig{Budget: f.budget, Catalog: f.catalog})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.globals.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	f.local, err = f.context.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.close)
	return f
}

func (f *rcfParamsFixture) close() {
	f.t.Helper()
	for _, err := range []error{
		f.context.Del(&f.local), f.globals.Close(), f.catalog.Close(),
		f.context.Close(), f.numbers.Close(), f.symbols.Close(),
	} {
		if err != nil {
			f.t.Error(err)
		}
	}
	if used := f.budget.Used(); used != 0 {
		f.t.Error("rcf params fixture retained budget", used)
	}
}

func (f *rcfParamsFixture) setModuleBool(name string, value bool) {
	f.t.Helper()
	moduleName, err := f.symbols.Intern("rcf")
	if err != nil {
		f.t.Fatal(err)
	}
	module, err := f.globals.Current.GetParams(moduleName)
	if err != nil {
		f.t.Fatal(err)
	}
	if err = f.context.SetBoolName(module, &name, value); err != nil {
		f.t.Fatal(err)
	}
}

func (f *rcfParamsFixture) setModuleUInt(name string, value uint32) {
	f.t.Helper()
	moduleName, err := f.symbols.Intern("rcf")
	if err != nil {
		f.t.Fatal(err)
	}
	module, err := f.globals.Current.GetParams(moduleName)
	if err != nil {
		f.t.Fatal(err)
	}
	if err = f.context.SetUIntName(module, &name, value); err != nil {
		f.t.Fatal(err)
	}
}

func TestRCFParamsDefaultsModuleFallbackAndLocalPrecedence(t *testing.T) {
	f := newRCFParamsFixture(t)
	defaults, err := NewRCFParams(f.context, f.globals, nil)
	if err != nil {
		t.Fatal(err)
	}
	usePrem, err := defaults.UsePrem()
	if err != nil {
		t.Fatal(err)
	}
	clean, err := defaults.CleanDenominators()
	if err != nil {
		t.Fatal(err)
	}
	initial, err := defaults.InitialPrecision()
	if err != nil {
		t.Fatal(err)
	}
	inf, err := defaults.InfPrecision()
	if err != nil {
		t.Fatal(err)
	}
	max, err := defaults.MaxPrecision()
	if err != nil {
		t.Fatal(err)
	}
	lazy, err := defaults.LazyAlgebraicNormalization()
	if err != nil {
		t.Fatal(err)
	}
	if !usePrem || !clean || initial != 24 || inf != 24 || max != 128 || !lazy {
		t.Fatal("generated literal defaults", usePrem, clean, initial, inf, max, lazy)
	}
	if err := defaults.Close(); err != nil {
		t.Fatal(err)
	}

	f.setModuleBool("use_prem", false)
	f.setModuleBool("clean_denominators", false)
	f.setModuleUInt("initial_precision", 7)
	f.setModuleUInt("inf_precision", 8)
	f.setModuleUInt("max_precision", 9)
	f.setModuleBool("lazy_algebraic_normalization", false)
	p, err := NewRCFParams(f.context, f.globals, &f.local)
	if err != nil {
		t.Fatal(err)
	}
	usePrem, _ = p.UsePrem()
	clean, _ = p.CleanDenominators()
	initial, _ = p.InitialPrecision()
	inf, _ = p.InfPrecision()
	max, _ = p.MaxPrecision()
	lazy, _ = p.LazyAlgebraicNormalization()
	if usePrem || clean || initial != 7 || inf != 8 || max != 9 || lazy {
		t.Fatal("module fallback", usePrem, clean, initial, inf, max, lazy)
	}

	for _, item := range []struct {
		name  string
		value uint32
	}{{"initial_precision", 17}, {"inf_precision", 18}, {"max_precision", 19}} {
		if err := f.context.SetUIntName(&f.local, &item.name, item.value); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"use_prem", "clean_denominators", "lazy_algebraic_normalization"} {
		if err := f.context.SetBoolName(&f.local, &name, true); err != nil {
			t.Fatal(err)
		}
	}
	usePrem, _ = p.UsePrem()
	clean, _ = p.CleanDenominators()
	initial, _ = p.InitialPrecision()
	inf, _ = p.InfPrecision()
	max, _ = p.MaxPrecision()
	lazy, _ = p.LazyAlgebraicNormalization()
	if !usePrem || !clean || initial != 17 || inf != 18 || max != 19 || !lazy {
		t.Fatal("local precedence and borrowed mutation", usePrem, clean, initial, inf, max, lazy)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.UsePrem(); !errors.Is(err, params.ErrParamsOwner) {
		t.Fatal("closed generated facade remained readable", err)
	}
	if err := p.Close(); err != nil {
		t.Fatal("generated facade close is not idempotent", err)
	}
}
