package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTContextSequenceAndRegexAPI(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	sequenceSort := f.pin(f.c.SequenceSort(boolean))
	if yes, err := f.c.IsSequenceSort(sequenceSort); err != nil || !yes {
		t.Fatal("sequence sort", yes, err)
	}
	regexSort := f.pin(f.c.RegexSort(sequenceSort))
	if yes, err := f.c.IsRegexSort(regexSort); err != nil || !yes {
		t.Fatal("regex sort", yes, err)
	}
	stringSort := f.pin(f.c.StringSort())
	if yes, err := f.c.IsStringSort(stringSort); err != nil || !yes {
		t.Fatal("string sort", yes, err)
	}
	a := f.pin(f.c.String("a"))
	b := f.pin(f.c.String("b"))
	ab := f.pin(f.c.SequenceConcat(a, b))
	if kind, err := f.m.GetDeclKind(ab); err != nil || kind != ast.ASTSequenceOpConcat {
		t.Fatal("concat", kind, err)
	}
	if got, yes, err := f.c.StringValue(a); err != nil || !yes || got != "a" {
		t.Fatal("string value", got, yes, err)
	}
	reA := f.pin(f.c.SequenceToRegex(a))
	reB := f.pin(f.c.SequenceToRegex(b))
	union := f.pin(f.c.RegexUnion(reA, reB))
	star := f.pin(f.c.RegexStar(union))
	member := f.pin(f.c.SequenceInRegex(ab, star))
	if sort := f.pin(f.m.GetSort(member)); sort != boolean {
		t.Fatal("membership sort", sort, boolean)
	}
	_ = f.pin(f.c.RegexLoop(reA, 1, 3))
	_ = f.pin(f.c.RegexComplement(reA))
	_ = f.pin(f.c.RegexRange(a, b))
	_ = f.pin(f.c.RegexEmpty(regexSort))
	_ = f.pin(f.c.RegexFull(regexSort))
}

func TestSMTContextSequenceAPIErrors(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if _, err := f.c.SequenceConcat(); err != ast.ErrASTDomain {
		t.Fatal("empty concat", err)
	}
	if _, err := f.c.RegexUnion(); err != ast.ErrASTDomain {
		t.Fatal("empty union", err)
	}
}
