package opt

import "testing"

func TestMaxResRegistrationRejectsUnsupportedEngines(t *testing.T) {
	for _, engine := range []string{"wmax", "unknown"} {
		got, err := maxSMTFallbackFactory(nil, nil, 0, maxSMTConfig{engine: engine})
		if err != nil {
			t.Fatalf("engine %q: %v", engine, err)
		}
		if got != nil {
			t.Fatalf("engine %q unexpectedly selected MaxRes", engine)
		}
	}
}

func TestMaxSMTUnknownEngineUsesSourceMaxResFallback(t *testing.T) {
	previous := maxSMTFallbackFactory
	t.Cleanup(func() { maxSMTFallbackFactory = previous })
	selected := ""
	maxSMTFallbackFactory = func(_ *ArithmeticSolver, _ *arithmeticObjective, _ int, config maxSMTConfig) (maxSMTEngine, error) {
		selected = config.engine
		return nil, nil
	}
	if _, err := selectMaxSMTEngine(nil, nil, 0, maxSMTConfig{engine: "unrecognized"}); err != nil {
		t.Fatal(err)
	}
	if selected != "maxres" {
		t.Fatal("unknown maxsat engine fallback", selected)
	}
}
