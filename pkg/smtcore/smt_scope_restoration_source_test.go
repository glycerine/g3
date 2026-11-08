package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func scopeRestorationLiterals(t *testing.T, s *SMTInternalizerState, expressions ...ast.ASTHandle) []SMTLiteral {
	t.Helper()
	result := make([]SMTLiteral, len(expressions))
	for i, e := range expressions {
		astTestMust(t, s.Internalize(e, true))
		var err error
		result[i], err = s.Literal(e)
		astTestMust(t, err)
	}
	return result
}

func TestSMTScopeRestorationReassertsUnitAndConflictInPlace(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		name := "unit"
		if conflict {
			name = "conflict"
		}
		t.Run(name, func(t *testing.T) {
			m := astTestManager(t)
			_, _, s := smtInternalizerFixture(t, m, nil)
			smtTestUserPush(t, s)
			a := smtRelevancyBool(t, m, "restore-replay-a")
			b := smtRelevancyBool(t, m, "restore-replay-b")
			base := scopeRestorationLiterals(t, s, a, b)
			for _, l := range base {
				astTestMust(t, s.AssignLiteral(l.Negated(), NewSMTBooleanAxiom()))
			}
			astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
			astTestMust(t, s.PushScope())
			c := smtRelevancyBool(t, m, "restore-replay-c")
			if conflict {
				var err error
				c, err = m.MkOr(a, b)
				astTestMust(t, err)
			}
			cl := scopeRestorationLiterals(t, s, c)[0]
			astTestMust(t, s.AddLearnedClause([]SMTLiteral{cl, base[0], base[1]}))
			original := s.clauses[len(s.clauses)-1].clause
			astTestMust(t, s.PopScopes(1))
			if s.clauses[0].clause != original || s.inconsistent != conflict {
				t.Fatal("restoration lost identity or conflict status", s.inconsistent, conflict)
			}
			if conflict {
				reason, ok := s.conflict.Clause()
				if !ok || reason != original {
					t.Fatal("conflict did not use the surviving clause")
				}
			} else {
				current, err := s.Literal(c)
				astTestMust(t, err)
				value, err := s.Assignment(current)
				astTestMust(t, err)
				reason, ok := s.booleanData[current.Var()].Justification().Clause()
				if value != lbool.True || !ok || reason != original {
					t.Fatal("unit was not reasserted with its original clause", value)
				}
			}
			astTestMust(t, s.endSearchPersistence())
			astTestMust(t, s.PopScopes(1))
		})
	}
}

func TestSMTScopeRestorationReassertsStableAuxiliaryLemma(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	smtTestUserPush(t, s)
	expressions := []ast.ASTHandle{smtRelevancyBool(t, m, "stable-aux-a"), smtRelevancyBool(t, m, "stable-aux-b")}
	ls := scopeRestorationLiterals(t, s, expressions...)
	astTestMust(t, s.AssignLiteral(ls[0].Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addClauseKind([]SMTLiteral{ls[1], ls[0]}, false, SMTClauseAuxLemma, ast.ASTHandle{}))
	original := s.clauses[0].clause
	if !original.InReinitStack() || original.ReinternalizeAtoms() {
		t.Fatal("wrong source reinitialization flags for stable auxiliary unit")
	}
	astTestMust(t, s.PopScopes(1))
	value, err := s.Assignment(ls[1])
	astTestMust(t, err)
	if value != lbool.True || s.clauses[0].clause != original || original.InReinitStack() {
		t.Fatal("stable auxiliary unit was not reasserted at the search base", value)
	}
	astTestMust(t, s.endSearchPersistence())
	astTestMust(t, s.PopScopes(1))
}

func TestSMTScopeRestorationCrossesMultipleSearchScopes(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	smtTestUserPush(t, s)
	a := smtRelevancyBool(t, m, "multi-restore-a")
	al := scopeRestorationLiterals(t, s, a)[0]
	astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
	astTestMust(t, s.PushScope())
	b := smtRelevancyBool(t, m, "multi-restore-b")
	bl := scopeRestorationLiterals(t, s, b)[0]
	astTestMust(t, p.MarkRelevant(b))
	astTestMust(t, s.PushScope())
	c := smtRelevancyBool(t, m, "multi-restore-c")
	cl := scopeRestorationLiterals(t, s, c)[0]
	astTestMust(t, p.MarkRelevant(c))
	astTestMust(t, s.AddLearnedClause([]SMTLiteral{al, bl.Negated(), cl}))
	original := s.clauses[0].clause
	if !original.ReinternalizeAtoms() || original.NumAtoms() != 3 {
		t.Fatal("scoped lemma did not acquire its source atom owners")
	}
	astTestMust(t, s.PopScopes(2))
	if s.scopeLevel != 1 || len(s.clauses) != 1 || s.clauses[0].clause != original || original.ReinternalizeAtoms() || len(original.atoms) != 0 {
		t.Fatal("multi-pop lost clause identity or kept temporary atom roots")
	}
	if p.IsRelevant(b) || p.IsRelevant(c) {
		t.Fatal("multi-pop restored discarded relevance")
	}
	for _, e := range []ast.ASTHandle{b, c} {
		v, found := s.BoolVar(e)
		if !found || s.booleanData[v].InternalLevel() != 1 {
			t.Fatal("multi-pop failed to bind at the search base")
		}
	}
	astTestMust(t, s.endSearchPersistence())
	astTestMust(t, s.PopScopes(1))
}

func TestSMTScopeRestorationSavesAtomsBeforeWatchSelection(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	smtTestUserPush(t, s)
	astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
	astTestMust(t, s.PushScope())
	expressions := []ast.ASTHandle{smtRelevancyBool(t, m, "restore-order-a"), smtRelevancyBool(t, m, "restore-order-b"), smtRelevancyBool(t, m, "restore-order-c")}
	ls := scopeRestorationLiterals(t, s, expressions...)
	astTestMust(t, s.AssignLiteral(ls[1].Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AddLearnedClause(ls))
	original := s.clauses[0].clause
	if original.literals[1] != ls[2] {
		t.Fatal("setup did not move the second watch")
	}
	for i, e := range expressions {
		if original.atoms[i].expression != e || original.atoms[i].sign {
			t.Fatal("saved atoms followed watch order instead of original clause order", i)
		}
	}
	astTestMust(t, s.PopScopes(2))
	previous := int32(-1)
	for _, e := range expressions {
		v, found := s.BoolVar(e)
		if !found || v <= previous {
			t.Fatal("reinitialization changed source atom creation order")
		}
		previous = v
	}
	astTestMust(t, s.endSearchPersistence())
	astTestMust(t, s.PopScopes(1))
}

func TestSMTScopeRestorationNestedMixedClausesAndMovedWatches(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	smtTestUserPush(t, s)
	a := smtRelevancyBool(t, m, "nested-restore-a")
	b := smtRelevancyBool(t, m, "nested-restore-b")
	base := scopeRestorationLiterals(t, s, a, b)
	astTestMust(t, p.MarkRelevant(a))
	astTestMust(t, p.MarkRelevant(b))
	astTestMust(t, p.Propagate())
	astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
	astTestMust(t, s.PushScope())
	d := smtRelevancyBool(t, m, "nested-restore-d")
	dl := scopeRestorationLiterals(t, s, d)[0]
	astTestMust(t, p.MarkRelevant(d))
	astTestMust(t, s.addGateClause([]SMTLiteral{base[0].Negated(), base[1]})) // scoped auxiliary before lemma
	astTestMust(t, s.AddLearnedClause([]SMTLiteral{base[0], base[1], dl}))
	first := s.clauses[len(s.clauses)-1].clause
	astTestMust(t, s.PushScope())
	e := smtRelevancyBool(t, m, "nested-restore-e")
	el := scopeRestorationLiterals(t, s, e)[0]
	astTestMust(t, p.MarkRelevant(e))
	astTestMust(t, s.addGateClause([]SMTLiteral{base[0].Negated(), el}))
	astTestMust(t, s.addClauseKind([]SMTLiteral{base[0], dl.Negated(), el}, false, SMTClauseAuxLemma, ast.ASTHandle{}))
	second := s.clauses[len(s.clauses)-1].clause
	astTestMust(t, s.AssignLiteral(base[0].Negated(), NewSMTBooleanAxiom()))
	ok, err := s.Propagate()
	astTestMust(t, err)
	if !ok {
		t.Fatal("unexpected setup conflict")
	}
	watches := [2]SMTLiteral{first.literals[0], first.literals[1]}
	if first.IsWatch(base[0]) {
		t.Fatal("setup did not move the false watch")
	}
	astTestMust(t, s.PopScopes(1))
	if len(s.clauses) != 3 || s.clauses[1].clause != first || s.clauses[2].clause != second {
		t.Fatal("nested pop lost clause order or identity", len(s.clauses))
	}
	if first.literals[0] != watches[0] || first.literals[1] != watches[1] {
		t.Fatal("surviving watch positions changed")
	}
	if !p.IsRelevant(d) || p.IsRelevant(e) {
		t.Fatal("nested relevance did not follow scope lifetimes")
	}
	astTestMust(t, s.PopScopes(1))
	if len(s.clauses) != 2 || s.clauses[0].clause != first || s.clauses[1].clause != second {
		t.Fatal("auxiliary deletion shifted or lost surviving lemmas")
	}
	if !p.IsRelevant(a) || !p.IsRelevant(b) || p.IsRelevant(d) || p.IsRelevant(e) {
		t.Fatal("base relevance mismatch")
	}
	for _, c := range []*SMTClause{first, second} {
		for _, l := range c.literals[:2] {
			if s.watches[l.Negated().Index()].FindClause(c) < 0 {
				t.Fatal("nested restoration lost a watch")
			}
		}
	}
	astTestMust(t, s.endSearchPersistence())
	astTestMust(t, s.PopScopes(1))
	if !first.closed || !second.closed {
		t.Fatal("nested lemmas outlived user base")
	}
}

func TestSMTScopeRestorationPreservesNonGateNotVariable(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	smtTestUserPush(t, s)
	astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
	astTestMust(t, s.PushScope())
	a := smtRelevancyBool(t, m, "restore-not-a")
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(notA, false))
	notVar, found := s.BoolVar(notA)
	if !found {
		t.Fatal("missing nongate NOT variable")
	}
	b := smtRelevancyBool(t, m, "restore-not-b")
	c := smtRelevancyBool(t, m, "restore-not-c")
	others := scopeRestorationLiterals(t, s, b, c)
	astTestMust(t, s.AddLearnedClause([]SMTLiteral{NewSMTLiteral(notVar, false), others[0], others[1]}))
	original := s.clauses[len(s.clauses)-1].clause
	astTestMust(t, s.PopScopes(1))
	notVar, found = s.BoolVar(notA)
	if !found {
		t.Fatal("restoration normalized away the source NOT variable")
	}
	found = false
	for _, l := range original.literals {
		found = found || l == NewSMTLiteral(notVar, false)
	}
	if !found {
		t.Fatal("restoration changed the NOT literal representation")
	}
	ls := scopeRestorationLiterals(t, s, a, b, c)
	astTestMust(t, s.AssignLiteral(ls[0], NewSMTBooleanAxiom()))
	astTestMust(t, s.AssignLiteral(ls[1].Negated(), NewSMTBooleanAxiom()))
	ok, err := s.Propagate()
	astTestMust(t, err)
	if !ok {
		t.Fatal("nongate NOT restoration caused conflict")
	}
	value, err := s.Assignment(ls[2])
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("nongate NOT restoration lost semantics", value)
	}
	astTestMust(t, s.endSearchPersistence())
	astTestMust(t, s.PopScopes(1))
}

// A theory can refuse its pop after the Boolean bindings have been retired.
// Hold that real owner boundary to exercise AST allocation refusal specifically
// during clause reinitialization, rather than only during snapshot acquisition.
type scopeRestorationHoldTheory struct {
	*smtInternalizerTestTheory
	hold bool
}

func (t *scopeRestorationHoldTheory) PopScopes(n uint32) error {
	if t.hold {
		return ast.ErrASTPending
	}
	return t.SMTTheoryBase.PopScopes(n)
}

func TestSMTScopeRestorationProofOwnershipFailureRetryAndClose(t *testing.T) {
	for _, stage := range []string{"capture", "rebind"} {
		for _, abandon := range []bool{false, true} {
			t.Run(stage+map[bool]string{false: "/retry", true: "/close"}[abandon], func(t *testing.T) {
				f := newSMTNativeCleanupFixture(t)
				m, s := f.manager, f.state
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				family, err := m.ArithmeticFamilyID()
				astTestMust(t, err)
				base, err := NewSMTTheoryBase(family)
				astTestMust(t, err)
				theory := &scopeRestorationHoldTheory{smtInternalizerTestTheory: &smtInternalizerTestTheory{SMTTheoryBase: base}}
				astTestMust(t, s.RegisterTheory(theory))
				smtTestUserPush(t, s)
				astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
				astTestMust(t, s.PushScope())
				expressions := []ast.ASTHandle{smtRelevancyBool(t, m, "restore-failure-a"), smtRelevancyBool(t, m, "restore-failure-b"), smtRelevancyBool(t, m, "restore-failure-c")}
				ls := scopeRestorationLiterals(t, s, expressions...)
				formula, err := m.MkOrArgs(expressions)
				astTestMust(t, err)
				proof, err := m.MkProofAsserted(formula)
				astTestMust(t, err)
				handler := &smtClauseTestDeleteHandler{}
				astTestMust(t, s.addClauseKindWithDeleteHandler(ls, false, SMTClauseLearned, proof, handler))
				original := s.clauses[0].clause
				owner := original.Justification()
				refs, err := m.GetRefCount(proof)
				astTestMust(t, err)
				if stage == "rebind" {
					theory.hold = true
					err = s.PopScopes(1)
					if !errors.Is(err, ast.ErrASTPending) || s.pendingPop == nil || s.pendingPop.stage != 3 || s.clauses[0].closeWatches != 2 {
						t.Fatal("did not reach detached clause boundary", err)
					}
					theory.hold = false
				}
				f.refuse(t, func() { err = s.PopScopes(1) })
				if !errors.Is(err, ast.ErrASTPending) || s.pendingPop == nil || original.closed || handler.calls != 0 {
					t.Fatal("native refusal lost live clause", err)
				}
				if stage == "rebind" && (s.pendingPop.stage != 6 || s.pendingPop.learned[0].installed || s.clauses[0].closeWatches != 2) {
					t.Fatal("refused reinitialization lost its cursor")
				}
				if abandon {
					astTestMust(t, s.Close())
					if !original.closed || handler.calls != 1 {
						t.Fatal("abandoned pop did not destroy clause exactly once")
					}
					return
				}
				astTestMust(t, s.PopScopes(1))
				if len(s.clauses) != 1 || s.clauses[0].clause != original || original.Justification() != owner || handler.calls != 0 {
					t.Fatal("retry replaced clause, proof owner, or deletion handler")
				}
				after, err := m.GetRefCount(proof)
				astTestMust(t, err)
				if after != refs {
					t.Fatal("retry leaked a proof reference", refs, after)
				}
				wrapper, ok := owner.(*SMTJustificationProofWrapper)
				if !ok {
					t.Fatal("missing proof wrapper")
				}
				actual, err := wrapper.Proof()
				astTestMust(t, err)
				if actual != proof {
					t.Fatal("restoration changed proof")
				}
				for _, l := range original.literals[:2] {
					count := 0
					for _, c := range s.watches[l.Negated().Index()].Clauses() {
						if c == original {
							count++
						}
					}
					if count != 1 {
						t.Fatal("retry lost or duplicated a watch", count)
					}
				}
				astTestMust(t, s.endSearchPersistence())
				astTestMust(t, s.PopScopes(1))
				if !original.closed || handler.calls != 1 {
					t.Fatal("final pop did not destroy clause exactly once")
				}
			})
		}
	}
}

// Pin the source contract without invoking or reading C++: a search backtrack
// keeps learned clause objects; only retired Boolean bindings are rebuilt.
// Relevance set in the discarded scope does not follow rebuilt literals.
func TestSMTScopeRestorationSourceContract(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		name := "surviving"
		if scoped {
			name = "scoped"
		}
		t.Run(name, func(t *testing.T) {
			m := astTestManager(t)
			_, p, s := smtInternalizerFixture(t, m, nil)
			smtTestUserPush(t, s)
			expressions := []ast.ASTHandle{smtRelevancyBool(t, m, "source-restore-a"), smtRelevancyBool(t, m, "source-restore-b"), smtRelevancyBool(t, m, "source-restore-c")}
			internalize := func() {
				for _, e := range expressions {
					astTestMust(t, s.Internalize(e, true))
					astTestMust(t, p.MarkRelevant(e))
				}
				astTestMust(t, p.Propagate())
			}
			if !scoped {
				internalize()
			}
			astTestMust(t, s.beginSearchPersistence(1, SMTParamsState{}))
			astTestMust(t, s.PushScope())
			if scoped {
				internalize()
			}
			literals := make([]SMTLiteral, len(expressions))
			for i, e := range expressions {
				var err error
				literals[i], err = s.Literal(e)
				astTestMust(t, err)
			}
			astTestMust(t, s.AddLearnedClause([]SMTLiteral{literals[0], literals[1].Negated(), literals[2]}))
			original := s.clauses[0].clause
			astTestMust(t, original.SetActivity(37))
			creation := s.booleanCreation[literals[0].Var()]
			made, deleted := s.statistics.NumMkClause, s.statistics.NumDelClause
			astTestMust(t, s.PopScopes(1))
			if len(s.clauses) != 1 || s.clauses[0].clause != original {
				t.Fatal("backtrack replaced the learned clause object")
			}
			if s.statistics.NumMkClause != made || s.statistics.NumDelClause != deleted {
				t.Fatal("backtrack created or deleted a surviving clause")
			}
			activity, err := original.Activity()
			astTestMust(t, err)
			if activity != 37 {
				t.Fatal("lost clause activity", activity)
			}
			for i, e := range expressions {
				variable, found := s.BoolVar(e)
				if !found {
					t.Fatal("lost learned atom")
				}
				if p.IsRelevant(e) == scoped {
					t.Fatal("relevance does not match the surviving scope", scoped, i)
				}
				if i == 0 && (s.booleanCreation[variable] != creation) != scoped {
					t.Fatal("wrong Boolean variable lifetime")
				}
				expected := NewSMTLiteral(variable, i == 1)
				found = false
				for _, l := range original.Literals() {
					found = found || l == expected
				}
				if !found {
					t.Fatal("lost literal or polarity", expected)
				}
			}
			for _, l := range original.Literals()[:2] {
				if s.watches[l.Negated().Index()].FindClause(original) < 0 {
					t.Fatal("missing restored watch")
				}
			}
			a, err := s.Literal(expressions[0])
			astTestMust(t, err)
			b, err := s.Literal(expressions[1])
			astTestMust(t, err)
			astTestMust(t, s.AssignLiteral(a.Negated(), NewSMTBooleanAxiom()))
			astTestMust(t, s.AssignLiteral(b, NewSMTBooleanAxiom()))
			ok, err := s.Propagate()
			astTestMust(t, err)
			if !ok {
				t.Fatal("unexpected restored clause conflict")
			}
			c, err := s.Literal(expressions[2])
			astTestMust(t, err)
			value, err := s.Assignment(c)
			astTestMust(t, err)
			if value != lbool.True {
				t.Fatal("restored clause lost propagation", value)
			}
			astTestMust(t, s.endSearchPersistence())
			astTestMust(t, s.PopScopes(1))
			if !original.closed || len(s.clauses) != 0 {
				t.Fatal("learned clause survived its user base")
			}
		})
	}
}

// smtTestUserPush models context::push: push_scope followed by m_base_lvl++.
func smtTestUserPush(t *testing.T, s *SMTInternalizerState) {
	t.Helper()
	astTestMust(t, s.PushScope())
	s.pushBaseScope()
}
