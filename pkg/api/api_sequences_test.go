package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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

func TestSMTContextSequenceAndRegexConstructorCoverage(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	stringSort := f.pin(f.c.StringSort())
	booleanSort := f.pin(f.m.MkBoolSort())
	truth := f.pin(f.m.MkTrue())
	booleanSequenceSort := f.pin(f.c.SequenceSort(booleanSort))
	integerSort := f.pin(f.c.IntegerSort())
	zero := f.pin(f.c.IntNumeral(0, integerSort))
	one := f.pin(f.c.IntNumeral(1, integerSort))
	a := f.pin(f.c.String("a"))
	b := f.pin(f.c.String("b"))
	ab := f.pin(f.c.SequenceConcat(a, b))

	empty := f.pin(f.c.SequenceEmpty(stringSort))
	unit := f.pin(f.c.SequenceUnit(truth))
	_ = f.pin(f.c.SequencePrefix(a, ab))
	_ = f.pin(f.c.SequenceSuffix(b, ab))
	_ = f.pin(f.c.SequenceContains(ab, a))
	_ = f.pin(f.c.SequenceExtract(ab, zero, one))
	_ = f.pin(f.c.SequenceReplace(ab, a, b))
	_ = f.pin(f.c.SequenceAt(ab, zero))
	_ = f.pin(f.c.SequenceLength(ab))
	_ = f.pin(f.c.SequenceIndex(ab, b, zero))
	_ = f.pin(f.c.IntToString(one))
	_ = f.pin(f.c.StringToInt(a))

	if got, yes, err := f.c.StringValue(truth); err != nil || yes || got != "" {
		t.Fatalf("non-literal string value = %q, %v, %v", got, yes, err)
	}
	if got, yes, err := f.c.StringValue(empty); err != nil || !yes || got != "" {
		t.Fatalf("empty string value = %q, %v, %v", got, yes, err)
	}

	reA := f.pin(f.c.SequenceToRegex(a))
	reB := f.pin(f.c.SequenceToRegex(b))
	_ = f.pin(f.c.RegexLoop(reA, 2))
	_ = f.pin(f.c.RegexPlus(reA))
	_ = f.pin(f.c.RegexOption(reA))
	_ = f.pin(f.c.RegexConcat(reA, reB))
	_ = f.pin(f.c.RegexUnion(reA, reB))
	_ = f.pin(f.c.RegexIntersect(reA, reB))

	for name, value := range map[string]ast.ASTHandle{
		"sequence empty": empty,
	} {
		if sort := f.pin(f.m.GetSort(value)); sort != stringSort {
			t.Fatalf("%s sort = %v, want %v", name, sort, stringSort)
		}
	}
	if sort := f.pin(f.m.GetSort(unit)); sort != booleanSequenceSort {
		t.Fatalf("sequence unit sort = %v, want %v", sort, booleanSequenceSort)
	}
}
