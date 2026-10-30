package api

import (
	"math"
	"testing"
)

func TestSMTContextOperationTimeoutTracksContextOption(t *testing.T) {
	context := newAPIInterruptFixture(t)
	if timeout, err := context.contextTimeout(); err != nil || timeout != math.MaxUint32 {
		t.Fatal("default timeout", timeout, err)
	}
	if err := context.SetUIntOption("timeout", 37); err != nil {
		t.Fatal(err)
	}
	if timeout, err := context.contextTimeout(); err != nil || timeout != 37 {
		t.Fatal("configured timeout", timeout, err)
	}
}
