package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"strings"
	"testing"
)

func TestAPIFixedpointSMT2IncludesOrderedQueries(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	truth := f.pin(f.m.MkTrue())
	astTestMust(t, f.fp.AddFact(f.p, nil))
	astTestMust(t, f.fp.AddRule(f.step, "api-display-step"))
	astTestMust(t, f.fp.Assert(truth))
	text, err := f.fp.SMT2(f.pAtom, f.qAtom)
	astTestMust(t, err)
	fragments := []string{
		"(declare-rel api-object-fp-p ())",
		"(assert true)",
		":named api-display-step",
		"(query api-object-fp-p)",
		"(query api-object-fp-q)",
	}
	position := -1
	for _, fragment := range fragments {
		next := strings.Index(text, fragment)
		if next < 0 {
			t.Fatalf("fixedpoint display omitted %q:\n%s", fragment, text)
		}
		if strings.HasPrefix(fragment, "(query") && next <= position {
			t.Fatalf("fixedpoint query order in:\n%s", text)
		}
		if strings.HasPrefix(fragment, "(query") {
			position = next
		}
	}
}

func TestAPIFixedpointSMT2UsesLocalPrintProfilesWithoutChangingResults(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.AddFact(f.p, nil))
	astTestMust(t, f.fp.AddRule(f.step, "display-step"))
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetBool("print_fixedpoint_extensions", false))
	astTestMust(t, f.fp.SetParameters(parameters))
	astTestMust(t, parameters.Close())
	status, err := f.fp.Query(f.q)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("query", status)
	}
	before := f.m.Stats().LiveReferences
	text, err := f.fp.SMT2(f.pAtom, f.qAtom)
	astTestMust(t, err)
	for _, part := range []string{"(set-logic HORN)", "(declare-fun api-object-fp-p () Bool)", "(check-sat)"} {
		if !strings.Contains(text, part) {
			t.Fatal("local plain profile missing", part, text)
		}
	}
	if strings.Contains(text, "declare-rel") || strings.Contains(text, "api-object-fp-u") {
		t.Fatal("wrong context declaration policy", text)
	}
	if f.m.Stats().LiveReferences != before {
		t.Fatal("API dump leaked owners")
	}
	answer, err := f.fp.Answer(f.q)
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&answer))
	other, err := f.c.NewFixedpoint()
	astTestMust(t, err)
	defer func() { astTestMust(t, other.Close()) }()
	d, err := other.DeclareRelation("other-display", nil)
	astTestMust(t, err)
	astTestMust(t, other.AddFact(d, nil))
	app := f.pin(f.m.MkApp(d, nil))
	separate, err := other.SMT2(app)
	astTestMust(t, err)
	if !strings.Contains(separate, "(declare-rel other-display ())") || strings.Contains(separate, "set-logic HORN") {
		t.Fatal("print options leaked to another fixedpoint", separate)
	}
}

func TestAPIFixedpointSMT2BridgesCompoundQueriesAndRejectsPlainBackground(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.AddFact(f.p, nil))
	astTestMust(t, f.fp.AddRule(f.step, "bridge-step"))
	query := f.pin(f.m.MkAndArray([]ast.ASTHandle{f.pAtom, f.qAtom}))
	text, err := f.fp.SMT2(query)
	astTestMust(t, err)
	for _, part := range []string{"(declare-rel q!", "(assert ", "(query q!"} {
		if !strings.Contains(text, part) {
			t.Fatal("missing source query bridge", part, text)
		}
	}
	if strings.Contains(text, "(query (and") {
		t.Fatal("compound query bypassed source bridge", text)
	}
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetBool("print_fixedpoint_extensions", false))
	astTestMust(t, f.fp.SetParameters(parameters))
	astTestMust(t, parameters.Close())
	truth := f.pin(f.m.MkTrue())
	astTestMust(t, f.fp.Assert(truth))
	status, err := f.fp.Query(f.q)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("query with true background", status)
	}
	before := f.m.Stats().LiveReferences
	text, err = f.fp.SMT2(f.qAtom)
	if err == nil || !strings.Contains(err.Error(), "Background axioms cannot be used with SMT-LIB2 HORN format") || text != "" {
		t.Fatal("source background rejection", text, err)
	}
	if f.m.Stats().LiveReferences != before {
		t.Fatal("API dump error leaked owners")
	}
	answer, err := f.fp.Answer(f.q)
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&answer))
}
