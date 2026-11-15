package ast

import (
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md ast_06-F3: ast_manager::is_pattern also requires every argument
// of the pattern to be an application.
func TestASTIsPatternRejectsVariableArgument(t *testing.T) {
	m := astTestManager(t)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	v, err := m.MkVar(0, boolSort)
	astTestMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "f"), []ASTHandle{boolSort}, boolSort, ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx, err := m.MkApp(f, []ASTHandle{v})
	astTestMust(t, err)
	good, err := m.MkFamilyApp(2, 0, nil, []ASTHandle{fx})
	astTestMust(t, err)
	bad, err := m.MkFamilyApp(2, 0, nil, []ASTHandle{v, fx})
	astTestMust(t, err)
	for _, tc := range []struct {
		h    ASTHandle
		want bool
	}{{good, true}, {bad, false}, {fx, false}} {
		got, e := m.IsPattern(tc.h)
		astTestMust(t, e)
		if got != tc.want {
			t.Fatalf("IsPattern = %v, want %v", got, tc.want)
		}
		got, e = m.RewriterPattern(tc.h)
		astTestMust(t, e)
		if got != tc.want {
			t.Fatalf("RewriterPattern = %v, want %v", got, tc.want)
		}
	}
}

// AUDIT.md ast_03-F5: mk_quantifier rejects patterns together with
// no-patterns with its ast_exception text.
func TestASTQuantifierPatternsAndNoPatternsMessage(t *testing.T) {
	m := astTestManager(t)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	v, err := m.MkVar(0, boolSort)
	astTestMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "f"), []ASTHandle{boolSort}, boolSort, ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx, err := m.MkApp(f, []ASTHandle{v})
	astTestMust(t, err)
	pattern, err := m.MkPattern([]ASTHandle{fx})
	astTestMust(t, err)
	_, err = m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{boolSort}, Names: []usymbol.Symbol{astTestSymbol(t, m, "x")},
		Body: fx, Patterns: []ASTHandle{pattern}, NoPatterns: []ASTHandle{fx}})
	if err == nil || err.Error() != "simultaneous patterns and no-patterns not supported" {
		t.Fatalf("patterns with no-patterns: %v", err)
	}
}
