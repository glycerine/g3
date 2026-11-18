package cmd_context

import (
	"bytes"

	"github.com/glycerine/g3/pkg/ast"
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

// AUDIT.md FUZZ_J-F4: opt_parse builds (op (ite c k 0) b) for every
// constraint, a one-term one included; there is no Boolean shortcut.
func TestLoadOPBSingleTermConstraintKeepsArithmeticShape(t *testing.T) {
	document, err := opt.ParseOPB(strings.NewReader("5 x1 >= 1;"))
	if err != nil {
		t.Fatal(err)
	}
	c := opbTestContext(t)
	c.InteractiveMode = true
	if _, err = c.LoadOPB(document); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = c.WriteAssertions(&out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.HasPrefix(text, "((>= (ite ") || !strings.HasSuffix(text, " 5 0) 1))\n") {
		t.Fatalf("assertions %q", out.String())
	}
}

// AUDIT.md cmd_context_01-F16: parse_id makes numerical-symbol constants
// and parse_ids nests binary ands (the printer flattens them).
func TestOPBLoaderBuildsSourceShapes(t *testing.T) {
	document, err := opt.ParseOPB(strings.NewReader("+3 x1 x2 x3 >= 2 ;"))
	if err != nil {
		t.Fatal(err)
	}
	c := opbTestContext(t)
	c.InteractiveMode = true
	if _, err = c.LoadOPB(document); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = c.WriteAssertions(&out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); text != "((>= (ite (and k!1 k!2 k!3) 3 0) 2))\n" {
		t.Fatalf("assertions %q", text)
	}
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	assertions, err := c.Assertions()
	if err != nil || len(assertions) != 1 {
		t.Fatal(assertions, err)
	}
	arg := func(app ast.ASTHandle, i uint32, count uint32) ast.ASTHandle {
		t.Helper()
		view, e := m.AppArgumentView(app)
		if e != nil || view.Count() != count {
			t.Fatalf("argument count %d, error %v; want %d", view.Count(), e, count)
		}
		h, e := view.At(i)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	condition := arg(arg(assertions[0], 0, 2), 0, 3)
	inner := arg(condition, 0, 2)
	arg(inner, 0, 2)
}
