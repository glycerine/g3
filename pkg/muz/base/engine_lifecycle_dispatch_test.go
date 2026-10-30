package base_test

import "testing"

func TestEngineBaseDispatchesCancelCleanupAndParameterUpdate(t *testing.T) {
	core, engine := newEngineDispatchFixture(t)
	ruleMust(t, engine.Cancel())
	ruleMust(t, engine.Cleanup())
	ruleMust(t, engine.UpdateParameters())
	if core.cancel != 1 || core.cleanup != 1 || core.update != 1 {
		t.Fatal("lifecycle dispatch", core.cancel, core.cleanup, core.update)
	}
}
