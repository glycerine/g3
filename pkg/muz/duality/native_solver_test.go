package duality_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	graph "github.com/glycerine/g3/pkg/duality"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestDualityNativeSolverSettingsAndProofModeRestoration(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := cmd_context.NewStandaloneContext()
	must(err)
	defer owner.Close()
	m, err := owner.Manager()
	must(err)
	initial, err := m.ProofMode()
	must(err)
	settings, err := owner.SolverTranslationConfig()
	must(err)
	config, err := graph.NewConfig(settings.Params, nil)
	must(err)
	defer config.Close()
	context, err := graph.NewContext(m, config)
	must(err)
	defer context.Close()
	factory, err := smtcore.NewSMTSolverFactory(settings)
	must(err)
	defer factory.Close()
	for _, mbqi := range []bool{false, true} {
		native, err := duality.NewNativeSolver(context, settings.Params, factory, mbqi)
		must(err)
		p, err := native.Params()
		must(err)
		for _, tc := range []struct {
			name string
			want bool
		}{
			{"proof", true}, {"model", true}, {"unsat_core", true}, {"eq2ineq", true}, {"mbqi", mbqi},
		} {
			got, err := settings.Params.GetBoolName(p, &tc.name, !tc.want)
			must(err)
			if got != tc.want {
				t.Fatalf("%s=%v want %v", tc.name, got, tc.want)
			}
		}
		name := "mbqi.id"
		id, present, err := settings.Params.GetStringName(p, &name, nil)
		must(err)
		if !present || id != "itp" {
			t.Fatal("interpolation quantifier id missing", id, present)
		}
		name = "mbqi.max_iterations"
		iterations, err := settings.Params.GetUIntName(p, &name, 0)
		must(err)
		if iterations != 1 {
			t.Fatal("quantifier iteration limit mismatch")
		}
		if mbqi {
			name = "array.extensional"
			value, err := settings.Params.GetBoolName(p, &name, false)
			must(err)
			if !value {
				t.Fatal("extensional arrays not enabled with mbqi")
			}
		}
		mode, err := m.ProofMode()
		must(err)
		if mode != initial {
			t.Fatal("native construction leaked proof mode")
		}
		must(native.Close())
	}
	if native, err := duality.NewNativeSolver(context, settings.Params, nil, true); err == nil || native != nil {
		t.Fatal("missing factory accepted")
	}
	mode, err := m.ProofMode()
	must(err)
	if mode != initial {
		t.Fatal("failed construction changed proof mode")
	}
}
