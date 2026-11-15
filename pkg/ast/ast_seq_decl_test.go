package ast

import "testing"

func TestASTSequenceGenericFamilyDispatch(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.prepareSequencePlugin())
	family := m.sequencePlugin.family
	truth, err := m.MkTrue()
	astTestMust(t, err)
	unit, err := m.MkFamilyApp(family, ASTSequenceOpUnit, nil, []ASTHandle{truth})
	astTestMust(t, err)
	first, err := m.MkString("a")
	astTestMust(t, err)
	second, err := m.MkString("b")
	astTestMust(t, err)
	stringSort, err := m.GetSort(first)
	astTestMust(t, err)
	unitSort, err := m.GetSort(unit)
	astTestMust(t, err)
	element, matched, err := m.SequenceSortElement(unitSort)
	astTestMust(t, err)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	if !matched || element != boolSort {
		t.Fatalf("generic unit sort: matched=%v element=%v", matched, element)
	}
	for _, tc := range []struct {
		kind int32
		args []ASTHandle
		name string
	}{
		{3, []ASTHandle{first, second}, "str.prefixof"},
		{ASTSequenceOpConcat, []ASTHandle{first, second}, "str.++"},
		{11, []ASTHandle{first}, "str.to.re"},
		{30, []ASTHandle{first, second}, "str.++"},
		{36, []ASTHandle{first}, "str.to.re"},
	} {
		h, e := m.MkFamilyApp(family, tc.kind, nil, tc.args)
		astTestMust(t, e)
		ref, e := m.internal(h)
		astTestMust(t, e)
		name, e := m.Symbols.String(m.declAt(m.appAt(ref.slot).decl.slot).name)
		astTestMust(t, e)
		if name != tc.name {
			t.Fatalf("generic family kind %d name: %q", tc.kind, name)
		}
	}
	concat3, err := m.MkFamilyApp(family, ASTSequenceOpConcat, nil, []ASTHandle{first, second, first})
	astTestMust(t, err)
	if wellSorted, e := m.IsWellSorted(concat3, nil); e != nil || !wellSorted {
		t.Fatalf("right-associated three-argument concat: sorted=%v error=%v", wellSorted, e)
	}
	regex, err := m.MkFamilyApp(family, 11, nil, []ASTHandle{first})
	astTestMust(t, err)
	if _, err = m.MkFamilyApp(family, 13, nil, []ASTHandle{regex}); err != nil {
		t.Fatalf("generic regex plus: %v", err)
	}
	lower, err := m.NewParameterInt(2)
	astTestMust(t, err)
	upper, err := m.NewParameterInt(5)
	astTestMust(t, err)
	loop, err := m.MkFamilyApp(family, 20, []ASTParameter{lower, upper}, []ASTHandle{regex})
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&lower))
	astTestMust(t, m.DelParameter(&upper))
	if kind, e := m.GetDeclKind(loop); e != nil || kind != 20 {
		t.Fatalf("generic regex loop: kind=%d error=%v", kind, e)
	}
	integerSort := m.external(m.arithmeticPlugin.fields[1])
	for _, domain := range [][]ASTHandle{{m.external(m.sequencePlugin.regex), integerSort}, {m.external(m.sequencePlugin.regex), integerSort, integerSort}} {
		decl, e := m.MkFamilyFuncDecl(family, 20, nil, domain, ASTHandle{})
		astTestMust(t, e)
		if kind, e := m.GetDeclKind(decl); e != nil || kind != 20 {
			t.Fatalf("expression-argument regex loop: kind=%d error=%v", kind, e)
		}
	}
	indexDecl, err := m.MkFamilyFuncDecl(family, 10, nil, []ASTHandle{stringSort, stringSort}, ASTHandle{})
	astTestMust(t, err)
	indexRange, err := m.GetRange(indexDecl)
	astTestMust(t, err)
	if indexRange != integerSort {
		t.Fatalf("two-argument index range: %v", indexRange)
	}
	for _, tc := range []struct {
		kind int32
		name string
	}{
		{40, "re.nostr"},
		{41, "re.allchar"},
	} {
		h, e := m.MkFamilyApp(family, tc.kind, nil, nil)
		astTestMust(t, e)
		ref, e := m.internal(h)
		astTestMust(t, e)
		decl := m.declAt(m.appAt(ref.slot).decl.slot)
		name, e := m.Symbols.String(decl.name)
		astTestMust(t, e)
		if name != tc.name {
			t.Fatalf("internal regex constant %d: name=%q", tc.kind, name)
		}
		info, e := m.funcInfo(decl.info)
		astTestMust(t, e)
		if info.kind != canonicalSequenceKind(tc.kind) {
			t.Fatalf("internal regex constant %d published kind %d", tc.kind, info.kind)
		}
	}

	emptyDecl, err := m.MkFamilyFuncDecl(family, ASTSequenceOpEmpty, nil, nil, stringSort)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(emptyDecl); e != nil || kind != ASTSequenceOpStringConst {
		t.Fatalf("canonical generic empty string declaration: kind=%d error=%v", kind, e)
	}
	emptyString, err := m.MkApp(emptyDecl, nil)
	astTestMust(t, err)
	if yes, e := m.IsSequenceEmpty(emptyString); e != nil || !yes {
		t.Fatalf("canonical generic empty string: %v %v", yes, e)
	}

	name := astTestSymbol(t, m, "generic-skolem")
	nameParameter, err := m.NewParameterSymbol(name)
	astTestMust(t, err)
	skolemDecl, err := m.MkFamilyFuncDecl(family, 42, []ASTParameter{nameParameter}, []ASTHandle{boolSort}, stringSort)
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&nameParameter))
	skolem, err := m.MkApp(skolemDecl, []ASTHandle{truth})
	astTestMust(t, err)
	if yes, e := m.IsSequenceSkolem(skolem); e != nil || !yes {
		t.Fatalf("generic sequence skolem: %v %v", yes, e)
	}
}

// AUDIT.md ast_06-F1: seq_decl_plugin::mk_func_decl builds re.all always,
// and re.allchar / re.empty whenever the range is not m_re, without matching
// the signature.
func TestASTSequenceRegexConstantsUncheckedRange(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.prepareSequencePlugin())
	family := m.sequencePlugin.family
	intSort := m.external(m.arithmeticPlugin.fields[1])
	element, err := m.NewParameterAST(intSort)
	astTestMust(t, err)
	reInt, err := m.MkFamilySort(family, 1, []ASTParameter{element})
	astTestMust(t, err)
	for _, tc := range []struct {
		kind int32
		name string
	}{{23, "re.all"}, {24, "re.allchar"}, {22, "re.empty"}} {
		decl, e := m.MkFamilyFuncDecl(family, tc.kind, nil, nil, reInt)
		if e != nil {
			t.Fatalf("kind %d with range (RegEx Int): %v", tc.kind, e)
		}
		r, e := m.GetRange(decl)
		astTestMust(t, e)
		if r != reInt {
			t.Fatalf("kind %d range: %v want %v", tc.kind, r, reInt)
		}
		ref, e := m.internal(decl)
		astTestMust(t, e)
		name, e := m.Symbols.String(m.declAt(ref.slot).name)
		astTestMust(t, e)
		if name != tc.name {
			t.Fatalf("kind %d name: %q want %q", tc.kind, name, tc.name)
		}
		app, e := m.MkFamilyAppRange(family, tc.kind, nil, nil, reInt)
		astTestMust(t, e)
		if s, e := m.GetSort(app); e != nil || s != reInt {
			t.Fatalf("kind %d app sort: %v %v", tc.kind, s, e)
		}
	}
}

// AUDIT.md ast_06-F2: seq_decl_plugin::mk_func_decl ignores the parameters
// of every kind except re.loop, string constants and skolems.
func TestASTSequenceIgnoresIndexParameters(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.prepareSequencePlugin())
	family := m.sequencePlugin.family
	str, err := m.MkString("a")
	astTestMust(t, err)
	stringSort, err := m.GetSort(str)
	astTestMust(t, err)
	p3, err := m.NewParameterInt(3)
	astTestMust(t, err)
	plain, err := m.MkFamilyFuncDecl(family, 31, nil, []ASTHandle{stringSort}, ASTHandle{})
	astTestMust(t, err)
	indexed, err := m.MkFamilyFuncDecl(family, 31, []ASTParameter{p3}, []ASTHandle{stringSort}, ASTHandle{})
	if err != nil {
		t.Fatalf("indexed str.len: %v", err)
	}
	if indexed != plain {
		t.Fatalf("indexed str.len decl %v differs from %v", indexed, plain)
	}
	elementSort := m.external(m.arithmeticPlugin.fields[1])
	element, err := m.NewParameterAST(elementSort)
	astTestMust(t, err)
	seqInt, err := m.MkFamilySort(family, 0, []ASTParameter{element})
	astTestMust(t, err)
	emptyPlain, err := m.MkFamilyFuncDecl(family, ASTSequenceOpEmpty, nil, nil, seqInt)
	astTestMust(t, err)
	emptyIndexed, err := m.MkFamilyFuncDecl(family, ASTSequenceOpEmpty, []ASTParameter{p3}, nil, seqInt)
	if err != nil || emptyIndexed != emptyPlain {
		t.Fatalf("indexed seq.empty: %v %v %v", emptyIndexed, emptyPlain, err)
	}
}
