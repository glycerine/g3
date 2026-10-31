package qe

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestQuantifierTacticNonlinearOptionSnapshot(t *testing.T) {
	f := newFMFixture(t)
	ctx := f.r.config.Context
	enabled, err := ctx.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Del(&enabled)) }()
	empty, err := ctx.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, ctx.Del(&empty)) }()
	key := "qe_nonlinear"
	fmMust(t, ctx.SetBoolName(&enabled, &key, true))
	descriptions, err := uparams.NewParamDescrs(f.r.manager.Symbols, uparams.ParamDescrsConfig{Budget: f.r.manager.Budget()})
	fmMust(t, err)
	defer func() { fmMust(t, descriptions.Close()) }()
	fmMust(t, collectQuantifierTacticOptions(descriptions))
	kind, err := descriptions.KindName("qe_nonlinear")
	fmMust(t, err)
	if kind != uparams.ParamBool {
		t.Fatal("option descriptor", kind)
	}
	var options quantifierTacticOptions
	fmMust(t, options.update(ctx, &enabled))
	if !options.nonlinear {
		t.Fatal("enabled option ignored")
	}
	fmMust(t, ctx.SetBoolName(&enabled, &key, false))
	if !options.nonlinear {
		t.Fatal("option state unexpectedly aliased")
	}
	fmMust(t, options.update(ctx, &empty))
	if options.nonlinear {
		t.Fatal("absent option did not reset")
	}
}
