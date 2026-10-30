package cmd_context

import "testing"

func TestOptimizationExecutionSettingsUseContextDefaults(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetUIntOption("timeout", 37))
	contextMust(t, c.SetUIntOption("rlimit", 41))
	settings, err := c.optimizationSettings()
	contextMust(t, err)
	if settings.timeout != 37 || settings.rlimit != 41 {
		t.Fatal(settings)
	}
}
