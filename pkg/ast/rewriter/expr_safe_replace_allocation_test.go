package rewriter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source expr_safe_replace.cpp owns rebuilt applications in m_refs and the
// quantifier's local var_shifter through construction failure. Use a cold wide
// application rewrite to cross reference pages, and a single quantifier to
// reach the shifter's partially initialized native owner without a large run.
func TestExprSafeReplaceAllocationFailureOwnsApplicationsAndShifter(t *testing.T) {
	for _, quantifier := range []bool{false, true} {
		t.Run(fmt.Sprintf("quantifier=%t", quantifier), func(t *testing.T) {
			failures, completed := 0, false
			for allowance := int64(0); allowance < 256 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					m := astTestManager(t)
					boolean, err := m.MkBoolSort()
					astTestMust(t, err)
					source, err := m.MkConst(astTestSymbol(t, m, "allocation-source"), boolean)
					astTestMust(t, err)
					astTestPin(t, m, source)
					destination, err := m.MkConst(astTestSymbol(t, m, "allocation-destination"), boolean)
					astTestMust(t, err)
					astTestPin(t, m, destination)
					var input ast.ASTHandle
					var applications, declarations []ast.ASTHandle
					binderName := astTestSymbol(t, m, "allocation-bound")
					if quantifier {
						input, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{binderName}, Body: source})
						astTestMust(t, err)
					} else {
						applications, declarations = make([]ast.ASTHandle, 512), make([]ast.ASTHandle, 512)
						for i := range applications {
							declarations[i], err = m.MkFuncDecl(astTestSymbol(t, m, fmt.Sprintf("allocation-predicate-%d", i)), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
							astTestMust(t, err)
							applications[i], err = m.MkApp(declarations[i], []ast.ASTHandle{source})
							astTestMust(t, err)
						}
						input, err = m.MkAndArgs(applications)
						astTestMust(t, err)
					}
					astTestPin(t, m, input)
					out := astTestPin(t, m, input)
					replace, err := NewExprSafeReplace(m)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, replace.Close()) })
					astTestMust(t, replace.Insert(source, destination))
					counts := func() [4]uint32 {
						s := m.Stats()
						return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
					}
					check := func(formula ast.ASTHandle, rewritten bool) {
						t.Helper()
						want := source
						if rewritten {
							want = destination
						}
						if quantifier {
							body, err := m.GetQuantifierBody(formula)
							astTestMust(t, err)
							forall, err := m.IsForall(formula)
							astTestMust(t, err)
							count, err := m.GetNumDeclarations(formula)
							astTestMust(t, err)
							name, err := m.GetDeclarationName(formula, 0)
							astTestMust(t, err)
							sort, err := m.GetDeclarationSort(formula, 0)
							astTestMust(t, err)
							if body != want || !forall || count != 1 || name != binderName || sort != boolean {
								t.Fatal("replacement changed quantifier semantics or metadata", rewritten, body, want, forall, count, name, sort)
							}
							return
						}
						count, err := m.GetNumArgs(formula)
						astTestMust(t, err)
						if count != uint32(len(applications)) {
							t.Fatal("replacement changed application count", count)
						}
						for i, declaration := range declarations {
							application, err := m.GetArg(formula, uint32(i))
							astTestMust(t, err)
							gotDecl, err := m.GetDecl(application)
							astTestMust(t, err)
							argument, err := m.GetArg(application, 0)
							astTestMust(t, err)
							if gotDecl != declaration || argument != want {
								t.Fatal("replacement changed predicate order or argument", rewritten, i, gotDecl, declaration, argument, want)
							}
							if !rewritten && application != applications[i] {
								t.Fatal("replacement mutated original application identity", i)
							}
						}
					}
					before := counts()
					m.Budget().FailAfter(allowance)
					cause := replace.Apply(input, out)
					m.Budget().FailAfter(-1)
					if cause != nil {
						failures++
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal("replacement lost allocation cause", cause)
						}
						if actual, err := m.RefGet(out); err != nil || actual != input {
							t.Fatal("failed replacement published partial output", actual, err)
						}
					} else {
						completed = true
						actual, err := m.RefGet(out)
						astTestMust(t, err)
						check(actual, true)
						astTestMust(t, m.SetRef(out, input))
					}
					if after := counts(); after != before {
						t.Fatal("replacement failure lost AST ownership", allowance, cause, before, after)
					}
					check(input, false)
					astTestMust(t, replace.Apply(input, out))
					actual, err := m.RefGet(out)
					astTestMust(t, err)
					check(actual, true)
					astTestMust(t, m.SetRef(out, input))
					if after := counts(); after != before {
						t.Fatal("successful replacement retry lost AST ownership", before, after)
					}
					check(input, false)
				}) {
					break
				}
			}
			if failures == 0 || !completed {
				t.Fatal("replacement allocation campaign missed failure or completion", failures, completed)
			}
		})
	}
}
