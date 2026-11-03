package tab

// Source: src/muz/tab/tab_context.cpp unifier::extract_subst and
// imp::get_proof replay only the winning branch after rule backtracking.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestTabProofReplaysNonlinearWinningBranchWithSparseVariableBank(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, m.DelRef(&ref)) })
		return h
	}
	zero, one := own(m.ArithmeticIntegerNumeralText("0")), own(m.ArithmeticIntegerNumeralText("1"))
	integer, err := m.GetSort(zero)
	tabDiagnosticsMust(t, err)
	boolean := own(m.MkBoolSort())
	declare := func(text string, domain ...ast.ASTHandle) ast.ASTHandle {
		name, err := m.Symbols.Intern(text)
		tabDiagnosticsMust(t, err)
		return own(m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{}))
	}
	p := declare("proof-branch-P", integer)
	bad := declare("proof-branch-Bad", integer)
	q := declare("proof-branch-Q", integer, integer)
	r := declare("proof-branch-R", integer)
	apply := func(declaration ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		return own(m.MkApp(declaration, args))
	}
	x, y := own(m.MkVar(0, integer)), own(m.MkVar(2, integer))
	truth, falsehood := own(m.MkTrue()), own(m.MkFalse())
	clause := func(head ast.ASTHandle, predicates ...ast.ASTHandle) *Clause {
		result, err := NewClause(m, f.rw, head, predicates, truth)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, result.Close()) })
		return result
	}
	px, p0 := apply(p, x), apply(p, zero)
	qxy, ry := apply(q, x, y), apply(r, y)
	q01, r1 := apply(q, zero, one), apply(r, one)
	failed := clause(px, apply(bad, x))
	winning := clause(px, qxy, ry)
	qFact, rFact := clause(q01), clause(r1)
	rules := &RuleIndex{manager: m, clauses: []*Clause{failed, winning, qFact, rFact}, byHead: map[ast.ASTHandle][]int{p: {0, 1}, q: {2}, r: {3}}}
	goal := clause(falsehood, p0)
	search, err := newTabSearch(goal, rules, f.rw, f.lite, f.querySolver, f.subSolver, "first")
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, search.Close()) }()
	status, err := search.Run()
	tabDiagnosticsMust(t, err)
	if status != lbool.True || search.unfold != 4 || search.unfoldFail != 0 || search.subsumed != 0 || len(search.stack) != 4 || len(search.owned) != 5 {
		t.Fatal("nonlinear sibling search differs from source", status, search.unfold, search.unfoldFail, search.subsumed, len(search.stack), len(search.owned))
	}
	for i, sequence := range []uint32{0, 2, 3, 4} {
		if search.stack[i].sequence != sequence {
			t.Fatal("failed sibling appeared on the winning path", i, search.stack[i].sequence, sequence)
		}
	}
	if search.stack[1].parentRule != 1 || search.stack[1].NumVars() != 1 || winning.NumVars() != 3 {
		t.Fatal("winning rule or sparse-bank compaction was lost")
	}
	predicates, err := search.stack[1].Predicates()
	tabDiagnosticsMust(t, err)
	y0 := own(m.MkVar(0, integer))
	if len(predicates) != 2 || predicates[0] != apply(q, zero, y0) || predicates[1] != apply(r, y0) {
		t.Fatal("source body splice or variable compaction changed", predicates)
	}
	answer, err := search.Proof()
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, m.DelRef(&answer)) }()
	proof, err := m.RefGet(answer)
	tabDiagnosticsMust(t, err)
	conclusion, err := m.GetProofFact(proof)
	tabDiagnosticsMust(t, err)
	if !m.IsFalse(conclusion) {
		t.Fatal("winning proof does not conclude false")
	}
	queryPremise := own(m.MkImplies(p0, falsehood))
	ruleBody := own(m.MkImplies(own(m.MkAnd(qxy, ry)), px))
	asserted := make(map[ast.ASTHandle]bool)
	seen := make(map[ast.ASTHandle]bool)
	pending := []ast.ASTHandle{proof}
	resolutions, sparseBanks := 0, 0
	for len(pending) != 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		kind, err := m.GetDeclKind(node)
		tabDiagnosticsMust(t, err)
		if kind == ast.ASTPrAsserted {
			fact, err := m.GetProofFact(node)
			tabDiagnosticsMust(t, err)
			factKind, err := m.GetKind(fact)
			tabDiagnosticsMust(t, err)
			if factKind == ast.ASTQuantifier {
				count, err := m.GetNumDeclarations(fact)
				tabDiagnosticsMust(t, err)
				gap, err := m.GetDeclarationSort(fact, 1)
				tabDiagnosticsMust(t, err)
				if count != 3 || gap != boolean {
					t.Fatal("source sparse binder closure changed", count, gap)
				}
				fact, err = m.GetQuantifierBody(fact)
				tabDiagnosticsMust(t, err)
			}
			if fact != queryPremise && fact != ruleBody && fact != q01 && fact != r1 {
				t.Fatal("proof asserted a failed sibling or a synthetic premise", fact)
			}
			asserted[fact] = true
		}
		if kind == ast.ASTPrHyperResolve {
			resolutions++
			output, err := ast.NewASTProofHyperOutput(m)
			tabDiagnosticsMust(t, err)
			fact, err := m.NewRef(ast.ASTHandle{})
			tabDiagnosticsMust(t, err)
			var premises []ast.ASTHandle
			matched, err := m.MatchProofHyperResolve(node, &premises, fact, output)
			tabDiagnosticsMust(t, err)
			if !matched || len(premises) != 2 || output.SubstitutionsLen() != 2 || output.PositionsLen() != 1 {
				t.Fatal("source binary resolution shape changed")
			}
			position, err := output.Position(0)
			tabDiagnosticsMust(t, err)
			if position != (ast.ASTProofPosition{First: 1, Second: 0}) {
				t.Fatal("source selected-tail pivot changed", position)
			}
			bank, err := output.Substitution(1)
			tabDiagnosticsMust(t, err)
			if len(bank) == 3 {
				if bank[0] != zero || !m.IsTrue(bank[1]) || bank[2] != y0 {
					t.Fatal("source sparse rule substitution changed", bank)
				}
				sparseBanks++
			}
			for _, premise := range premises {
				tabDiagnosticsMust(t, m.DecRef(premise))
			}
			tabDiagnosticsMust(t, m.DelRef(&fact))
			tabDiagnosticsMust(t, output.Close())
		}
		parents, err := m.GetProofNumParents(node)
		tabDiagnosticsMust(t, err)
		for i := uint32(0); i < parents; i++ {
			parent, err := m.GetProofParent(node, i)
			tabDiagnosticsMust(t, err)
			pending = append(pending, parent)
		}
	}
	if resolutions != 3 || sparseBanks != 1 || len(asserted) != 4 {
		t.Fatal("proof does not contain exactly the winning source derivation", resolutions, sparseBanks, len(asserted))
	}
	// Check derivability in a fresh solver as well as the proof metadata.
	// The source checker does not itself enforce pivot complementarity.
	oracle, err := f.ctx.NewSimpleSolver()
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, oracle.Close()) }()
	originalRule := own(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer, boolean, integer},
		Names:  []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1), symbol.Numerical(2)},
		Body:   ruleBody,
	}))
	for _, premise := range []ast.ASTHandle{originalRule, q01, r1} {
		tabDiagnosticsMust(t, oracle.Assert(premise))
	}
	status, err = oracle.CheckSat(nil)
	tabDiagnosticsMust(t, err)
	if status != lbool.True {
		t.Fatal("winning program premises are not independently consistent", status)
	}
	tabDiagnosticsMust(t, oracle.Assert(queryPremise))
	status, err = oracle.CheckSat(nil)
	tabDiagnosticsMust(t, err)
	if status != lbool.False {
		t.Fatal("winning original premises do not refute the negated query", status)
	}
	checker, err := proofs.NewChecker(m)
	tabDiagnosticsMust(t, err)
	valid, conditions, err := checker.Check(proof)
	tabDiagnosticsMust(t, err)
	defer func() { tabDiagnosticsMust(t, checker.ReleaseSideConditions(conditions)) }()
	if !valid || len(conditions) == 0 {
		t.Fatal("nonlinear proof did not produce independently checkable obligations", valid, len(conditions))
	}
	for _, condition := range conditions {
		tabDiagnosticsMust(t, f.querySolver.Push())
		tabDiagnosticsMust(t, f.querySolver.Assert(own(m.MkNot(condition))))
		status, err := f.querySolver.CheckSat(nil)
		tabDiagnosticsMust(t, err)
		tabDiagnosticsMust(t, f.querySolver.Pop(1))
		if status != lbool.False {
			t.Fatal("proof obligation is not independently valid", status)
		}
	}
}
