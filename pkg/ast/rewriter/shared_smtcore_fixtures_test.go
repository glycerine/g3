package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func dynAckSourceFixture(t testing.TB) (*uparams.ParamsContext, *uparams.GlobalParamsService, *uparams.ParamsRef, *arena.Budget) {
	t.Helper()
	c, sy, _ := paramsTestOwner(t)
	b := arena.NewBudget(^uint64(0))
	catalog, e := uparams.NewGlobalParamsCatalog(sy, catalogTestConfig(b))
	paramsMust(t, e)
	cfg := gparamsTestConfig(b)
	cfg.Catalog = catalog
	g, e := uparams.NewGlobalParamsService(sy, c, cfg)
	paramsMust(t, e)
	paramsMust(t, g.InitializeSource())
	local, e := c.NewValue()
	paramsMust(t, e)
	t.Cleanup(func() {
		paramsMust(t, c.Del(&local))
		paramsMust(t, g.Close())
		paramsMust(t, catalog.Close())
		if b.Used() != 0 {
			t.Fatal("actual service/header roots", b.Used())
		}
	})
	return c, g, &local, b
}

var _ umemory.ConstructionFailure = (*unumeral.RationalService)(nil)

func catalogTestConfig(b *arena.Budget) uparams.GlobalParamsCatalogConfig {
	return uparams.GlobalParamsCatalogConfig{Budget: b, Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}}, Text: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}}}
}
func paramsMust(t testing.TB, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

func gparamsTestConfig(b *arena.Budget) uparams.GlobalParamsConfig {
	return uparams.GlobalParamsConfig{Budget: b, DescriptorPageSize: 2, DescriptorWordsPageSize: 16}
}
