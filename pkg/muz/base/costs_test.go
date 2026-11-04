package base

// Source: src/muz/base/dl_costs.cpp concrete costs, accounted_object, and
// cost_recorder behavior.

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestCostsArithmeticThresholdAndOutput(t *testing.T) {
	cost := Costs{Milliseconds: 19, Instructions: 7}
	delta, err := cost.Subtract(Costs{Milliseconds: 4, Instructions: 2})
	if err != nil || delta != (Costs{Milliseconds: 15, Instructions: 5}) {
		t.Fatal("cost subtraction", delta, err)
	}
	if _, err = cost.Subtract(Costs{Milliseconds: 19, Instructions: 1}); !errors.Is(err, ErrCosts) {
		t.Fatal("source strict subtraction assertion", err)
	}
	if !cost.PassesThreshold(19) || cost.PassesThreshold(20) {
		t.Fatal("cost threshold")
	}
	var out bytes.Buffer
	if err = cost.Output(&out); err != nil || out.String() != "instr: 7  time: 19ms" {
		t.Fatal("cost output", out.String(), err)
	}
}

func TestCostRecorderSwitchesObjectsAndChargesWholeMilliseconds(t *testing.T) {
	now := time.Duration(0)
	recorder := newCostRecorder(func() time.Duration { return now })
	first := &AccountedObject{}
	second := &AccountedObject{}
	if err := recorder.Start(first); err != nil {
		t.Fatal(err)
	}
	now = 2500 * time.Microsecond
	if err := recorder.Start(second); err != nil {
		t.Fatal(err)
	}
	now = 7 * time.Millisecond
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	if first.beingRecorded || second.beingRecorded {
		t.Fatal("recorder left object active")
	}
	if first.current != (Costs{Milliseconds: 2, Instructions: 1}) || second.current != (Costs{Milliseconds: 5, Instructions: 1}) {
		t.Fatal("recorded costs", first.current, second.current)
	}
}

func TestCostRecorderTruncatesEachSampleBeforeSubtraction(t *testing.T) {
	now := 1900 * time.Microsecond
	recorder := newCostRecorder(func() time.Duration { return now })
	object := &AccountedObject{}
	if err := recorder.Start(object); err != nil {
		t.Fatal(err)
	}
	now = 2100 * time.Microsecond
	if err := recorder.Finish(); err != nil {
		t.Fatal(err)
	}
	if object.current != (Costs{Milliseconds: 1, Instructions: 1}) {
		t.Fatal("fractional boundary cost", object.current)
	}
}
