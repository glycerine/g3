package duality_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/duality"
)

func TestDualityQueryOptionsDefaultsAndLocalModulePrecedence(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := cmd_context.NewStandaloneContext()
	must(err)
	defer owner.Close()
	config, err := owner.SolverTranslationConfig()
	must(err)
	defaults, err := duality.ReadQueryOptions(config.Params, config.Globals, nil)
	must(err)
	if defaults != duality.DefaultQueryOptions() || defaults.UseUnderapprox || !defaults.FeasibleEdges {
		t.Fatal("fixedpoint defaults differ from source", defaults)
	}
	for _, name := range []string{"full_expand", "no_conj", "use_underapprox", "stratified_inlining", "batch_expand", "enable_restarts", "profile"} {
		must(config.Globals.SourceSetName("fixedpoint.duality."+name, "true"))
	}
	must(config.Globals.SourceSetName("fixedpoint.duality.feasible_edges", "false"))
	must(config.Globals.SourceSetName("fixedpoint.duality.conjecture_file", "global-conjectures"))
	global, err := duality.ReadQueryOptions(config.Params, config.Globals, nil)
	must(err)
	if !global.Profile || !global.FullExpand || !global.NoConj || global.FeasibleEdges || !global.UseUnderapprox || !global.StratifiedInlining || !global.BatchExpand || !global.EnableRestarts || global.ConjectureFile != "global-conjectures" {
		t.Fatal("module option lookup failed", global)
	}
	local := config.Params.EmptyValue()
	defer config.Params.Del(&local)
	name := "duality.full_expand"
	must(config.Params.SetBoolName(&local, &name, false))
	name = "duality.profile"
	must(config.Params.SetBoolName(&local, &name, false))
	name = "duality.feasible_edges"
	must(config.Params.SetBoolName(&local, &name, true))
	name = "duality.conjecture_file"
	text := "local-conjectures"
	must(config.Params.SetStringName(&local, &name, &text))
	override, err := duality.ReadQueryOptions(config.Params, config.Globals, &local)
	must(err)
	if override.Profile || override.FullExpand || !override.FeasibleEdges || override.ConjectureFile != "local-conjectures" || !override.NoConj || !override.UseUnderapprox || !override.StratifiedInlining || !override.BatchExpand || !override.EnableRestarts {
		t.Fatal("local/module precedence failed", override)
	}
	// Empty local strings override populated module strings.
	text = ""
	must(config.Params.SetStringName(&local, &name, &text))
	empty, err := duality.ReadQueryOptions(config.Params, config.Globals, &local)
	must(err)
	if empty.ConjectureFile != "" {
		t.Fatal("empty local filename did not override module")
	}
	if global.ConjectureFile != "global-conjectures" {
		t.Fatal("read options did not own a stable string value")
	}
}
