package ast

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestASTExprSubstitutionNativeDependencyEraseFinishesSameEntry(t *testing.T) {
	m := astTestManager(t)
	key := astDependencyExpr(t, m, 10200)
	value := astDependencyExpr(t, m, 10201)
	dep, e := m.DependencyLeaf(astDependencyExpr(t, m, 10202))
	astTestMust(t, e)
	s, e := m.NewExprSubstitutionCores(true)
	astTestMust(t, e)
	astTestMust(t, m.ExprSubstitutionInsert(s, key, value, ASTHandle{}, dep))
	m.BudgetField.FailAfter(0)
	e = m.ExprSubstitutionErase(s, key)
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	found, e := m.ExprSubstitutionContains(s, key)
	astTestMust(t, e)
	if found {
		t.Fatal("native erase must remove the same entry")
	}
	if m.dependencies.live != 0 {
		t.Fatal("dependency roots before Close", m.dependencies.live)
	}
	astTestMust(t, m.DelExprSubstitution(&s))
}

func TestASTExprSubstitutionNativeValueReplacement(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	key, e := m.MkVar(11600, sort)
	astTestMust(t, e)
	old, e := m.MkVar(11601, sort)
	astTestMust(t, e)
	incoming, e := m.MkVar(11602, sort)
	astTestMust(t, e)
	astTestMust(t, m.IncRef(key))
	astTestMust(t, m.IncRef(incoming))
	subst, e := m.NewExprSubstitution()
	astTestMust(t, e)
	astTestMust(t, m.ExprSubstitutionInsert(subst, key, old, ASTHandle{}, ASTDependency{}))
	m.BudgetField.FailAfter(0)
	astTestMust(t, m.ExprSubstitutionInsert(subst, key, incoming, ASTHandle{}, ASTDependency{}))
	m.BudgetField.FailAfter(-1)
	table, e := m.substTable(subst, 0)
	astTestMust(t, e)
	keyRef, e := m.internal(key)
	astTestMust(t, e)
	incomingRef, e := m.internal(incoming)
	astTestMust(t, e)
	entry, found := table.entries.Get2(astSharedKey(keyRef))
	if !found || entry.value != incomingRef || len(m.exprSubstitutions[subst.record].pending) != 0 {
		t.Fatal("native value replacement", entry, found)
	}
	if m.IsLiveHandle(old) {
		t.Fatal("old value survived")
	}
	astTestMust(t, m.DelExprSubstitution(&subst))
	if rc, e := m.GetRefCount(incoming); e != nil || rc != 1 {
		t.Fatal("incoming reference after close", rc, e)
	}
	astTestMust(t, m.DecRef(key))
	astTestMust(t, m.DecRef(incoming))
}

func TestASTExprSubstitutionNativeProofReplacement(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ASTProofEnabled))
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	key, e := m.MkVar(11610, sort)
	astTestMust(t, e)
	value, e := m.MkVar(11611, sort)
	astTestMust(t, e)
	old, e := m.MkProofTrue()
	astTestMust(t, e)
	incoming, e := m.MkProofAsserted(key)
	astTestMust(t, e)
	for _, h := range []ASTHandle{key, value, incoming} {
		astTestMust(t, m.IncRef(h))
	}
	subst, e := m.NewExprSubstitutionPolicy(false, true)
	astTestMust(t, e)
	astTestMust(t, m.ExprSubstitutionInsert(subst, key, value, old, ASTDependency{}))
	m.BudgetField.FailAfter(0)
	astTestMust(t, m.ExprSubstitutionInsert(subst, key, value, incoming, ASTDependency{}))
	m.BudgetField.FailAfter(-1)
	table, e := m.substTable(subst, 1)
	astTestMust(t, e)
	keyRef, e := m.internal(key)
	astTestMust(t, e)
	incomingRef, e := m.internal(incoming)
	astTestMust(t, e)
	entry, found := table.entries.Get2(astSharedKey(keyRef))
	if !found || entry.value != incomingRef || len(m.exprSubstitutions[subst.record].pending) != 0 {
		t.Fatal("native proof replacement", entry, found)
	}
	if m.IsLiveHandle(old) {
		t.Fatal("old proof survived")
	}
	astTestMust(t, m.DelExprSubstitution(&subst))
	if rc, e := m.GetRefCount(incoming); e != nil || rc != 1 {
		t.Fatal("incoming proof reference after close", rc, e)
	}
	for _, h := range []ASTHandle{incoming, key, value} {
		astTestMust(t, m.DecRef(h))
	}
}
func TestASTExprSubstitutionNativeDestructorAfterSparseErase(t *testing.T) {
	m := astTestManager(t)
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	var keys [100]ASTHandle
	value := astDependencyExpr(t, m, 10300)
	for i := range keys {
		keys[i] = astDependencyExpr(t, m, uint32(10400+i))
		astTestMust(t, m.ExprSubstitutionInsert(s, keys[i], value, ASTHandle{}, ASTDependency{}))
	}
	for i := 0; i < 95; i++ {
		astTestMust(t, m.ExprSubstitutionErase(s, keys[i]))
	}
	m.BudgetField.FailAfter(0)
	e = m.DelExprSubstitution(&s)
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	if s != (ASTExprSubstitution{}) {
		t.Fatal("fully consumed lexical owner must be reclaimed", s)
	}
}
func TestASTExprSubstitutionResetRefusalKeepsConsumedKeyProgress(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	key, e := m.MkVar(11500, sort)
	astTestMust(t, e)
	value, e := m.MkVar(11501, sort)
	astTestMust(t, e)
	base := m.allocator.Stats().Live
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	astTestMust(t, m.ExprSubstitutionInsert(s, key, value, ASTHandle{}, ASTDependency{}))
	m.nodeAt(value.slot).rc = 0 // Actual refused source counter guard, no fake callback.
	e = m.ExprSubstitutionReset(s)
	if !errors.Is(e, ErrASTDomain) {
		t.Fatal(e)
	}
	if m.IsLiveHandle(key) {
		t.Fatal("consumed key must be dead")
	}
	table, e := m.substTable(s, 0)
	astTestMust(t, e)
	entry, found := table.entries.Get2(uint64(key.slot)<<32 | uint64(key.generation))
	if !found {
		t.Fatal("stopped value entry lost")
	}
	if entry.owned != 2 {
		t.Fatal("retain only unconsumed value", entry)
	}
	m.nodeAt(value.slot).rc = 1
	astTestMust(t, m.ExprSubstitutionReset(s))
	astTestMust(t, m.DelExprSubstitution(&s))
	if m.allocator.Stats().Live != base {
		t.Fatal("cold records before Close")
	}
}
func TestASTExprSubstitutionConstructorEveryReservationPrefix(t *testing.T) {
	success := false
	for fail := int64(0); fail < 80; fail++ {
		m := astTestManager(t)
		astTestMust(t, m.ToggleProofMode(ASTProofEnabled))
		cells := m.allocator.Stats().Live
		m.BudgetField.FailAfter(fail)
		s, e := m.NewExprSubstitutionPolicy(true, true)
		m.BudgetField.FailAfter(-1)
		if e != nil {
			if !errors.Is(e, arena.ErrInjectedFailure) {
				t.Fatal(fail, e)
			}
			if s != (ASTExprSubstitution{}) {
				t.Fatal(s)
			}
		} else {
			success = true
			astTestMust(t, m.DelExprSubstitution(&s))
		}
		if m.allocator.Stats().Live != cells {
			t.Fatal("constructor immediate owned graph", fail, m.allocator.Stats().Live, cells)
		}
		if success {
			break
		}
	}
	if !success {
		t.Fatal("reservation sweep never reached success")
	}
}
func TestASTExprSubstitutionNativeOwnerIdentityAndBorrowedProvenance(t *testing.T) {
	for _, ty := range []reflect.Type{reflect.TypeOf(ASTExprSubstitution{}), reflect.TypeOf(ASTScopedExprSubstitution{}), reflect.TypeOf(astExprSubstEntry{})} {
		astTestMust(t, arena.ValidateRecordType(ty))
	}
	m := astTestManager(t)
	s, e := m.NewExprSubstitutionPolicy(true, false)
	astTestMust(t, e)
	scope, e := m.NewScopedExprSubstitution(s)
	astTestMust(t, e)
	if n, e := m.allocator.Size(s.record); e != nil || n != astExprSubstBytes {
		t.Fatal("actual scalar owner identity", n, e)
	}
	primary, e := m.substTable(s, 0)
	astTestMust(t, e)
	dep, e := m.substTable(s, 2)
	astTestMust(t, e)
	if primary == dep {
		t.Fatal("policy tables share dictionary state")
	}
	foreign := astTestManager(t)
	if _, e := foreign.ExprSubstitutionEmpty(s); !errors.Is(e, ErrASTHandle) {
		t.Fatal("foreign owner accepted", e)
	}
	if _, e := foreign.ScopedExprSubstitutionScopeLevel(scope); !errors.Is(e, ErrASTHandle) {
		t.Fatal("foreign scope accepted", e)
	}
	staleScope := scope
	astTestMust(t, m.DelScopedExprSubstitution(&scope))
	scope, e = m.NewScopedExprSubstitution(s)
	astTestMust(t, e)
	if _, e := m.ScopedExprSubstitutionScopeLevel(staleScope); !errors.Is(e, ErrASTHandle) {
		t.Fatal("dead scope accepted after new owner publication", e)
	}
	astTestMust(t, m.DelScopedExprSubstitution(&scope))
	stale := s
	astTestMust(t, m.DelExprSubstitution(&s))
	if _, e := m.ExprSubstitutionEmpty(stale); !errors.Is(e, ErrASTHandle) {
		t.Fatal("dead owner accepted", e)
	}
}

func TestASTExprSubstitutionExplicitProofGuardRunsAfterPrimaryConstruction(t *testing.T) {
	m := astTestManager(t)
	base := m.allocator.Stats().Live
	s, e := m.NewExprSubstitutionPolicy(false, true)
	if ASTSourceDebug {
		if !errors.Is(e, ErrASTDomain) || s != (ASTExprSubstitution{}) {
			t.Fatal(s, e)
		}
	} else {
		astTestMust(t, e)
		astTestMust(t, m.DelExprSubstitution(&s))
	}
	if m.allocator.Stats().Live != base {
		t.Fatal("explicit ctor cleanup")
	}
	m.BudgetField.FailAfter(0)
	_, e = m.NewExprSubstitutionPolicy(false, true)
	m.BudgetField.FailAfter(-1)
	if !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal("storage preparation precedes assertion", e)
	}
}
func TestASTExprSubstitutionNativeSparseResetReleasesDependencyRoots(t *testing.T) {
	m := astTestManager(t)
	s, e := m.NewExprSubstitutionCores(true)
	astTestMust(t, e)
	value := astDependencyExpr(t, m, 12000)
	dep, e := m.DependencyLeaf(value)
	astTestMust(t, e)
	astTestMust(t, m.DependencyInc(dep))
	var keys [100]ASTHandle
	for i := range keys {
		keys[i] = astDependencyExpr(t, m, uint32(12100+i))
		astTestMust(t, m.ExprSubstitutionInsert(s, keys[i], value, ASTHandle{}, dep))
	}
	for i := 0; i < 95; i++ {
		astTestMust(t, m.ExprSubstitutionErase(s, keys[i]))
	}
	m.BudgetField.FailAfter(0)
	e = m.ExprSubstitutionReset(s)
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	rc, e := m.DependencyRefCount(dep)
	astTestMust(t, e)
	if rc != 1 {
		t.Fatal("native reset must release all core references", rc)
	}
	astTestMust(t, m.ExprSubstitutionReset(s))
	astTestMust(t, m.DelExprSubstitution(&s))
	astTestMust(t, m.DependencyDec(dep))
}
func TestASTScopedExprSubstitutionNativeColdTrailPush(t *testing.T) {
	m := astTestManager(t)
	key := astDependencyExpr(t, m, 12200)
	value := astDependencyExpr(t, m, 12201)
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	scope, e := m.NewScopedExprSubstitution(s)
	astTestMust(t, e)
	m.BudgetField.FailAfter(0)
	e = m.ScopedExprSubstitutionInsert(scope, key, value, ASTHandle{}, ASTDependency{})
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	rc, e := m.GetRefCount(key)
	astTestMust(t, e)
	if rc != 3 {
		t.Fatal("map and native trail INC", rc)
	}
	a, e := m.scopedSubstState(scope)
	astTestMust(t, e)
	if len(a.trail) != 1 {
		t.Fatal(a.trail)
	}
	found, e := m.ExprSubstitutionContains(s, key)
	astTestMust(t, e)
	if !found {
		t.Fatal("installed trailed binding")
	}
	astTestMust(t, m.DelScopedExprSubstitution(&scope))
	astTestMust(t, m.DelExprSubstitution(&s))
	rc, e = m.GetRefCount(key)
	astTestMust(t, e)
	if rc != 1 {
		t.Fatal("native trail and map release", rc)
	}
}
func TestASTScopedExprSubstitutionNativeDependencyPopFinishesSuffix(t *testing.T) {
	m := astTestManager(t)
	key := astDependencyExpr(t, m, 12300)
	value := astDependencyExpr(t, m, 12301)
	dep, e := m.DependencyLeaf(value)
	astTestMust(t, e)
	s, e := m.NewExprSubstitutionCores(true)
	astTestMust(t, e)
	scope, e := m.NewScopedExprSubstitution(s)
	astTestMust(t, e)
	astTestMust(t, m.ScopedExprSubstitutionPush(scope))
	astTestMust(t, m.ScopedExprSubstitutionInsert(scope, key, value, ASTHandle{}, dep))
	m.BudgetField.FailAfter(0)
	e = m.ScopedExprSubstitutionPop(scope, 1)
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	a, e := m.scopedSubstState(scope)
	astTestMust(t, e)
	if a.operation != 0 || len(a.limits) != 0 {
		t.Fatal("native pop retained pending scope state", a)
	}
	if m.dependencies.live != 0 {
		t.Fatal(m.dependencies.live)
	}
	astTestMust(t, m.DelScopedExprSubstitution(&scope))
	astTestMust(t, m.DelExprSubstitution(&s))
}
func TestASTExprSubstitutionNativeEraseAfterLastKeyReference(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	key, e := m.MkVar(12400, b)
	astTestMust(t, e)
	value := astDependencyExpr(t, m, 12401)
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	astTestMust(t, m.ExprSubstitutionInsert(s, key, value, ASTHandle{}, ASTDependency{}))
	m.BudgetField.FailAfter(0)
	astTestMust(t, m.ExprSubstitutionErase(s, key))
	m.BudgetField.FailAfter(-1)
	if m.IsLiveHandle(key) {
		t.Fatal("sole-owned key was not destroyed")
	}
	empty, e := m.ExprSubstitutionEmpty(s)
	astTestMust(t, e)
	if !empty {
		t.Fatal("native erase retained a dead key")
	}
	astTestMust(t, m.DelExprSubstitution(&s))
}

func TestASTExprSubstitutionNativeCleanupWithReservationsDisabled(t *testing.T) {
	m := astTestManager(t)
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	value := astDependencyExpr(t, m, 12500)
	for i := 0; i < 257; i++ {
		key := astDependencyExpr(t, m, uint32(12600+i))
		astTestMust(t, m.ExprSubstitutionInsert(s, key, value, ASTHandle{}, ASTDependency{}))
	}
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	astTestMust(t, m.ExprSubstitutionCleanup(s))
	astTestMust(t, m.ExprSubstitutionCleanup(s))
	m.BudgetField.FailAfter(-1)
	empty, e := m.ExprSubstitutionEmpty(s)
	astTestMust(t, e)
	if !empty || m.BudgetField.Used() != before {
		t.Fatal("native cleanup changed actual object storage")
	}
	rc, e := m.GetRefCount(value)
	astTestMust(t, e)
	if rc != 1 {
		t.Fatal("cleanup lost caller value retain", rc)
	}
	astTestMust(t, m.DelExprSubstitution(&s))
}

func TestASTExprSubstitutionNativeEraseReinsertDiscoveryOrder(t *testing.T) {
	m := astTestManager(t)
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	keys := markTestNodes(t, m, 4)
	for _, key := range keys {
		astTestMust(t, m.IncRef(key))
	}
	defer func() {
		for _, key := range keys {
			astTestMust(t, m.DecRef(key))
		}
	}()
	value := astDependencyExpr(t, m, 13800)
	for _, key := range keys {
		astTestMust(t, m.ExprSubstitutionInsert(s, key, value, ASTHandle{}, ASTDependency{}))
	}
	m.BudgetField.FailAfter(0)
	astTestMust(t, m.ExprSubstitutionErase(s, keys[1]))
	astTestMust(t, m.ExprSubstitutionErase(s, keys[2]))
	astTestMust(t, m.ExprSubstitutionInsert(s, keys[1], value, ASTHandle{}, ASTDependency{}))
	t0, e := m.substTable(s, 0)
	astTestMust(t, e)
	want := []ASTHandle{keys[0], keys[3], keys[1]}
	i := 0
	for _, entry := range t0.entries.All() {
		if i >= len(want) || m.external(entry.key) != want[i] {
			t.Fatal("native discovery order", i)
		}
		i++
	}
	m.BudgetField.FailAfter(-1)
	if i != len(want) {
		t.Fatal("lost native binding", i)
	}
	astTestMust(t, m.DelExprSubstitution(&s))
}

func TestASTExprSubstitutionNativeDependencyResetFinishesAllEntries(t *testing.T) {
	m := astTestManager(t)
	s, e := m.NewExprSubstitutionCores(true)
	astTestMust(t, e)
	value := astDependencyExpr(t, m, 14000)
	var deps [2]ASTDependency
	for i := range deps {
		key := astDependencyExpr(t, m, uint32(14001+i))
		deps[i], e = m.DependencyLeaf(value)
		astTestMust(t, e)
		astTestMust(t, m.ExprSubstitutionInsert(s, key, value, ASTHandle{}, deps[i]))
	}
	m.BudgetField.FailAfter(0)
	e = m.ExprSubstitutionReset(s)
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	if m.dependencies.live != 0 {
		t.Fatal("native reset must clean all dependencies", m.dependencies.live)
	}
	astTestMust(t, m.ExprSubstitutionReset(s))
	astTestMust(t, m.DelExprSubstitution(&s))
	if m.dependencies.live != 0 {
		t.Fatal(m.dependencies.live)
	}
}
func TestASTExprSubstitutionDisabledPolicyDoesNotInspectUnusedForeignArguments(t *testing.T) {
	m := astTestManager(t)
	foreign := astTestManager(t)
	astTestMust(t, foreign.ToggleProofMode(ASTProofEnabled))
	proof, e := foreign.MkProofTrue()
	astTestMust(t, e)
	astTestPin(t, foreign, proof)
	dep, e := foreign.DependencyLeaf(astDependencyExpr(t, foreign, 15000))
	astTestMust(t, e)
	astTestMust(t, foreign.DependencyInc(dep))
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	key := astDependencyExpr(t, m, 15001)
	value := astDependencyExpr(t, m, 15002)
	astTestMust(t, m.ExprSubstitutionInsert(s, key, value, proof, dep))
	astTestMust(t, m.DelExprSubstitution(&s))
	astTestMust(t, foreign.DependencyDec(dep))
}
func TestASTScopedExprSubstitutionConstructorEveryReservationPrefix(t *testing.T) {
	success := false
	for fail := int64(0); fail < 80; fail++ {
		m := astTestManager(t)
		s, e := m.NewExprSubstitution()
		astTestMust(t, e)
		cells := m.allocator.Stats().Live
		m.BudgetField.FailAfter(fail)
		scope, e := m.NewScopedExprSubstitution(s)
		m.BudgetField.FailAfter(-1)
		if e != nil {
			if !errors.Is(e, arena.ErrInjectedFailure) || scope != (ASTScopedExprSubstitution{}) {
				t.Fatal(fail, scope, e)
			}
		} else {
			success = true
			astTestMust(t, m.DelScopedExprSubstitution(&scope))
		}
		if m.allocator.Stats().Live != cells {
			t.Fatal("scope actual constructor unwind before Close", fail)
		}
		astTestMust(t, m.DelExprSubstitution(&s))
		if success {
			break
		}
	}
	if !success {
		t.Fatal("scope sweep did not reach completion")
	}
}
func TestASTExprSubstitutionNativePolicyMapGrowthAndExactReferences(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ASTProofEnabled))
	s, e := m.NewExprSubstitutionPolicy(true, true)
	astTestMust(t, e)
	value := astDependencyExpr(t, m, 16000)
	proof, e := m.MkProofAsserted(value)
	astTestMust(t, e)
	astTestPin(t, m, proof)
	dep, e := m.DependencyLeaf(value)
	astTestMust(t, e)
	astTestMust(t, m.DependencyInc(dep))
	var keys []ASTHandle
	for i := 0; i < 257; i++ {
		keys = append(keys, astDependencyExpr(t, m, uint32(16001+i)))
	}
	valueRC, e := m.GetRefCount(value)
	astTestMust(t, e)
	proofRC, e := m.GetRefCount(proof)
	astTestMust(t, e)
	depRC, e := m.DependencyRefCount(dep)
	astTestMust(t, e)
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	for _, key := range keys {
		astTestMust(t, m.ExprSubstitutionInsert(s, key, value, proof, dep))
	}
	for _, key := range keys {
		var got, p ASTHandle
		var d ASTDependency
		found, e := m.ExprSubstitutionFindDependency(s, key, &got, &p, &d)
		astTestMust(t, e)
		rc, e := m.GetRefCount(key)
		astTestMust(t, e)
		if !found || got != value || p != proof || d != dep || rc != 2 {
			t.Fatal("native policy publication", found, got, p, d, rc)
		}
	}
	rc, e := m.GetRefCount(value)
	astTestMust(t, e)
	if rc != valueRC+257 {
		t.Fatal("definition retains", rc, valueRC)
	}
	rc, e = m.GetRefCount(proof)
	astTestMust(t, e)
	if rc != proofRC+257 {
		t.Fatal("proof retains", rc, proofRC)
	}
	drc, e := m.DependencyRefCount(dep)
	astTestMust(t, e)
	if drc != depRC+257 {
		t.Fatal("dependency retains", drc, depRC)
	}
	if m.BudgetField.Used() != before {
		t.Fatal("native maps charged bucket growth")
	}
	astTestMust(t, m.ExprSubstitutionReset(s))
	m.BudgetField.FailAfter(-1)
	rc, e = m.GetRefCount(value)
	astTestMust(t, e)
	if rc != valueRC {
		t.Fatal("definition reset", rc, valueRC)
	}
	rc, e = m.GetRefCount(proof)
	astTestMust(t, e)
	if rc != proofRC {
		t.Fatal("proof reset", rc, proofRC)
	}
	drc, e = m.DependencyRefCount(dep)
	astTestMust(t, e)
	if drc != depRC {
		t.Fatal("dependency reset", drc, depRC)
	}
	astTestMust(t, m.DelExprSubstitution(&s))
	astTestMust(t, m.DependencyDec(dep))
}

func TestASTExprSubstitutionNativeGrowthAndDuplicateReplacement(t *testing.T) {
	m := astTestManager(t)
	s, e := m.NewExprSubstitution()
	astTestMust(t, e)
	value := astDependencyExpr(t, m, 17200)
	var keys [8]ASTHandle
	for i := range keys {
		keys[i] = astDependencyExpr(t, m, uint32(17201+i))
		if i < 7 {
			astTestMust(t, m.ExprSubstitutionInsert(s, keys[i], value, ASTHandle{}, ASTDependency{}))
		}
	}
	m.BudgetField.FailAfter(0)
	e = m.ExprSubstitutionInsert(s, keys[7], value, ASTHandle{}, ASTDependency{})
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	astTestMust(t, m.ExprSubstitutionInsert(s, keys[7], value, ASTHandle{}, ASTDependency{}))
	table, e := m.substTable(s, 0)
	astTestMust(t, e)
	if table.entries.Len() != 8 {
		t.Fatal("retry must retain previous bindings", table)
	}
	astTestMust(t, m.DelExprSubstitution(&s))
}
