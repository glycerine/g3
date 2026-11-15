package ast

import (
	"strings"
	"testing"
)

func TestLegacySMTDatatypeCollectorVisitsFieldsAndOrdersMutualDefinitions(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	u, err := m.MkUninterpretedSort(astTestSymbol(t, m, "LegacyField"), nil)
	astTestMust(t, err)
	sorts, err := m.MkDatatypes([]DatatypeDefinition{
		{Name: astTestSymbol(t, m, "LegacyA"), Constructors: []DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "legacy-nil"), Recognizer: astTestSymbol(t, m, "legacy-is-nil")},
			{Name: astTestSymbol(t, m, "legacy-a"), Recognizer: astTestSymbol(t, m, "legacy-is-a"), Accessors: []DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "legacy-field"), Range: u}, {Name: astTestSymbol(t, m, "legacy-b-field"), Recursive: true, RecursiveIndex: 1}}},
		}},
		{Name: astTestSymbol(t, m, "LegacyB"), Constructors: []DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "legacy-b"), Recognizer: astTestSymbol(t, m, "legacy-is-b"), Accessors: []DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "legacy-a-field"), Recursive: true, RecursiveIndex: 0}}},
		}},
	})
	astTestMust(t, err)
	astTestPin(t, m, sorts[0])
	astTestPin(t, m, sorts[1])
	c := newASTDeclCollector(m, false)
	astTestMust(t, c.visit(sorts[0]))
	astTestMust(t, c.orderDeps())
	if len(c.sorts) != 3 || c.sorts[0] != u || len(c.decls) != 0 {
		t.Fatal("datatype field/sibling dependencies were omitted", c.sorts, c.decls)
	}
	p, err := NewLegacySMTPrinter(m)
	astTestMust(t, err)
	var out strings.Builder
	astTestMust(t, p.DisplaySortDeclaration(&out, sorts[0]))
	text := out.String()
	if strings.Count(text, "(declare-datatypes") != 1 || !strings.Contains(text, "(LegacyA 0)") || !strings.Contains(text, "(LegacyB 0)") || !strings.Contains(text, "(legacy-field LegacyField)") {
		t.Fatal("mutual declaration block", text)
	}
	constructors, err := m.DatatypeConstructors(sorts[0])
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "legacy-value"), sorts[0])
	astTestMust(t, err)
	parameter, err := m.NewParameterAST(constructors[1])
	astTestMust(t, err)
	family, err := m.DatatypeFamilyID()
	astTestMust(t, err)
	recognizer, err := m.MkFamilyApp(family, ASTDatatypeIs, []ASTParameter{parameter}, []ASTHandle{x})
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&parameter))
	astTestPin(t, m, recognizer)
	out.Reset()
	astTestMust(t, p.DisplayExpr(&out, recognizer, nil))
	if out.String() != "((_ is legacy-a ) legacy-value)" {
		t.Fatal("constructor metadata leaked into recognizer reference", out.String())
	}
}

func TestLegacySMTParametricDatatypeBenchmarkDeclaresGenericBlockOnce(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	formal, err := m.MkUninterpretedSort(astTestSymbol(t, m, "T"), nil)
	astTestMust(t, err)
	name := astTestSymbol(t, m, "LegacyBox")
	_, err = m.MkDatatypes([]DatatypeDefinition{{Name: name, Parameters: []ASTHandle{formal}, Constructors: []DatatypeConstructorDefinition{{Name: astTestSymbol(t, m, "legacy-box"), Recognizer: astTestSymbol(t, m, "legacy-is-box"), Accessors: []DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "legacy-unbox"), Range: formal}}}}}})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	u, err := m.MkUninterpretedSort(astTestSymbol(t, m, "LegacyU"), nil)
	astTestMust(t, err)
	instantiate := func(argument ASTHandle) ASTHandle {
		p0, err := m.NewParameterSymbol(name)
		astTestMust(t, err)
		p1, err := m.NewParameterAST(argument)
		astTestMust(t, err)
		sort, err := m.MkFamilySort(family, ASTDatatypeSort, []ASTParameter{p0, p1})
		astTestMust(t, err)
		astTestMust(t, m.DelParameter(&p1))
		astTestMust(t, m.DelParameter(&p0))
		astTestPin(t, m, sort)
		return sort
	}
	boxBool, boxU := instantiate(boolean), instantiate(u)
	x, err := m.MkConst(astTestSymbol(t, m, "legacy-box-bool"), boxBool)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "legacy-box-u"), boxU)
	astTestMust(t, err)
	d, err := m.MkFuncDecl(astTestSymbol(t, m, "legacy-box-p"), []ASTHandle{boxBool, boxU}, boolean, ASTFuncDeclInfo{})
	astTestMust(t, err)
	root, err := m.MkApp(d, []ASTHandle{x, y})
	astTestMust(t, err)
	astTestPin(t, m, root)
	p, err := NewLegacySMTPrinter(m)
	astTestMust(t, err)
	var out strings.Builder
	astTestMust(t, p.DisplayBenchmark(&out, root, LegacySMTBenchmarkConfig{}))
	text := out.String()
	if strings.Count(text, "(declare-datatypes") != 1 || !strings.Contains(text, "(par (T)(LegacyBox (legacy-box (legacy-unbox T))))") || !strings.Contains(text, "(LegacyBox Bool)") || !strings.Contains(text, "(LegacyBox LegacyU)") || strings.Contains(text, "(declare-sort T)") {
		t.Fatal("parametric declaration was instantiated or repeated", text)
	}
}

// AUDIT.md ast_07-F4: pp_dt and pp_sort_decl end with newline(), which
// emits m_indent spaces; display_smt2 passes m_simplify_implies as indent.
func TestLegacySMTBenchmarkDatatypeDeclarationIndent(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	sorts, err := m.MkDatatypes([]DatatypeDefinition{
		{Name: astTestSymbol(t, m, "IndentD"), Constructors: []DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "indent-c"), Recognizer: astTestSymbol(t, m, "indent-is-c")},
		}},
	})
	astTestMust(t, err)
	astTestPin(t, m, sorts[0])
	x, err := m.MkConst(astTestSymbol(t, m, "indent-x"), sorts[0])
	astTestMust(t, err)
	astTestPin(t, m, x)
	root, err := m.MkEq(x, x)
	astTestMust(t, err)
	astTestPin(t, m, root)
	for _, simplify := range []bool{true, false} {
		p, err := NewLegacySMTPrinter(m)
		astTestMust(t, err)
		astTestMust(t, p.SetSimplifyImplies(simplify))
		var out strings.Builder
		astTestMust(t, p.DisplayBenchmark(&out, root, LegacySMTBenchmarkConfig{}))
		want := "))\n(declare-fun indent-x"
		if simplify {
			want = "))\n (declare-fun indent-x"
		}
		if !strings.Contains(out.String(), want) {
			t.Fatalf("simplify_implies=%v: %q", simplify, out.String())
		}
	}
}
