package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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
	want = fmt.Sprintf("expression -> bool_var:\n(#%d -> p!%d) (#%d -> p!%d) \n", bid, bv, aid, av)
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	astTestMust(t, s.PopScopes(1))
	out.Reset()
	astTestMust(t, s.DisplayExpressionBoolVarMap(&out))
	want = fmt.Sprintf("expression -> bool_var:\n(#%d -> p!%d) \n", bid, bv)
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
		// A fresh context already maps context::init's true and false enodes.
		if out.Len() != 0 && out.String() != "expression -> enode:\n(#1 -> e!0) (#2 -> e!1) \n" {
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
