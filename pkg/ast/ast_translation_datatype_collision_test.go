package ast

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/symbol"
)

func translationEnum(t *testing.T, m *ASTManager, constructors ...string) ASTHandle {
	t.Helper()
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	definition := DatatypeDefinition{Name: astTestSymbol(t, m, "TranslationEnum")}
	for _, name := range constructors {
		definition.Constructors = append(definition.Constructors, DatatypeConstructorDefinition{
			Name: astTestSymbol(t, m, name), Recognizer: astTestSymbol(t, m, "is-"+name),
		})
	}
	sorts, err := m.MkDatatypes([]DatatypeDefinition{definition})
	astTestMust(t, err)
	return sorts[0]
}

func TestASTTranslationRejectsConflictingDatatypeWithoutReplacingTarget(t *testing.T) {
	for _, row := range []struct {
		name   string
		source []string
	}{
		{"different-count", []string{"source-a", "source-b", "source-c"}},
		{"different-names", []string{"source-a", "source-b"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			from, to := astTranslationManagers(t)
			translationEnum(t, from, row.source...)
			target := translationEnum(t, to, "target-a", "target-b")
			before, err := to.DatatypeConstructors(target)
			astTestMust(t, err)
			translation, err := NewASTTranslation(from, to, true)
			if translation != nil {
				astTestMust(t, translation.Close())
			}
			if !errors.Is(err, ErrASTDomain) {
				t.Fatalf("expected incompatible datatype error, got %v", err)
			}
			after, err := to.DatatypeConstructors(target)
			astTestMust(t, err)
			if len(after) != len(before) || after[0] != before[0] || after[1] != before[1] {
				t.Fatal("incompatible translation replaced target datatype")
			}
		})
	}
}

func TestASTTranslationReusesIdenticalDatatypeDefinitions(t *testing.T) {
	from, to := astTranslationManagers(t)
	source := translationEnum(t, from, "same-a", "same-b")
	target := translationEnum(t, to, "same-a", "same-b")
	before, err := to.DatatypeConstructors(target)
	astTestMust(t, err)
	for i := 0; i < 2; i++ {
		translation, e := NewASTTranslation(from, to, true)
		astTestMust(t, e)
		got, e := translation.Translate(source)
		astTestMust(t, e)
		if got != target {
			t.Fatal("identical datatype did not use target sort")
		}
		astTestMust(t, translation.Close())
	}
	after, err := to.DatatypeConstructors(target)
	astTestMust(t, err)
	if len(after) != len(before) || after[0] != before[0] || after[1] != before[1] {
		t.Fatal("identical translation changed target constructors")
	}
}

func TestASTTranslationReusesIdenticalRecursiveDatatype(t *testing.T) {
	from, to := astTranslationManagers(t)
	build := func(m *ASTManager) ASTHandle {
		_, err := m.RegisterDatatypePlugin()
		astTestMust(t, err)
		boolean, err := m.MkBoolSort()
		astTestMust(t, err)
		name := func(s string) symbol.Symbol { return astTestSymbol(t, m, s) }
		sorts, err := m.MkDatatypes([]DatatypeDefinition{{Name: name("TranslationTree"), Constructors: []DatatypeConstructorDefinition{
			{Name: name("leaf"), Recognizer: name("is-leaf"), Accessors: []DatatypeAccessorDefinition{{Name: name("value"), Range: boolean}}},
			{Name: name("branch"), Recognizer: name("is-branch"), Accessors: []DatatypeAccessorDefinition{{Name: name("child"), Recursive: true}}},
		}}})
		astTestMust(t, err)
		return sorts[0]
	}
	source, target := build(from), build(to)
	for i := 0; i < 2; i++ {
		translation, err := NewASTTranslation(from, to, true)
		astTestMust(t, err)
		got, err := translation.Translate(source)
		astTestMust(t, err)
		if got != target {
			t.Fatal("recursive target datatype was not reused")
		}
		astTestMust(t, translation.Close())
	}
}

func TestASTTranslationRejectsConflictingRecursiveField(t *testing.T) {
	from, to := astTranslationManagers(t)
	build := func(m *ASTManager, field string) ASTHandle {
		_, err := m.RegisterDatatypePlugin()
		astTestMust(t, err)
		name := func(s string) symbol.Symbol { return astTestSymbol(t, m, s) }
		sorts, err := m.MkDatatypes([]DatatypeDefinition{{Name: name("FieldTree"), Constructors: []DatatypeConstructorDefinition{
			{Name: name("leaf"), Recognizer: name("is-leaf")},
			{Name: name("branch"), Recognizer: name("is-branch"), Accessors: []DatatypeAccessorDefinition{{Name: name(field), Recursive: true}}},
		}}})
		astTestMust(t, err)
		return sorts[0]
	}
	build(from, "source-child")
	target := build(to, "target-child")
	constructors, err := to.DatatypeConstructors(target)
	astTestMust(t, err)
	before, err := to.DatatypeConstructorAccessors(constructors[1])
	astTestMust(t, err)
	translation, err := NewASTTranslation(from, to, true)
	if translation != nil {
		astTestMust(t, translation.Close())
	}
	if !errors.Is(err, ErrASTDomain) {
		t.Fatalf("expected conflicting recursive field error, got %v", err)
	}
	after, err := to.DatatypeConstructorAccessors(constructors[1])
	astTestMust(t, err)
	if len(after) != 1 || after[0] != before[0] {
		t.Fatal("recursive target definition changed")
	}
}
