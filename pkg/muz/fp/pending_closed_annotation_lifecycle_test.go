package fp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// A pending closed input is an original formula, not its eager normalization.
// Its annotations and identity must survive a scoped named replacement.
func TestPendingClosedAnnotationsNamedUpdateAndScope(t *testing.T) {
	f := newContextDisplayCleanupFixture(t)
	m, e := f.m, f.e
	p, r := e.predicateOrder[0], e.predicateOrder[2]
	boolean, err := m.GetDomain(p, 0)
	fragmentMust(t, err)
	x := f.own(m.MkVar(0, boolean))
	px := f.own(m.MkApp(p, []ast.ASTHandle{x}))
	rx := f.own(m.MkApp(r, []ast.ASTHandle{x}))
	pattern := f.own(m.MkPattern([]ast.ASTHandle{px}))
	binder, err := m.Symbols.Intern("phase-original-binder")
	fragmentMust(t, err)
	qid, err := m.Symbols.Intern("phase-original-qid")
	fragmentMust(t, err)
	skid, err := m.Symbols.Intern("phase-original-skolem")
	fragmentMust(t, err)
	original := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{binder},
		Body: f.own(m.MkImplies(px, rx)), Weight: 7, QID: qid, SkolemID: skid,
		Patterns: []ast.ASTHandle{pattern},
	}))
	fragmentMust(t, e.AddRuleWithBound(original, "phase-annotation", 29))
	read := func() []ast.ASTHandle {
		t.Helper()
		refs, err := e.RuleFormulas()
		fragmentMust(t, err)
		f.roots = append(f.roots, refs...)
		result := make([]ast.ASTHandle, len(refs))
		for i, ref := range refs {
			result[i], err = m.RefGet(ref)
			fragmentMust(t, err)
		}
		return result
	}
	display := func() string {
		t.Helper()
		var output bytes.Buffer
		fragmentMust(t, e.WriteContextSMT2(&output, nil, nil))
		return output.String()
	}
	checkOriginal := func() {
		t.Helper()
		forms := read()
		if len(forms) < 2 || forms[len(forms)-2] != original {
			t.Fatal("pending original lost identity", forms)
		}
		gotWeight, err := m.GetQuantifierWeight(original)
		fragmentMust(t, err)
		gotQID, gotSkid, err := m.GetQuantifierIDs(original)
		fragmentMust(t, err)
		gotName, err := m.GetDeclarationName(original, 0)
		fragmentMust(t, err)
		count, err := m.GetNumPatterns(original)
		fragmentMust(t, err)
		gotPattern, err := m.GetPattern(original, 0)
		fragmentMust(t, err)
		if gotWeight != 7 || gotQID != qid || gotSkid != skid || gotName != binder || count != 1 || gotPattern != pattern {
			t.Fatal("pending annotation metadata changed", gotWeight, gotQID, gotSkid, gotName, count)
		}
		if text := display(); !strings.Contains(text, ":named phase-annotation") || !strings.Contains(text, "display-P") || !strings.Contains(text, "display-R") {
			t.Fatal("pending display lost source metadata", text)
		}
	}
	checkOriginal()
	namedLine := func(text string) string {
		t.Helper()
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, ":named phase-annotation") {
				return line
			}
		}
		t.Fatal("missing named rule in display", text)
		return ""
	}
	originalLine := namedLine(display())
	if !strings.Contains(originalLine, "=>") || !strings.Contains(originalLine, "display-P") {
		t.Fatal("pending original has wrong rule body", originalLine)
	}
	fragmentMust(t, e.Push())
	replacementName, err := m.Symbols.Intern("phase-replacement-binder")
	fragmentMust(t, err)
	replacement := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{replacementName}, Body: rx, Weight: 3}))
	fragmentMust(t, e.UpdateRule(replacement, "phase-annotation"))
	replacedLine := namedLine(display())
	if replacedLine == originalLine || strings.Contains(replacedLine, "display-P") {
		t.Fatal("scoped update displayed stale original", replacedLine, originalLine)
	}
	fragmentMust(t, e.Pop())
	checkOriginal()
	if restoredLine := namedLine(display()); restoredLine != originalLine {
		t.Fatal("scope rollback changed the displayed rule", restoredLine, originalLine)
	}
	fragmentMust(t, e.flushPendingRules())
	flushed := read()
	if len(flushed) < 2 || flushed[len(flushed)-2] == original {
		t.Fatal("flush did not expose normalized formula", flushed)
	}
	if text := display(); strings.Contains(text, "phase-original-binder") || strings.Contains(text, ":weight 7") {
		t.Fatal("flushed display retained source-only annotations", text)
	}
	if again := read(); len(again) != len(flushed) || again[len(again)-2] != flushed[len(flushed)-2] {
		t.Fatal("repeated enumeration changed normalized identity", flushed, again)
	}
}

// Explicit quantifier display should expose pending input annotations; after
// flush, output comes from normalized clauses rather than the raw binder.
func TestPendingClosedAnnotationDisplayProfiles(t *testing.T) {
	f := newFPRuleNameFixture(t)
	m, e := f.m, f.e
	boolean := f.own(m.MkBoolSort())
	x := f.own(m.MkVar(0, boolean))
	_, head := f.predicate("phase-profile-P", x)
	pattern := f.own(m.MkPattern([]ast.ASTHandle{head}))
	binder, err := m.Symbols.Intern("phase-profile-binder")
	fragmentMust(t, err)
	qid, err := m.Symbols.Intern("phase-profile-qid")
	fragmentMust(t, err)
	original := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{binder}, Body: head, Weight: 7, QID: qid, Patterns: []ast.ASTHandle{pattern}}))
	fragmentMust(t, e.AddRule(original, "phase-profile-rule"))
	_, otherHead := f.predicate("phase-profile-Q", x)
	noPattern := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{binder}, Body: otherHead, NoPatterns: []ast.ASTHandle{otherHead}}))
	fragmentMust(t, e.AddRule(noPattern, "phase-no-pattern-rule"))
	params := e.params.EmptyValue()
	key := "print_with_variable_declarations"
	fragmentMust(t, e.params.SetBoolName(&params, &key, false))
	defer func() { fragmentMust(t, e.params.Del(&params)) }()
	display := func() string {
		t.Helper()
		var output bytes.Buffer
		fragmentMust(t, e.WriteContextSMT2(&output, nil, &params))
		return output.String()
	}
	pending := display()
	for _, expected := range []string{"phase-profile-binder", ":weight 7", ":qid phase-profile-qid", ":pattern", ":no-pattern", ":named phase-profile-rule", ":named phase-no-pattern-rule"} {
		if !strings.Contains(pending, expected) {
			t.Fatal("pending display omitted annotation", expected, pending)
		}
	}
	fragmentMust(t, e.flushPendingRules())
	flushed := display()
	for _, stale := range []string{"phase-profile-binder", ":weight 7", ":qid phase-profile-qid", ":pattern", ":no-pattern"} {
		if strings.Contains(flushed, stale) {
			t.Fatal("normalized display kept pending annotation", stale, flushed)
		}
	}
	if !strings.Contains(flushed, ":named phase-profile-rule") || !strings.Contains(flushed, ":named phase-no-pattern-rule") {
		t.Fatal("normalized display lost rule name", flushed)
	}
}
