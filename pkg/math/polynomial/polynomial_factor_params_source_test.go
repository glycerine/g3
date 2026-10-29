package polynomial

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestPolynomialFactorParameterDescriptorSchemaSourceOrder(t *testing.T) {
	cfg, _, budget := algebraicManagerFixture(t)
	d, e := uparams.NewParamDescrs(cfg.Globals.Symbols, uparams.ParamDescrsConfig{
		Budget: budget,
		Text: arena.VectorConfig{
			PageSize:    64,
			Descriptors: arena.Config{Budget: budget, PageSize: 2},
		},
	})
	serviceMust(t, e)
	defer func() { serviceMust(t, d.Close()) }()

	serviceMust(t, CollectPolynomialFactorParamDescriptions(d))
	serviceMust(t, CollectPolynomialFactorParamDescriptions(d))

	rows := []struct{ key, defaultValue, description string }{
		{"max_search_size", "4294967295", "(default: infty) Z3 polynomial factorization is composed of three steps: factorization in GF(p), lifting and search. This parameter can be used to limit the search space."},
		{"max_prime", "4294967295", "(default: infty) Z3 polynomial factorization is composed of three steps: factorization in GF(p), lifting and search. This parameter limits the maximum prime number p to be used in the first step."},
		{"num_primes", "1", "(default: 1) Z3 polynomial factorization is composed of three steps: factorization in GF(p), lifting and search. This parameter specify the maximum number of finite factorizations to be considered, before lifiting and searching."},
	}
	n, e := d.Size()
	serviceMust(t, e)
	if n != uint32(len(rows)) {
		t.Fatal("source schema size after repeated collection", n)
	}
	for i, row := range rows {
		name, e := d.ParamName(uint32(i))
		serviceMust(t, e)
		expected, e := cfg.Globals.Symbols.Intern(row.key)
		serviceMust(t, e)
		kind, e := d.KindName(row.key)
		serviceMust(t, e)
		value, present, e := d.DefaultName(row.key)
		serviceMust(t, e)
		module, hasModule, e := d.ModuleName(row.key)
		serviceMust(t, e)
		description, hasDescription, e := d.DescriptionName(row.key)
		serviceMust(t, e)
		if name != expected || kind != uparams.ParamUInt || !present || value != row.defaultValue || !hasModule || module != "factor" || !hasDescription || description != row.description {
			t.Fatal("source descriptor binding", i, row, value, module, description)
		}
	}
}
