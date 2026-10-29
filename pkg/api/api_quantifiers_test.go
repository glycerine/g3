package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"strings"
	"testing"
)

func TestSMTContextQuantifierAndPatternAPI(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	variable := f.pin(f.m.MkVar(0, boolean))
	triggerTerm := f.pin(f.m.MkNot(variable))
	pattern := f.pin(f.c.Pattern(triggerTerm))
	quantifier := f.pin(f.c.Quantifier(QuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []string{"x"}, Body: triggerTerm, Weight: 7, QID: "qid", SkolemID: "skid", Patterns: []ast.ASTHandle{pattern}}))
	if yes, err := f.c.IsQuantifier(quantifier); err != nil || !yes {
		t.Fatal("quantifier", yes, err)
	}
	if yes, err := f.c.IsForall(quantifier); err != nil || !yes {
		t.Fatal("forall", yes, err)
	}
	if weight, err := f.c.QuantifierWeight(quantifier); err != nil || weight != 7 {
		t.Fatal("weight", weight, err)
	}
	if count, err := f.c.QuantifierBoundCount(quantifier); err != nil || count != 1 {
		t.Fatal("binders", count, err)
	}
	if name, err := f.c.QuantifierBoundName(quantifier, 0); err != nil || name != "x" {
		t.Fatal("name", name, err)
	}
	if sort := f.pin(f.c.QuantifierBoundSort(quantifier, 0)); sort != boolean {
		t.Fatal("sort", sort, boolean)
	}
	if body := f.pin(f.c.QuantifierBody(quantifier)); body != triggerTerm {
		t.Fatal("body", body, triggerTerm)
	}
	if count, err := f.c.QuantifierPatternCount(quantifier); err != nil || count != 1 {
		t.Fatal("patterns", count, err)
	}
	if count, err := f.c.PatternTermCount(pattern); err != nil || count != 1 {
		t.Fatal("terms", count, err)
	}
	if term := f.pin(f.c.PatternTerm(pattern, 0)); term != triggerTerm {
		t.Fatal("term", term, triggerTerm)
	}
	if text, err := f.c.PatternString(pattern); err != nil || !strings.Contains(text, "not") {
		t.Fatal("pattern string", text, err)
	}
	if got, err := f.c.Forall(nil, nil, triggerTerm); err != nil || got != triggerTerm {
		t.Fatal("empty binder", got, err)
	}
}

func TestSMTContextQuantifierAPIErrors(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	body := f.pin(f.m.MkTrue())
	if _, err := f.c.Quantifier(QuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Body: body}); err != ast.ErrASTDomain {
		t.Fatal("mismatched binders", err)
	}
	if _, err := f.c.Pattern(); err != ast.ErrASTDomain {
		t.Fatal("empty pattern", err)
	}
	text := f.pin(f.c.String("not-bool"))
	if _, err := f.c.Forall(nil, nil, text); err != ast.ErrASTDomain {
		t.Fatal("non-boolean body", err)
	}
}
