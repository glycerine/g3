package core

import (
	"testing"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestInjectivityParameterDescriptionsMatchSource(t *testing.T) {
	f := newInjectivityFixture(t)
	descriptions, err := uparams.NewParamDescrs(f.m.Symbols, uparams.ParamDescrsConfig{Budget: f.m.Budget()})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, descriptions.Close()) })
	simplifyMust(t, f.runner.CollectParamDescrs(descriptions))
	simplifyMust(t, f.runner.CollectParamDescrs(descriptions))
	size, err := descriptions.Size()
	simplifyMust(t, err)
	if size != 2 {
		t.Fatalf("descriptor count = %d, want 2", size)
	}
	for i, want := range []struct {
		name, description string
		kind              uparams.ParamKind
	}{
		{"max_memory", "(default: infty) maximum amount of memory in megabytes.", uparams.ParamUInt},
		{"produce_models", "(default: false) model generation.", uparams.ParamBool},
	} {
		name, err := descriptions.ParamName(uint32(i))
		simplifyMust(t, err)
		text, err := f.m.Symbols.String(name)
		simplifyMust(t, err)
		kind, err := descriptions.Kind(name)
		simplifyMust(t, err)
		description, present, err := descriptions.Description(name)
		simplifyMust(t, err)
		if text != want.name || kind != want.kind || !present || description != want.description {
			t.Fatalf("descriptor %d = %q, %v, %q, %v", i, text, kind, description, present)
		}
		if _, present, err = descriptions.Default(name); err != nil || present {
			t.Fatalf("descriptor %q has unexpected separate default: %v, %v", text, present, err)
		}
	}
}
