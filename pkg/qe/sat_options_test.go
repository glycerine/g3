package qe

import (
	"testing"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestSATTacticOptionsDefaultsAndUpdates(t *testing.T) {
	f := newFMFixture(t)
	c := f.r.config.Context
	p, err := c.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, c.Del(&p)) }()
	empty, err := c.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, c.Del(&empty)) }()
	o := newSATTacticOptions()
	if o.extrapolate != 0 || !o.projection || !o.strongSimplify || o.simplifyLocal {
		t.Fatal("source defaults")
	}
	key := "extrapolate_strategy"
	fmMust(t, c.SetUIntName(&p, &key, 3))
	key = "projection_mode"
	fmMust(t, c.SetBoolName(&p, &key, false))
	key = "strong_context_simplify"
	fmMust(t, c.SetBoolName(&p, &key, false))
	key = "strong_context_simplify_local"
	fmMust(t, c.SetBoolName(&p, &key, true))
	fmMust(t, o.update(c, &p))
	fmMust(t, o.update(c, &empty))
	if o.extrapolate != 3 || o.projection || o.strongSimplify || !o.simplifyLocal {
		t.Fatal("update did not preserve absent parameters")
	}
	d, err := uparams.NewParamDescrs(f.r.manager.Symbols, uparams.ParamDescrsConfig{Budget: f.r.manager.Budget()})
	fmMust(t, err)
	defer func() { fmMust(t, d.Close()) }()
	fmMust(t, collectSATTacticOptions(d))
	for _, tc := range []struct {
		name string
		kind uparams.ParamKind
	}{{"extrapolate_strategy", uparams.ParamUInt}, {"projection_mode", uparams.ParamBool}, {"strong_context_simplify", uparams.ParamBool}, {"strong_context_simplify_local", uparams.ParamBool}} {
		kind, e := d.KindName(tc.name)
		fmMust(t, e)
		if kind != tc.kind {
			t.Fatalf("wrong parameter kind for %s", tc.name)
		}
	}
}
