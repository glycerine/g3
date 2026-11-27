package params

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func BenchmarkParamDescriptionLookup(b *testing.B) {
	for _, count := range []int{32, 512, 4096} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			_, symbols, _ := paramsTestOwner(b)
			budget := arena.NewBudget(^uint64(0))
			d, err := NewParamDescrs(symbols, ParamDescrsConfig{Budget: budget})
			paramsMust(b, err)
			b.Cleanup(func() { paramsMust(b, d.Close()) })
			keys := make([]usymbol.Symbol, count)
			description := "Boolean option"
			for i := range keys {
				keys[i], err = symbols.Intern("option" + strconv.Itoa(i))
				paramsMust(b, err)
				paramsMust(b, d.Insert(keys[i], ParamBool, &description, nil, nil))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				kind, err := d.Kind(keys[i&(count-1)])
				if err != nil || kind != ParamBool {
					b.Fatal(kind, err)
				}
			}
		})
	}
}

// Each registry sees its first request for the global default. Its symbol
// manager and catalog are shared with the next registry, as on a warm restart.
func BenchmarkFirstGlobalParameterLookup(b *testing.B) {
	ctx, symbols, _ := paramsTestOwner(b)
	budget := arena.NewBudget(^uint64(0))
	catalog, err := NewGlobalParamsCatalog(symbols, catalogTestConfig(budget))
	paramsMust(b, err)
	defer func() { paramsMust(b, catalog.Close()) }()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g, err := NewGlobalParams(symbols, ctx, GlobalParamsConfig{Budget: budget, Catalog: catalog})
		paramsMust(b, err)
		value, err := g.GetValueName("auto_config")
		if err != nil || value != "true" {
			b.Fatal(value, err)
		}
		paramsMust(b, g.Close())
	}
}
