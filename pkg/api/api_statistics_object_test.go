package api

import (
	"errors"
	"testing"
)

func TestAPIStatisticsLifecycleOwnsSnapshot(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	source := apiStatisticsFixture()
	object, err := newAPIStatistics(f.c.smtContextState, source)
	astTestMust(t, err)
	source.Reset()
	if size, e := object.Size(); e != nil || size != 2 {
		t.Fatal("snapshot size", size, e)
	}
	astTestMust(t, object.Close())
	astTestMust(t, object.Close())
	if _, err = object.Size(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed statistics", err)
	}
}
