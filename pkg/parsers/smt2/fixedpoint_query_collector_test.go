package smt2_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestFixedpointQueryCollectorScopesAndVariables(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := c.Close(); e != nil {
			t.Error(e)
		}
	}()
	script := `(declare-rel R (Int Bool)) (declare-rel S ()) (query R) (fixedpoint-push) (query S) (fixedpoint-pop) (query S)`
	refs, err := smt2.ParseAssertionsWithFixedpointQueries(c, strings.NewReader(script))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for i := range refs {
			if e := m.DelRef(&refs[i]); e != nil {
				t.Error(e)
			}
		}
	}()
	if len(refs) != 2 {
		t.Fatalf("got %d queries, want 2", len(refs))
	}
	for i, want := range []uint32{2, 0} {
		h, e := m.RefGet(refs[i])
		if e != nil {
			t.Fatal(e)
		}
		count, e := m.GetNumArgs(h)
		if e != nil || count != want {
			t.Fatalf("query %d arity %d: %v", i, count, e)
		}
		for j := uint32(0); j < count; j++ {
			a, e := m.GetArg(h, j)
			if e != nil {
				t.Fatal(e)
			}
			index, e := m.GetVarIndex(a)
			if e != nil || index != j {
				t.Fatalf("variable %d index %d: %v", j, index, e)
			}
		}
	}
}

func TestFixedpointQueryCollectorParseErrorCleansUp(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := c.Close(); e != nil {
			t.Error(e)
		}
	}()
	_, err = smt2.ParseAssertionsWithFixedpointQueries(c, strings.NewReader(`(declare-rel R (Int)) (query R) (fixedpoint-push) (query R) (assert`))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFixedpointQueryCollectorOrdinaryScopesDoNotDiscardQueries(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := c.Close(); e != nil {
			t.Error(e)
		}
	}()
	refs, err := smt2.ParseAssertionsWithFixedpointQueries(c, strings.NewReader(`(declare-rel R ()) (push) (query R) (pop)`))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for i := range refs {
			if e := m.DelRef(&refs[i]); e != nil {
				t.Error(e)
			}
		}
	}()
	if len(refs) != 1 {
		t.Fatalf("got %d queries, want 1", len(refs))
	}
}

func TestFixedpointProgramCollectsBoundRulesAndScopes(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := c.Close(); e != nil {
			t.Error(e)
		}
	}()
	script := `(declare-rel R (Int)) (declare-var x Int) (rule (R x) first) (fixedpoint-push) (rule (R x) dropped) (query R) (fixedpoint-pop) (query R)`
	p, err := smt2.ParseFixedpointProgram(c, strings.NewReader(script))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for i := range p.Queries {
			if e := m.DelRef(&p.Queries[i]); e != nil {
				t.Error(e)
			}
		}
		for i := range p.Rules {
			if e := m.DelRef(&p.Rules[i]); e != nil {
				t.Error(e)
			}
		}
	}()
	if len(p.Rules) != 1 || len(p.Names) != 1 || p.Names[0] != "first" || len(p.Queries) != 1 {
		t.Fatalf("rules %d names %v queries %d", len(p.Rules), p.Names, len(p.Queries))
	}
	h, err := m.RefGet(p.Rules[0])
	if err != nil {
		t.Fatal(err)
	}
	kind, err := m.GetKind(h)
	if err != nil {
		t.Fatal(err)
	}
	if kind != ast.ASTQuantifier {
		t.Fatalf("bound rule kind %v", kind)
	}
	engineRules, err := c.FixedpointRules()
	if err != nil {
		t.Fatal(err)
	}
	for i := range engineRules {
		if e := m.DelRef(&engineRules[i]); e != nil {
			t.Error(e)
		}
	}
	if len(engineRules) != 0 {
		t.Fatal("collector ingested rules", len(engineRules))
	}
}
