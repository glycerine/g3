package api

import (
	"bytes"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeSoftGroupAndObjectiveDisplay(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-object-opt-p", boolean))
	notP := f.pin(f.c.Not(p))
	first, err := f.opt.AssertSoft(p, big.NewRat(2, 1), "preferences")
	astTestMust(t, err)
	second, err := f.opt.AssertSoft(notP, big.NewRat(1, 1), "preferences")
	astTestMust(t, err)
	if first != second || first != 0 {
		t.Fatal("soft group indices", first, second)
	}
	objective, err := f.opt.Objective(first)
	astTestMust(t, err)
	if !objective.Soft || len(objective.SoftTerms) != 2 {
		t.Fatal("soft objective", objective)
	}
	replay, err := f.opt.SMT2()
	astTestMust(t, err)
	if strings.Count(replay, "(assert-soft ") != 2 || !strings.Contains(replay, ":weight 2 :id preferences") {
		t.Fatal("soft objective replay", replay)
	}
	status, err := f.opt.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("soft optimize status", status)
	}
	value, err := f.opt.ObjectiveValue(first)
	astTestMust(t, err)
	if !value.Available || value.SoftCost.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("soft optimum", value)
	}
	var output bytes.Buffer
	astTestMust(t, f.opt.WriteObjectives(&output))
	if !strings.Contains(output.String(), "preferences 1") {
		t.Fatal("soft objective output", output.String())
	}
}
