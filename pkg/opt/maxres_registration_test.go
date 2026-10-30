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
