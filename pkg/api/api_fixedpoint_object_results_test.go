package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointOwningAnswerAndStatistics(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.AddFact(f.p, nil))
	status, err := f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("fixedpoint query", status)
	}
	answer, err := f.fp.Answer(f.p)
	astTestMust(t, err)
	answerHandle, err := f.m.RefGet(answer)
	astTestMust(t, err)
	if !f.m.IsTrue(answerHandle) {
		t.Fatal("fixedpoint answer is not true")
	}
	astTestMust(t, f.m.DelRef(&answer))

	value, err := f.fp.Statistics()
	astTestMust(t, err)
	if value.Size() == 0 {
		t.Fatal("fixedpoint statistics are empty")
	}
	object, err := f.fp.StatisticsObject()
	astTestMust(t, err)
	if size, x := object.Size(); x != nil || size == 0 {
		t.Fatal("fixedpoint statistics object", size, x)
	}
	astTestMust(t, object.Close())
}
