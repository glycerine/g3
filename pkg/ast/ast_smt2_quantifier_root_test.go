package ast

import (
	"strings"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestASTSMT2QuantifierRootSourceFormat(t *testing.T) {
	c, g, p, budget := dynAckSourceFixture(t)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.Close()) }()
	boolSort, err := m.MkBoolSort()
	paramsMust(t, err)
	x := astTestSymbol(t, m, "x")
	y := astTestSymbol(t, m, "y")
	xBang1 := astTestSymbol(t, m, "x!1")
	body, err := m.MkVar(0, boolSort)
	paramsMust(t, err)
	print := func(h ASTHandle) string {
		t.Helper()
		format, e := m.SMT2PPBooleanTree(h, nil)
		paramsMust(t, e)
		out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: budget})
		paramsMust(t, e)
		paramsMust(t, m.PrintFormat(out, format, c, p, g))
		text, e := out.String()
		paramsMust(t, e)
		paramsMust(t, out.Close())
		return text
	}
	for _, tc := range []struct {
		forall bool
		names  []usymbol.Symbol
		want   string
	}{
		{true, []usymbol.Symbol{x}, "(forall ((x Bool)) x)"},
		{false, []usymbol.Symbol{x, y}, "(exists ((x Bool) (y Bool)) y)"},
		{true, []usymbol.Symbol{usymbol.Numerical(0)}, "(forall ((x!1 Bool)) x!1)"},
		{false, []usymbol.Symbol{usymbol.Numerical(0), usymbol.Numerical(1)}, "(exists ((x!1 Bool) (x!2 Bool)) x!2)"},
		{true, []usymbol.Symbol{x, x}, "(forall ((x Bool) (x!1 Bool)) x!1)"},
		{true, []usymbol.Symbol{xBang1, usymbol.Numerical(0)}, "(forall ((x!1 Bool) (x!2 Bool)) x!2)"},
	} {
		sorts := make([]ASTHandle, len(tc.names))
		for i := range sorts {
			sorts[i] = boolSort
		}
		q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: tc.forall, Sorts: sorts, Names: tc.names, Body: body})
		paramsMust(t, e)
		if got := print(q); got != tc.want {
			t.Fatalf("quantifier format %q, want %q", got, tc.want)
		}
	}
	q, err := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{x}, Body: body, Weight: 2})
	paramsMust(t, err)
	if got := print(q); got != "(forall ((x Bool)) (! x :weight 2))" {
		t.Fatalf("weighted quantifier format %q", got)
	}
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "f"), []ASTHandle{boolSort}, boolSort, ASTFuncDeclInfo{})
	paramsMust(t, err)
	term, err := m.MkApp(f, []ASTHandle{body})
	paramsMust(t, err)
	pattern, err := m.MkFamilyApp(2, 0, nil, []ASTHandle{term})
	paramsMust(t, err)
	qid := astTestSymbol(t, m, "qid")
	skid := astTestSymbol(t, m, "skid")
	q, err = m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{x}, Body: body,
		Patterns: []ASTHandle{pattern}, Weight: 3, QID: qid, SkolemID: skid})
	paramsMust(t, err)
	if got := print(q); got != "(forall ((x Bool)) (! x :pattern ((f x)) :weight 3 :skolemid skid :qid qid))" {
		t.Fatalf("pattern annotation format %q", got)
	}
	q, err = m.MkQuantifier(ASTQuantifierSpec{Forall: false, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{x}, Body: body,
		NoPatterns: []ASTHandle{term}, QID: usymbol.Numerical(7)})
	paramsMust(t, err)
	if got := print(q); got != "(exists ((x Bool)) (! x :no-pattern (f x)))" {
		t.Fatalf("no-pattern annotation format %q", got)
	}
	inner, err := m.MkQuantifier(ASTQuantifierSpec{Forall: false, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{x}, Body: body})
	paramsMust(t, err)
	outer, err := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{x}, Body: inner})
	paramsMust(t, err)
	if got := print(outer); got != "(forall ((x Bool)) (exists ((x!1 Bool)) x!1))" {
		t.Fatalf("nested shadowed quantifier format %q", got)
	}
	truth, err := m.MkFamilyApp(0, ASTOpTrue, nil, nil)
	paramsMust(t, err)
	compound, err := m.MkAnd(inner, truth)
	paramsMust(t, err)
	if got := print(compound); got != "(and (exists ((x Bool)) x) true)" {
		t.Fatalf("application with quantified child format %q", got)
	}
	// Source quantifier scopes keep let aliases inside the binder and skip
	// binder names when assigning a!N aliases.
	aliasBinder := astTestSymbol(t, m, "a!1")
	sharedArgs := []ASTHandle{body}
	for i := 0; i < 8; i++ {
		atom, e := m.MkConst(astTestSymbol(t, m, string(rune('a'+i))), boolSort)
		paramsMust(t, e)
		sharedArgs = append(sharedArgs, atom)
	}
	shared, err := m.MkAndArgs(sharedArgs)
	paramsMust(t, err)
	sharedBody, err := m.MkAnd(shared, shared)
	paramsMust(t, err)
	q, err = m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{aliasBinder}, Body: sharedBody})
	paramsMust(t, err)
	want := "(forall ((a!1 Bool)) (let ((a!2 (and a!1 a b c d e f g h))) (and a!2 a!2)))"
	if got := print(q); got != want {
		t.Fatalf("quantified local let format %q, want %q", got, want)
	}
	repeatedQuantifier, err := m.MkAnd(q, q)
	paramsMust(t, err)
	want = "(let ((a!1 (forall ((a!1 Bool))\n             (let ((a!2 (and a!1 a b c d e f g h)))\n               (and a!2 a!2)))))\n  (and a!1 a!1))"
	if got := print(repeatedQuantifier); got != want {
		t.Fatalf("shared quantifier format %q, want %q", got, want)
	}
	deepQuantifier := body
	for i := 0; i < 4; i++ {
		deepQuantifier, err = m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{x}, Body: deepQuantifier})
		paramsMust(t, err)
	}
	deepParent, err := m.MkNot(deepQuantifier)
	paramsMust(t, err)
	var depthInfo smt2PPInfo
	_, err = m.smt2PPExprTreeWithInfo(deepQuantifier, nil, false, &depthInfo)
	paramsMust(t, err)
	if depthInfo.weight != 9 || depthInfo.depth != 5 {
		t.Fatalf("source quantified metadata: %+v", depthInfo)
	}
	deepText := print(deepParent)
	if !strings.Contains(deepText, "(let ((a!1 (forall") || !strings.Contains(deepText, "(not a!1))") {
		t.Fatalf("deep quantified child did not receive source depth alias: %q", deepText)
	}
}

// AUDIT.md ast_07-F6: register_var_names renames a clashing quoted binder
// to the unquoted next_name symbol, which pp_var then quotes again.
func TestASTSMT2QuantifierRenamesQuotedBinder(t *testing.T) {
	c, g, p, budget := dynAckSourceFixture(t)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.Close()) }()
	boolSort, err := m.MkBoolSort()
	paramsMust(t, err)
	xy := astTestSymbol(t, m, "x y")
	v, err := m.MkVar(0, boolSort)
	paramsMust(t, err)
	inner, err := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{xy}, Body: v})
	paramsMust(t, err)
	outer, err := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{xy}, Body: inner})
	paramsMust(t, err)
	format, err := m.SMT2PPBooleanTree(outer, nil)
	paramsMust(t, err)
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: budget})
	paramsMust(t, err)
	paramsMust(t, m.PrintFormat(out, format, c, p, g))
	text, err := out.String()
	paramsMust(t, err)
	paramsMust(t, out.Close())
	if want := `(forall ((|x y| Bool)) (forall ((|\|x y\|!1| Bool)) |\|x y\|!1|))`; text != want {
		t.Fatalf("renamed quoted binder %q, want %q", text, want)
	}
}
