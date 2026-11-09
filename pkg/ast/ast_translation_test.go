package ast

// Source: src/ast/ast_translation.cpp; src/ast/ast_translation.h.

import (
	"bytes"
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTTranslationInheritsDatatypeDefinitions(t *testing.T) {
	from, to := astTranslationManagers(t)
	_, err := from.RegisterDatatypePlugin()
	astTestMust(t, err)
	boolean, err := from.MkBoolSort()
	astTestMust(t, err)
	sorts, err := from.MkDatatypes([]DatatypeDefinition{
		{Name: astTestSymbol(t, from, "Tree"), Constructors: []DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, from, "leaf"), Recognizer: astTestSymbol(t, from, "is-leaf"), Accessors: []DatatypeAccessorDefinition{{Name: astTestSymbol(t, from, "leaf-value"), Range: boolean}}},
			{Name: astTestSymbol(t, from, "branch"), Recognizer: astTestSymbol(t, from, "is-branch"), Accessors: []DatatypeAccessorDefinition{{Name: astTestSymbol(t, from, "left"), Recursive: true}, {Name: astTestSymbol(t, from, "right"), Recursive: true}}},
		}},
	})
	astTestMust(t, err)
	translation, err := NewASTTranslation(from, to, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, translation.Close()) }()
	translated, err := translation.Translate(sorts[0])
	astTestMust(t, err)
	constructors, err := to.DatatypeConstructors(translated)
	astTestMust(t, err)
	if len(constructors) != 2 {
		t.Fatal("translated datatype constructors", len(constructors))
	}
	accessors, err := to.DatatypeConstructorAccessors(constructors[1])
	astTestMust(t, err)
	if len(accessors) != 2 {
		t.Fatal("translated recursive accessors", len(accessors))
	}
	for _, accessor := range accessors {
		if rangeSort, e := to.GetRange(accessor); e != nil || rangeSort != translated {
			t.Fatal("translated recursive range", rangeSort, translated, e)
		}
	}
	var display bytes.Buffer
	astTestMust(t, to.DisplayDatatype(translated, &display))
	if want := "Tree where\nTree =\n  leaf :: (leaf-value: Bool) \n  branch :: (left: Tree) (right: Tree) \n"; display.String() != want {
		t.Fatalf("translated datatype display\nwant: %q\n got: %q", want, display.String())
	}
}

func TestASTTranslationIterativeQuantifierAndPersistentSharedCache(t *testing.T) {
	from, to := astTranslationManagers(t)
	boolSort, err := from.MkBoolSort()
	astTestMust(t, err)
	variable, err := from.MkVar(0, boolSort)
	astTestMust(t, err)
	decl, err := from.MkFuncDecl(astTestSymbol(t, from, "predicate"), []ASTHandle{boolSort}, boolSort, ASTFuncDeclInfo{})
	astTestMust(t, err)
	app, err := from.MkApp(decl, []ASTHandle{variable})
	astTestMust(t, err)
	pattern, err := from.MkPattern([]ASTHandle{app})
	astTestMust(t, err)
	quantifier, err := from.MkQuantifier(ASTQuantifierSpec{
		Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{astTestSymbol(t, from, "x")},
		Body: app, Weight: 7, QID: astTestSymbol(t, from, "qid"), SkolemID: astTestSymbol(t, from, "skid"), Patterns: []ASTHandle{pattern},
	})
	astTestMust(t, err)
	astTestPin(t, from, quantifier)

	translation, err := NewASTTranslation(from, to, true)
	astTestMust(t, err)
	translated, err := translation.Translate(quantifier)
	astTestMust(t, err)
	targetRoot, err := to.NewRef(translated)
	astTestMust(t, err)
	defer func() { astTestMust(t, to.DelRef(&targetRoot)) }()

	if kind, e := to.GetKind(translated); e != nil || kind != ASTQuantifier {
		t.Fatal("translated quantifier", kind, e)
	}
	body, err := to.GetQuantifierBody(translated)
	astTestMust(t, err)
	targetPattern, err := to.GetPattern(translated, 0)
	astTestMust(t, err)
	patternApp, err := to.GetArg(targetPattern, 0)
	astTestMust(t, err)
	if body != patternApp {
		t.Fatal("shared application identity was not preserved")
	}
	if found, _ := translation.cacheContains(app); !found {
		t.Fatal("source shared node was not retained in the persistent cache")
	}
	again, err := translation.Translate(app)
	astTestMust(t, err)
	if again != body {
		t.Fatal("persistent cache miss", again, body)
	}
	if translation.From() != from || translation.To() != to {
		t.Fatal("manager accessors")
	}
	astTestMust(t, translation.Close())
	if _, err = to.GetKind(translated); err != nil {
		t.Fatal("caller pin did not preserve translated root", err)
	}
}

func TestASTTranslationCopiesEveryOrdinaryParameterKind(t *testing.T) {
	from, to := astTranslationManagers(t)
	boolSort, err := from.MkBoolSort()
	astTestMust(t, err)
	integer, err := from.NewParameterInt(-17)
	astTestMust(t, err)
	astParameter, err := from.NewParameterAST(boolSort)
	astTestMust(t, err)
	symbolParameter, err := from.NewParameterString("parameter-name")
	astTestMust(t, err)
	rationalValue, err := from.numerals.NewValue()
	astTestMust(t, err)
	astTestMust(t, from.numerals.SetString(&rationalValue, "123456789012345678901/37"))
	rationalParameter, err := from.NewParameterRational(&rationalValue)
	astTestMust(t, err)
	astTestMust(t, from.numerals.Del(&rationalValue))
	doubleParameter, err := from.NewParameterDouble(1.25)
	astTestMust(t, err)
	parameters := []ASTParameter{integer, astParameter, symbolParameter, rationalParameter, doubleParameter}
	defer func() {
		for i := len(parameters) - 1; i >= 0; i-- {
			astTestMust(t, from.DelParameter(&parameters[i]))
		}
	}()
	info, err := from.NewSortInfo(NullFamilyID, 42, parameters, FiniteSortSize(19), true)
	astTestMust(t, err)
	sort, err := from.MkSort(astTestSymbol(t, from, "parameterized-sort"), info)
	astTestMust(t, err)
	astTestMust(t, from.DelSortInfo(&info))
	astTestPin(t, from, sort)

	translation, err := NewASTTranslation(from, to, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, translation.Close()) }()
	translated, err := translation.Translate(sort)
	astTestMust(t, err)
	targetRoot, err := to.NewRef(translated)
	astTestMust(t, err)
	defer func() { astTestMust(t, to.DelRef(&targetRoot)) }()

	to.mu.Lock()
	ref, err := to.requireKind(translated, ASTSort)
	if err != nil {
		to.mu.Unlock()
		t.Fatal(err)
	}
	record, err := to.sortInfo(to.sortAt(ref.slot).info)
	if err != nil {
		to.mu.Unlock()
		t.Fatal(err)
	}
	if record.family != NullFamilyID || record.kind != 42 || !record.private || record.count != 5 {
		to.mu.Unlock()
		t.Fatal("translated sort metadata", *record)
	}
	got := make([]astParameterRecord, record.count)
	for i := range got {
		p, e := to.infoParameter(record, uint32(i))
		if e == nil {
			var value *astParameterRecord
			value, e = to.parameter(p)
			if e == nil {
				got[i] = *value
			}
		}
		if e != nil {
			to.mu.Unlock()
			t.Fatal(e)
		}
	}
	to.mu.Unlock()
	if got[0].Kind != ASTParameterInt || got[0].integer != -17 || got[1].Kind != ASTParameterAST || to.external(got[1].ast) != mustBoolSort(t, to) || got[2].Kind != ASTParameterSymbol || got[4].Kind != ASTParameterDouble || got[4].double != 1.25 {
		t.Fatal("ordinary parameter translation", got)
	}
	text, err := to.Symbols.String(got[2].symbol)
	astTestMust(t, err)
	if text != "parameter-name" {
		t.Fatal(text)
	}
	rational, err := to.numerals.String(&got[3].Rational)
	astTestMust(t, err)
	if rational != "123456789012345678901/37" {
		t.Fatal(rational)
	}
}

func mustBoolSort(t *testing.T, m *ASTManager) ASTHandle {
	t.Helper()
	h, err := m.MkBoolSort()
	astTestMust(t, err)
	return h
}

func TestASTTranslationUsesAnExplicitStackForDeepTerms(t *testing.T) {
	from, to := astTranslationManagers(t)
	term, err := from.MkTrue()
	astTestMust(t, err)
	for i := 0; i < 4096; i++ {
		term, err = from.MkNot(term)
		astTestMust(t, err)
	}
	astTestPin(t, from, term)
	translation, err := NewASTTranslation(from, to, true)
	astTestMust(t, err)
	translated, err := translation.Translate(term)
	astTestMust(t, err)
	targetRoot, err := to.NewRef(translated)
	astTestMust(t, err)
	astTestMust(t, translation.Close())
	defer func() { astTestMust(t, to.DelRef(&targetRoot)) }()
	depth, err := to.GetDepth(translated)
	astTestMust(t, err)
	if depth != 4097 {
		t.Fatal("deep translated term", depth)
	}
}

func TestASTTranslationCopiesFloatingPointExternalParameters(t *testing.T) {
	from, to := astTranslationManagers(t)
	_, err := from.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = from.RegisterBitVectorPlugin()
	astTestMust(t, err)
	_, err = from.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	p := from.floatingPointPlugin
	significand := unumeral.NewInt(19)
	var value unumeral.ArbitraryFloat
	astTestMust(t, p.numeric.SetComponents(&value, 8, 24, true, 3, &significand))
	ref, err := p.mkNumeral(&value, nil)
	astTestMust(t, err)
	astTestMust(t, p.numeric.Del(&value))
	source := from.external(ref)
	astTestPin(t, from, source)

	translation, err := NewASTTranslation(from, to, true)
	astTestMust(t, err)
	translated, err := translation.Translate(source)
	astTestMust(t, err)
	targetRoot, err := to.NewRef(translated)
	astTestMust(t, err)
	astTestMust(t, translation.Close())
	defer func() { astTestMust(t, to.DelRef(&targetRoot)) }()
	sourceText, sourceMatch, err := from.FPANumeralString(source)
	astTestMust(t, err)
	targetText, targetMatch, err := to.FPANumeralString(translated)
	astTestMust(t, err)
	if !sourceMatch || !targetMatch || sourceText != targetText {
		t.Fatal("floating-point external translation", sourceText, sourceMatch, targetText, targetMatch)
	}
}

func TestASTTranslationLinearizesAndRebuildsDependencies(t *testing.T) {
	from, to := astTranslationManagers(t)
	truth, err := from.MkTrue()
	astTestMust(t, err)
	falsehood, err := from.MkFalse()
	astTestMust(t, err)
	progress, err := from.DependencyJoinValues([]ASTHandle{truth, falsehood, truth})
	astTestMust(t, err)
	source := progress.Completed
	astTestMust(t, from.DependencyInc(source))
	defer func() { astTestMust(t, from.DependencyDec(source)) }()

	translation, err := NewASTTranslation(from, to, true)
	astTestMust(t, err)
	target, err := translation.TranslateDependency(source)
	astTestMust(t, err)
	astTestMust(t, to.DependencyInc(target))
	defer func() { astTestMust(t, to.DependencyDec(target)) }()
	astTestMust(t, translation.Close())
	leaves, err := to.DependencyLinearize(target, nil)
	astTestMust(t, err)
	if len(leaves) != 2 {
		t.Fatal("translated dependency leaves", leaves)
	}
	wantTrue, err := to.MkTrue()
	astTestMust(t, err)
	wantFalse, err := to.MkFalse()
	astTestMust(t, err)
	if leaves[0] != wantTrue || leaves[1] != wantFalse {
		t.Fatal("translated dependency order", leaves, wantTrue, wantFalse)
	}
}

func TestTranslateASTNonpersistentHelperLeavesSharedRootLive(t *testing.T) {
	from, to := astTranslationManagers(t)
	boolSort, err := from.MkBoolSort()
	astTestMust(t, err)
	root, err := from.MkConst(astTestSymbol(t, from, "shared-root"), boolSort)
	astTestMust(t, err)
	first, err := from.NewRef(root)
	astTestMust(t, err)
	defer func() { astTestMust(t, from.DelRef(&first)) }()
	second, err := from.NewRef(root)
	astTestMust(t, err)
	defer func() { astTestMust(t, from.DelRef(&second)) }()
	translated, err := TranslateAST(root, from, to)
	astTestMust(t, err)
	if kind, e := to.GetKind(translated); e != nil || kind != ASTApp {
		t.Fatal("nonpersistent helper returned an expired root", kind, e)
	}
}
