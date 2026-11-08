package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextDisplayDefinitionsShareVisitedAcrossActualOwners(t *testing.T) {
	m, solver := smtAssertedNNFFixture(t, false)
	s := solver.kernel.internalizer
	a := smtRelevancyBool(t, m, "dump-def-a")
	b := smtRelevancyBool(t, m, "dump-def-b")
	astTestMust(t, s.Internalize(a, true))
	v, _ := s.BoolVar(a)
	astTestMust(t, s.ensureBooleanEnode(a, v, true, false, false))
	astTestMust(t, solver.Assert(a))
	astTestMust(t, solver.Assert(a))
	astTestMust(t, solver.Assert(b))
	aid, err := m.GetASTID(a)
	astTestMust(t, err)
	bid, err := m.GetASTID(b)
	astTestMust(t, err)
	// context::init's true and false enodes are displayed after a's Boolean definition.
	want := fmt.Sprintf("#%d := dump-def-a\n#1 := true\n#2 := false\n#%d := dump-def-b\nasserted formulas:\n#%d #%d #%d \n", aid, bid, aid, aid, bid)
	var out bytes.Buffer
	for i := 0; i < 2; i++ {
		out.Reset()
		astTestMust(t, s.DisplayDefinitions(&out))
		if out.String() != want {
			t.Fatal("definitions must be shared within each dump and reset between dumps", out.String(), want)
		}
	}
	astTestMust(t, s.PushScope())
	c := smtRelevancyBool(t, m, "dump-def-child")
	astTestMust(t, s.Internalize(c, true))
	out.Reset()
	astTestMust(t, s.DisplayDefinitions(&out))
	if !strings.Contains(out.String(), " := dump-def-child\n") || strings.Count(out.String(), " := dump-def-a\n") != 1 {
		t.Fatal(out.String())
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, s.DisplayDefinitions(&out))
	if out.String() != want {
		t.Fatal("popped Boolean definition survived", out.String(), want)
	}
}

func TestSMTContextDisplayHotVariablesThresholdDecayAndWidths(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "dump-hot-a")
	b := smtRelevancyBool(t, m, "dump-hot-b")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, s.Internalize(b, true))
	av, _ := s.BoolVar(a)
	bv, _ := s.BoolVar(b)
	for i := 0; i < 11; i++ {
		astTestMust(t, s.incrementBooleanActivity(av))
		if i < 10 {
			astTestMust(t, s.incrementBooleanActivity(bv))
		}
	}
	id, err := m.GetASTID(a)
	astTestMust(t, err)
	var out bytes.Buffer
	astTestMust(t, s.DisplayHotBooleanVariables(&out))
	// Source width5 AST identity followed by two spaces and width12 values.
	want := "hot bool vars:\n" + fmt.Sprintf("#%-5d", id) + "            11            11\n"
	if out.String() != want {
		t.Fatalf("width/strict threshold: got %q want %q", out.String(), want)
	}
	s.decayBooleanActivity()
	out.Reset()
	astTestMust(t, s.DisplayHotBooleanVariables(&out))
	want = "hot bool vars:\n" + fmt.Sprintf("#%-5d", id) + "            11       10.4563\n"
	if out.String() != want {
		t.Fatalf("six significant digits: got %q want %q", out.String(), want)
	}
	s.decayBooleanActivity()
	out.Reset()
	astTestMust(t, s.DisplayHotBooleanVariables(&out))
	if out.String() != "hot bool vars:\n" || s.activity[av] != 11 || s.activity[bv] != 10 {
		t.Fatal("normalized threshold or borrowed activity changed", out.String())
	}
}

func TestSMTContextDisplayDefinitionsAndActivityWriterPrefixes(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "dump-stream-a")
	astTestMust(t, s.Internalize(a, true))
	v, _ := s.BoolVar(a)
	for i := 0; i < 12; i++ {
		astTestMust(t, s.incrementBooleanActivity(v))
	}
	methods := []func(io.Writer) error{s.DisplayDefinitions, s.DisplayHotBooleanVariables}
	for _, display := range methods {
		astTestMust(t, display(io.Discard))
	}
	before := m.Stats()
	for _, display := range methods {
		var out bytes.Buffer
		astTestMust(t, display(&out))
		want := out.String()
		for cut := 0; cut < len(want); cut++ {
			w := &smtLRAOutputPrefix{remaining: cut}
			if err := display(w); !errors.Is(err, io.ErrClosedPipe) || w.text.String() != want[:cut] {
				t.Fatal(cut, err, w.text.String(), want[:cut])
			}
		}
		if err := display(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal(err)
		}
		if err := display(nil); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal(err)
		}
	}
	if m.Stats() != before {
		t.Fatal("display changed manager ownership", before, m.Stats())
	}
	var absent *SMTInternalizerState
	for _, display := range []func(io.Writer) error{absent.DisplayDefinitions, absent.DisplayHotBooleanVariables} {
		if err := display(io.Discard); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal(err)
		}
	}
}

func TestSMTContextDisplayDefinitionsAllocationRefusalRetainsInputs(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "dump-refusal-a")
	astTestMust(t, s.Internalize(a, true))
	before := m.Stats()
	m.Budget().FailAfter(0)
	var out bytes.Buffer
	err := s.DisplayDefinitions(&out)
	m.Budget().FailAfter(-1)
	if err == nil || out.Len() != 0 {
		t.Fatal("cold formatter storage refusal was ignored", err, out.String())
	}
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveParameters != before.LiveParameters || after.LiveInfos != before.LiveInfos {
		t.Fatal("refusal changed AST ownership", before, after)
	}
	astTestMust(t, s.DisplayDefinitions(&out))
	id, err := m.GetASTID(a)
	astTestMust(t, err)
	if want := fmt.Sprintf("#%d := dump-refusal-a\n#1 := true\n#2 := false\n", id); out.String() != want {
		t.Fatal("retained input failed on retry", out.String(), want)
	}
}
