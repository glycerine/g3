package macros_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/params"
)

// The incremental solver shares its parameter services across all expansions.
func BenchmarkMacroExpansionWithSharedParams(b *testing.B) {
	m, manager, cases := macroExpansionCases(b)
	ctx, err := params.NewParamsContext(m.Symbols, m.NumeralService(), params.ParamsConfig{
		Budget: m.Budget(), Text: arena.VectorConfig{Descriptors: arena.Config{Budget: m.Budget()}},
	})
	must(b, err)
	b.Cleanup(func() { must(b, ctx.Close()) })
	catalog, err := params.NewGlobalParamsCatalog(m.Symbols, params.GlobalParamsCatalogConfig{Budget: m.Budget()})
	must(b, err)
	b.Cleanup(func() { must(b, catalog.Close()) })
	globals, err := params.NewGlobalParamsService(m.Symbols, ctx, params.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	must(b, err)
	b.Cleanup(func() { must(b, globals.Close()) })
	must(b, globals.InitializeSource())
	must(b, manager.SetRewriterParams(ctx, globals))
	b.Cleanup(func() { must(b, manager.SetRewriterParams(nil, nil)) })
	benchmarkMacroExpansionCases(b, m, manager, cases)
}
