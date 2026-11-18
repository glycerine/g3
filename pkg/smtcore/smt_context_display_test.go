package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextDisplayAssignmentLiteralOrderAndPop(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "display-a")
	b := smtRelevancyBool(t, m, "display-b")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, s.Internalize(b, true))
	av, _ := s.BoolVar(a)
	bv, _ := s.BoolVar(b)
	aid, err := m.GetASTID(a)
	astTestMust(t, err)
	bid, err := m.GetASTID(b)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(bv, true), NewSMTBooleanAxiom()))
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(av, false), NewSMTBooleanAxiom()))
	want := fmt.Sprintf("current assignment:\n(not #%d): (not display-b)\n#%d: display-a\n", bid, aid)
	var out bytes.Buffer
	astTestMust(t, s.DisplayAssignment(&out))
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, s.DisplayAssignment(&out))
	want = fmt.Sprintf("current assignment:\n(not #%d): (not display-b)\n", bid)
	if out.String() != want {
		t.Fatal("assignment pop", out.String(), want)
	}
}

func TestSMTContextDisplayMapsAndEquivalenceClassesUseSourceOrder(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	_, terms := smtUFConstants(t, m, "display-uf-", 3)
	// Internalization order intentionally differs from AST creation order.
	for _, i := range []int{1, 0} {
		astTestMust(t, s.Internalize(terms[i], false))
	}
	id := make([]uint32, 3)
	for i, term := range terms {
		var err error
		id[i], err = m.GetASTID(term)
		astTestMust(t, err)
	}
	var out bytes.Buffer
	astTestMust(t, s.DisplayApplicationEnodeMap(&out))
	// context::init's true and false enodes come first.
	base := fmt.Sprintf("expression -> enode:\n(#1 -> e!0) (#2 -> e!1) (#%d -> e!2) (#%d -> e!3) \n", id[1], id[0])
	if out.String() != base {
		t.Fatal(out.String(), base)
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, s.Internalize(terms[2], false))
	astTestMust(t, s.addEquality(s.context.enodes.get(terms[1]), s.context.enodes.get(terms[0]), NewSMTEqualityAxiom()))
	out.Reset()
	astTestMust(t, s.DisplayEquivalenceClasses(&out))
	want := fmt.Sprintf("equivalence classes:\n#%d -> #%d: display-uf-b -> display-uf-a\n", id[1], id[0])
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	out.Reset()
	astTestMust(t, s.DisplayApplicationEnodeMap(&out))
	want = fmt.Sprintf("expression -> enode:\n(#1 -> e!0) (#2 -> e!1) (#%d -> e!2) (#%d -> e!3) (#%d -> e!4) \n", id[1], id[0], id[2])
	if out.String() != want {
		t.Fatal("map must not substitute class root", out.String(), want)
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, s.DisplayEquivalenceClasses(&out))
	if out.Len() != 0 {
		t.Fatal("popped equality still displayed", out.String())
	}
	astTestMust(t, s.DisplayApplicationEnodeMap(&out))
	if out.String() != base {
		t.Fatal("popped enode still displayed", out.String(), base)
	}

	a := smtRelevancyBool(t, m, "display-map-a")
	b := smtRelevancyBool(t, m, "display-map-b")
	astTestMust(t, s.Internalize(b, true))
	astTestMust(t, s.PushScope())
	astTestMust(t, s.Internalize(a, true))
	av, _ := s.BoolVar(a)
	bv, _ := s.BoolVar(b)
	aid, err := m.GetASTID(a)
	astTestMust(t, err)
	bid, err := m.GetASTID(b)
	astTestMust(t, err)
	out.Reset()
	astTestMust(t, s.DisplayExpressionBoolVarMap(&out))
	want = fmt.Sprintf("expression -> bool_var:\n(#1 -> p!0) (#%d -> p!%d) (#%d -> p!%d) \n", bid, bv, aid, av)
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, s.DisplayExpressionBoolVarMap(&out))
	want = fmt.Sprintf("expression -> bool_var:\n(#1 -> p!0) (#%d -> p!%d) \n", bid, bv)
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
}

func TestSMTContextDisplayPrefixesBorrowingAndGuards(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	methods := []func(io.Writer) error{s.DisplayAssignment, s.DisplayEquivalenceClasses, s.DisplayExpressionBoolVarMap, s.DisplayApplicationEnodeMap}
	for _, display := range methods {
		var out bytes.Buffer
		astTestMust(t, display(&out))
		// A fresh context already maps context::init's true and false enodes,
		// and true is bool_var 0.
		if out.Len() != 0 && out.String() != "expression -> enode:\n(#1 -> e!0) (#2 -> e!1) \n" && out.String() != "expression -> bool_var:\n(#1 -> p!0) \n" {
			t.Fatal("empty section emitted a header", out.String())
		}
		if err := display(nil); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal(err)
		}
	}
	a := smtRelevancyBool(t, m, "display-prefix-a")
	astTestMust(t, s.Internalize(a, true))
	av, _ := s.BoolVar(a)
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(av, true), NewSMTBooleanAxiom()))
	_, terms := smtUFConstants(t, m, "display-prefix-", 2)
	for _, term := range terms {
		astTestMust(t, s.Internalize(term, false))
	}
	astTestMust(t, s.addEquality(s.context.enodes.get(terms[0]), s.context.enodes.get(terms[1]), NewSMTEqualityAxiom()))
	// SMT2 formatting initializes manager-owned scratch storage on first use.
	for _, display := range methods {
		astTestMust(t, display(io.Discard))
	}
	before := m.Stats()
	for _, display := range methods {
		var out bytes.Buffer
		astTestMust(t, display(&out))
		want := out.String()
		if want == "" {
			t.Fatal("fixture section empty")
		}
		for cut := 0; cut < len(want); cut++ {
			writer := &smtLRAOutputPrefix{remaining: cut}
			if err := display(writer); !errors.Is(err, io.ErrClosedPipe) || writer.text.String() != want[:cut] {
				t.Fatal(cut, err, writer.text.String(), want[:cut])
			}
		}
		if err := display(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal("short write", err)
		}
	}
	if after := m.Stats(); after != before {
		t.Fatal("display changed AST ownership", before, after)
	}
	var absent *SMTInternalizerState
	for _, display := range []func(io.Writer) error{absent.DisplayAssignment, absent.DisplayEquivalenceClasses, absent.DisplayExpressionBoolVarMap, absent.DisplayApplicationEnodeMap} {
		if err := display(io.Discard); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal("nil owner", err)
		}
	}
}

// AUDIT.md smtcore_02-F12: context::display prints m_base_lvl (user scopes)
// and the retained m_search_lvl, not the current scope level.
func TestSMTContextDisplayBaseAndSearchLevels(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	a := smtRelevancyBool(t, m, "display-level-a")
	astTestMust(t, k.Push())
	header := func() string {
		t.Helper()
		var out bytes.Buffer
		astTestMust(t, k.internalizer.DisplayContext(&out))
		lines := strings.SplitN(out.String(), "\n", 5)
		return strings.Join(lines[:4], "\n")
	}
	for i := 0; i < 3; i++ {
		astTestMust(t, k.internalizer.PushScope())
	}
	if got := header(); got != "Logical context:\nscope-lvl: 4\nbase-lvl:  1\nsearch-lvl:  1" {
		t.Fatal("decision scopes changed the base level", got)
	}
	astTestMust(t, k.internalizer.PopScopes(3))
	result, err := k.CheckAssumptions([]ast.ASTHandle{a})
	astTestMust(t, err)
	if result != lbool.True {
		t.Fatal(result)
	}
	// A satisfied check keeps the assumption scope as the search level.
	if got := header(); !strings.HasSuffix(got, "base-lvl:  1\nsearch-lvl:  2") {
		t.Fatal("search level not retained after an assumption check", got)
	}
}
