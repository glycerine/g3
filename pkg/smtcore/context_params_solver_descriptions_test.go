package smtcore

import (
	"testing"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestCollectSolverParamDescriptionsExactContextEntries(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	descriptions, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	astTestMust(t, CollectSolverParamDescriptions(descriptions))
	for _, name := range []string{"proof", "model", "unsat_core"} {
		if kind, e := descriptions.KindName(name); e != nil || kind != uparams.ParamBool {
			t.Fatal(name, kind, e)
		}
		if _, found, e := descriptions.DescriptionName(name); e != nil || !found {
			t.Fatal("description", name, found, e)
		}
	}
	if size, e := descriptions.Size(); e != nil || size != 3 {
		t.Fatal("size", size, e)
	}
	astTestMust(t, descriptions.Close())
	_ = config
}
