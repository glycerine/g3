package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSMTContextASTSimplifyDefaultAndConfigured(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("simplify-x", boolean))
	y := f.pin(f.c.Constant("simplify-y", boolean))
	truth := f.pin(f.c.Bool(true))
	withIdentity := f.pin(f.c.And(x, truth))

	defaultRef, err := f.c.Simplify(withIdentity)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&defaultRef)) })
	defaultResult, err := f.m.RefGet(defaultRef)
	astTestMust(t, err)
	if defaultResult != x {
		t.Fatal("Boolean identity was not simplified", defaultResult, x)
	}

	conjunction := f.pin(f.c.And(x, y))
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	astTestMust(t, parameters.SetBool("elim-and", true))
	configuredRef, err := f.c.SimplifyWithParameters(conjunction, parameters)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&configuredRef)) })
	configured, err := f.m.RefGet(configuredRef)
	astTestMust(t, err)
	if configured == conjunction {
		t.Fatal("elim_and parameter did not change conjunction")
	}
	if yes, e := f.c.IsApplication(configured); e != nil || !yes {
		t.Fatal("configured simplified expression", yes, e)
	}
}

func TestSMTContextASTSimplifyDescriptionsHelpAndOwnership(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	descriptions, err := f.c.SimplifyParameterDescriptions()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, descriptions.Close()) })
	if kind, e := descriptions.Kind("elim-and"); e != nil || kind != ParameterBool {
		t.Fatal("elim_and description", kind, e)
	}
	if documentation, e := descriptions.Documentation("elim_and"); e != nil || documentation == "" {
		t.Fatal("elim_and documentation", documentation, e)
	}
	help, err := f.c.SimplifyHelp()
	astTestMust(t, err)
	if !strings.Contains(help, "elim_and") || !strings.Contains(help, "conjunctions") {
		t.Fatal("simplify help lacks parameter details", help)
	}

	other := newSMTAPIFixture(t, "ALL", nil)
	foreignParameters, err := other.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, foreignParameters.Close()) })
	value := f.pin(f.c.Bool(true))
	if _, err = f.c.SimplifyWithParameters(value, foreignParameters); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign parameter owner", err)
	}
}
