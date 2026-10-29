package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func apiStatisticsFixture() statistics.Statistics {
	var value statistics.Statistics
	value.UpdateUint("integer counter", 7)
	value.UpdateDouble(":floating counter", 2.5)
	return value
}

func TestAPIStatisticsKeysKindsAndBounds(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	object, err := newAPIStatistics(f.c.smtContextState, apiStatisticsFixture())
	astTestMust(t, err)
	defer func() { astTestMust(t, object.Close()) }()
	if key, e := object.Key(0); e != nil || key != "integer counter" {
		t.Fatal("uint key", key, e)
	}
	if yes, e := object.IsUint(0); e != nil || !yes {
		t.Fatal("uint kind", yes, e)
	}
	if key, e := object.Key(1); e != nil || key != ":floating counter" {
		t.Fatal("double key", key, e)
	}
	if yes, e := object.IsDouble(1); e != nil || !yes {
		t.Fatal("double kind", yes, e)
	}
	if _, err = object.Key(2); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("key bounds", err)
	}
}
