package main

import (
	"os"
	"testing"
	"time"
)

// BV_RM_VAL encodes ties-to-even as 0 and ties-to-away as 1; the converter
// and the model's bv2rm decoder must agree, or an RM excluded by the
// assertions is reported as its value.
func TestFPARoundingModeModelValuesCPPReference(t *testing.T) {
	script, err := os.ReadFile("test_vectors/qffp_rounding_mode_model_values.smt2")
	if err != nil {
		t.Fatal(err)
	}
	want := "sat\n((r roundNearestTiesToEven)\n (x (fp #b0 #b011 #b000)))\nsat\n((s roundNearestTiesToAway))\n"
	if got := runScriptWithin(t, string(script), time.Minute); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
