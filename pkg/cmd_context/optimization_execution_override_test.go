package cmd_context

import "testing"

func TestOptimizationExecutionSettingsReadOptimizeLocalOverrides(t *testing.T) {
	c := contextFixture(t)
	local, err := c.config.Params.NewValue()
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&local)) }()
	timeoutName, rlimitName := "timeout", "rlimit"
	contextMust(t, c.config.Params.SetUIntName(&local, &timeoutName, 11))
	contextMust(t, c.config.Params.SetUIntName(&local, &rlimitName, 13))
	contextMust(t, c.UpdateOptimizationParameters(&local))
	settings, err := c.optimizationSettings()
	contextMust(t, err)
	if settings.timeout != 11 || settings.rlimit != 13 {
		t.Fatal(settings)
	}
}
