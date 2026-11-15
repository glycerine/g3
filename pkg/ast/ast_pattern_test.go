package ast

import "testing"

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
