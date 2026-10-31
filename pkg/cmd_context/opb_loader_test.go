package cmd_context

import (
	"strings"
	"testing"

	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func opbTestContext(t *testing.T) *Context {
	t.Helper()
	c, err := NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SetLogic("QF_LIA"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c
}

func TestLoadOPBConjunctionMonomialAndNegatedLiteral(t *testing.T) {
	document, err := opt.ParseOPB(strings.NewReader("1 x1 x2 >= 1; 1 ~x1 >= 1;"))
	if err != nil {
		t.Fatal(err)
	}
	c := opbTestContext(t)
	summary, err := c.LoadOPB(document)
	if err != nil {
		t.Fatal(err)
	}
	if summary != (OPBLoadSummary{Variables: 2, Constraints: 2}) {
		t.Fatal(summary)
	}
	status, err := c.CheckSat(nil)
	if err != nil || status != lbool.False {
		t.Fatal(status, err)
	}
}

func TestLoadOPBThreeConstraintRelations(t *testing.T) {
	document, err := opt.ParseOPB(strings.NewReader("1 x1 >= 1; 1 x1 <= 1; 1 x1 = 1;"))
	if err != nil {
		t.Fatal(err)
	}
	c := opbTestContext(t)
	if _, err = c.LoadOPB(document); err != nil {
		t.Fatal(err)
	}
	status, err := c.CheckSat(nil)
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
}

func TestLoadOPBSingleTermConstraintTruthTable(t *testing.T) {
	for _, tc := range []struct {
		name, constraint, forced string
		want                     lbool.LBool
	}{
		{"greater requires true", "2 x1 >= 1;", "1 ~x1 >= 1;", lbool.False},
		{"less requires false", "-2 x1 <= -1;", "1 ~x1 >= 1;", lbool.False},
		{"equality accepts false", "-2 x1 = 0;", "1 ~x1 >= 1;", lbool.True},
		{"always true", "0 x1 = 0;", "1 x1 >= 1;", lbool.True},
		{"always false", "2 x1 = 1;", "1 x1 >= 1;", lbool.False},
		{"negated monomial", "2 ~x1 x2 >= 1;", "1 x1 >= 1;", lbool.False},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document, err := opt.ParseOPB(strings.NewReader(tc.constraint + " " + tc.forced))
			if err != nil {
				t.Fatal(err)
			}
			c := opbTestContext(t)
			if _, err = c.LoadOPB(document); err != nil {
				t.Fatal(err)
			}
			status, err := c.CheckSat(nil)
			if err != nil || status != tc.want {
				t.Fatalf("status %v, error %v, want %v", status, err, tc.want)
			}
		})
	}
}
