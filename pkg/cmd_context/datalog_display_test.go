package cmd_context

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogDisplayRepeatedLoadsAndReset(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.ParseDatalogString("D 2\n\nP(x:D) printtuples\nP(\"alpha\").\n"))
	contextMust(t, c.ParseDatalogString("D 2\n\nQ(x:D) outputtuples\nQ(\"beta\").\n"))
	rows := wpaSemanticSnapshots(t, c, "P", "Q")
	if !reflect.DeepEqual(rows[0].Facts, [][]uint64{{0}}) || !reflect.DeepEqual(rows[1].Facts, [][]uint64{{1}}) {
		t.Fatal("repeated parsers did not share source domain ordinals", rows)
	}
	var out bytes.Buffer
	contextMust(t, c.WriteFixedpointDatalogTuples(&out))
	want := "Tuples in P: \n\t(x=alpha(0))\nTuples in Q: \n\t(x=beta(1))\n"
	if out.String() != want {
		t.Fatalf("repeated load output = %q, want %q", out.String(), want)
	}
	owner := c.datalogDisplay
	contextMust(t, c.Reset())
	if c.datalogDisplay != nil {
		t.Fatal("reset left the display owner attached")
	}
	if _, err := owner.Manager(); !errors.Is(err, base.ErrDatalogContextOwner) {
		t.Fatal("reset did not close the old display owner", err)
	}
	contextMust(t, c.ParseDatalogString("D 2\n\nR(x:D) printtuples\nR(\"gamma\").\n"))
	rows = wpaSemanticSnapshots(t, c, "R")
	if !reflect.DeepEqual(rows[0].Facts, [][]uint64{{0}}) {
		t.Fatal("reset reused old domain numbering", rows)
	}
	out.Reset()
	contextMust(t, c.WriteFixedpointDatalogTuples(&out))
	if want := "Tuples in R: \n\t(x=gamma(0))\n"; out.String() != want {
		t.Fatalf("reset output = %q, want %q", out.String(), want)
	}
}

func TestDatalogDisplayRepeatedWPANumericLoads(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("fixedpoint.datalog.use_map_names", false))
	for _, input := range []struct{ predicate, number string }{{"P", "7"}, {"Q", "19"}} {
		dir := wpaSemanticDirectory(t, map[string]string{
			"V.map":                  "7 alpha\n19 beta\n",
			"program.rules":          input.predicate + "(x:V) outputtuples\n",
			input.predicate + ".rel": input.number + "\n",
		})
		contextMust(t, c.ParseWPADirectory(dir))
	}
	rows := wpaSemanticSnapshots(t, c, "P", "Q")
	if !reflect.DeepEqual(rows[0].Facts, [][]uint64{{0}}) || !reflect.DeepEqual(rows[1].Facts, [][]uint64{{1}}) {
		t.Fatal("repeated WPA parsers did not share numeric ordinals", rows)
	}
	var out bytes.Buffer
	contextMust(t, c.WriteFixedpointDatalogTuples(&out))
	want := "Tuples in P: \n\t(x=7(0))\nTuples in Q: \n\t(x=19(1))\n"
	if out.String() != want {
		t.Fatalf("numeric output = %q, want %q", out.String(), want)
	}
}
