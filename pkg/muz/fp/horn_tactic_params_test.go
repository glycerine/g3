package fp_test

import (
	"testing"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestHornTacticCopiesParametersAndDescriptors(t *testing.T) {
	f := newHornLifecycleFixture(t, false)
	globals, err := f.command.GlobalParameters()
	fpMust(t, err)
	value, err := globals.Context.NewValue()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, globals.Context.Del(&value)) })
	key := "engine"
	spacer, err := f.manager.Symbols.Intern("spacer")
	fpMust(t, err)
	fpMust(t, globals.Context.SetSymbolName(&value, &key, spacer))
	fpMust(t, f.tactic.UpdateParams(&value))
	descriptions, err := uparams.NewParamDescrs(f.manager.Symbols, uparams.ParamDescrsConfig{Budget: f.manager.Budget()})
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, descriptions.Close()) })
	fpMust(t, f.tactic.CollectParamDescrs(descriptions))
	if count, e := descriptions.Size(); e != nil || count == 0 {
		t.Fatal("fixedpoint descriptors", count, e)
	}
	fpMust(t, f.tactic.UpdateParams(nil))
}
