package tab

// Source: src/muz/tab/tab_context.cpp matcher::match_app appends uncertain
// equalities into the caller's owning expr_ref_vector of side conditions.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
)

func TestSemanticMatcherEqualityReferenceFailureAndRetry(t *testing.T) {
	f, err := newTabTestContext()
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, f.Close()) }()
	m := f.manager
	var roots []ast.ASTRef
	defer func() {
		m.Budget().FailAfter(-1)
		for i := len(roots) - 1; i >= 0; i-- {
			tabDiagnosticsMust(t, m.DelRef(&roots[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		roots = append(roots, ref)
		return h
	}
	zero := own(m.ArithmeticIntegerNumeralText("0"))
	integer, err := m.GetSort(zero)
	tabDiagnosticsMust(t, err)
	boolean := own(m.MkBoolSort())
	constant := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		tabDiagnosticsMust(t, err)
		return own(m.MkConst(s, integer))
	}
	a, b, c := constant("matcher-allocation-a"), constant("matcher-allocation-b"), constant("matcher-allocation-c")
	name, err := m.Symbols.Intern("matcher-allocation-P")
	tabDiagnosticsMust(t, err)
	predicate := own(m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
	pattern := own(m.MkApp(predicate, []ast.ASTHandle{a}))
	term := own(m.MkApp(predicate, []ast.ASTHandle{b}))
	// Prime the equality declaration with a different application, keeping
	// the actual matcher condition fresh while avoiding cache-growth noise.
	warm, err := m.MkEq(a, c)
	tabDiagnosticsMust(t, err)
	warmRef, err := m.NewRef(warm)
	tabDiagnosticsMust(t, err)
	tabDiagnosticsMust(t, m.DelRef(&warmRef))
	bindings, err := substitution.New(m)
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, bindings.Close()) }()
	tabDiagnosticsMust(t, bindings.Reserve(2, 0))
	matcher, err := NewSemanticMatcher(m)
	tabDiagnosticsMust(t, err)
	truth, err := m.MkTrue()
	tabDiagnosticsMust(t, err)
	previous, err := m.NewRef(truth)
	tabDiagnosticsMust(t, err)
	conditions := []ast.ASTRef{previous}
	defer func() {
		for i := range conditions {
			tabDiagnosticsMust(t, m.DelRef(&conditions[i]))
		}
	}()
	for {
		m.Budget().FailAfter(0)
		ref, cause := m.NewRef(ast.ASTHandle{})
		m.Budget().FailAfter(-1)
		if cause != nil {
			if ref != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
				t.Fatal("reference allocation frontier", ref, cause)
			}
			break
		}
		roots = append(roots, ref)
		if len(roots) > 4096 {
			t.Fatal("did not reach a physical reference allocation")
		}
	}
	before := m.Stats()
	checkOwners := func(label string) {
		t.Helper()
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
			t.Fatal(label, "changed live AST ownership", before, after)
		}
	}
	m.Budget().FailAfter(0)
	matched, cause := matcher.Match(pattern, term, bindings, &conditions)
	m.Budget().FailAfter(-1)
	if matched || !errors.Is(cause, arena.ErrInjectedFailure) || len(conditions) != 1 || conditions[0] != previous {
		t.Fatal("failed matcher changed the caller's existing conditions", matched, cause, conditions)
	}
	checkOwners("failed equality publication")
	for i := 0; i < 2; i++ {
		matched, err = matcher.Match(pattern, term, bindings, &conditions)
		tabDiagnosticsMust(t, err)
		if !matched || len(conditions) != 2 || conditions[0] != previous {
			t.Fatal("retry lost an owning equality condition", matched, conditions)
		}
		equality, err := m.RefGet(conditions[1])
		tabDiagnosticsMust(t, err)
		expected, err := m.MkEq(a, b)
		tabDiagnosticsMust(t, err)
		if equality != expected {
			t.Fatal("retry changed the source equality condition")
		}
		tabDiagnosticsMust(t, m.DelRef(&conditions[1]))
		conditions = conditions[:1]
		checkOwners("completed retry")
	}
}
