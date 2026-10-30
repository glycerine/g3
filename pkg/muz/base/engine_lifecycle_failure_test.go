package base_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestEngineBaseDispatchPropagatesCapabilityFailures(t *testing.T) {
	core, engine := newEngineDispatchFixture(t)
	core.failure = errEngineDispatch
	checks := []func() error{
		engine.ResetStatistics,
		func() error { return engine.CollectStatistics(statistics.New()) },
		func() error { return engine.DisplayProfile(&bytes.Buffer{}) },
		func() error { return engine.DisplayCertificate(&bytes.Buffer{}) },
		engine.Cancel,
		engine.Cleanup,
		engine.UpdateParameters,
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, errEngineDispatch) {
			t.Fatalf("capability %d failure = %v", i, err)
		}
	}
}
