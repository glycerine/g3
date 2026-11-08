package tab

// Source: src/muz/tab/tab_context.cpp index::normalize/check_substitution.
// Normalization preserves quantifiers, then source is_ground(expr*) declines
// them because it requires an application, even for a valid implication.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestTabQuantifiedSubsumptionDeclinesWithoutErrorAndContinues(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, m.DelRef(&ref)) })
		return h
	}
	boolean := own(m.MkBoolSort())
	x := own(m.MkVar(0, boolean))
	quantified := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		tabDiagnosticsMust(t, err)
		// F and G are theory functions, not registered Horn predicates.
		declaration := own(m.MkFuncDecl(s, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
		body := own(m.MkApp(declaration, []ast.ASTHandle{x}))
		return own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	}
	c, d := quantified("subsumption-theory-F"), quantified("subsumption-theory-G")
	for _, quantifier := range []ast.ASTHandle{c, d} {
		ref, err := m.NewRef(quantifier)
		tabDiagnosticsMust(t, err)
		forms := []ast.ASTRef{ref}
		tabDiagnosticsMust(t, f.lite.EliminateConjunctions(nil, false, &forms))
		if len(forms) != 1 {
			t.Fatal("QE did not preserve the quantified test constraint", len(forms))
		}
		reduced, err := m.RefGet(forms[0])
		tabDiagnosticsMust(t, err)
		kind, err := m.GetKind(reduced)
		tabDiagnosticsMust(t, err)
		if kind != ast.ASTQuantifier {
			t.Fatal("test no longer reaches quantified literal normalization", kind)
		}
		tabDiagnosticsMust(t, m.DelRef(&forms[0]))
	}
	// Check semantic entailment separately from TAB's intentionally incomplete
	// subsumption decision: C entails C but does not entail independent D.
	for _, tc := range []struct {
		consequence ast.ASTHandle
		want        lbool.LBool
	}{{c, lbool.False}, {d, lbool.True}} {
		tabDiagnosticsMust(t, f.querySolver.Push())
		tabDiagnosticsMust(t, f.querySolver.Assert(c))
		tabDiagnosticsMust(t, f.querySolver.Assert(own(m.MkNot(tc.consequence))))
		status, err := f.querySolver.CheckSat(nil)
		tabDiagnosticsMust(t, err)
		tabDiagnosticsMust(t, f.querySolver.Pop(1))
		if status != tc.want {
			t.Fatal("independent implication oracle", status, tc.want)
		}
	}
	falsehood := own(m.MkFalse())
	clause := func(constraint ast.ASTHandle) *Clause {
		result, err := NewClause(m, f.rw, falsehood, nil, constraint)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, result.Close()) })
		return result
	}
	query := clause(c)
	entailed, independent := clause(c), clause(d)
	entailed.sequence, independent.sequence = 11, 12
	counted := &countingTabSolver{Solver: f.subSolver}
	// The first use sets the solvers up, and setup keeps state (theory_seq
	// init builds its seq_expr_solver kernel, as in the source). Measure the
	// ownership of declined subsumption after that one-time setup.
	if _, _, err := isClauseSubsumed(query, []*Clause{entailed}, f.lite, counted); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []*Clause{entailed, independent, entailed} {
		before := m.Stats()
		found, sequence, err := isClauseSubsumed(query, []*Clause{candidate}, f.lite, counted)
		tabDiagnosticsMust(t, err)
		if found || sequence != 0 || counted.checks != 0 {
			t.Fatal("quantified constraint escaped the source non-ground decline", found, sequence, counted.checks)
		}
		depth, err := counted.ScopeLevel()
		tabDiagnosticsMust(t, err)
		if depth != 0 {
			t.Fatal("declined quantified constraint left a solver scope", depth)
		}
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
			t.Fatal("declined quantified constraint changed AST ownership", before, after)
		}
	}
	// Declining the first candidate must leave the source search able to
	// prove an ordinary ground implication using the next candidate.
	name, err := m.Symbols.Intern("subsumption-ground-condition")
	tabDiagnosticsMust(t, err)
	ground := own(m.MkConst(name, boolean))
	mixedQuery := clause(own(m.MkAnd(c, ground)))
	ordinary := clause(ground)
	ordinary.sequence = 19
	for i := 0; i < 2; i++ {
		found, sequence, err := isClauseSubsumed(mixedQuery, []*Clause{entailed, ordinary}, f.lite, counted)
		tabDiagnosticsMust(t, err)
		if !found || sequence != ordinary.sequence || counted.checks != uint32(i+1) {
			t.Fatal("quantified decline interrupted later ground subsumption", found, sequence, counted.checks)
		}
		depth, err := counted.ScopeLevel()
		tabDiagnosticsMust(t, err)
		if depth != 0 {
			t.Fatal("successful ground fallback left a solver scope", depth)
		}
	}
}
