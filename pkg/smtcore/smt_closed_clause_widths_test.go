package smtcore

import (
	"fmt"
	"runtime"
	"slices"
	"testing"
	"unsafe"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTRetiredDirectoryKeepsLiteralStorageAtEveryWidth(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	astTestMust(t, s.SetBinaryClauseOptimization(false))
	var atoms []ast.ASTHandle
	for i := 0; i < 130; i++ {
		atoms = append(atoms, smtRelevancyBool(t, m, fmt.Sprintf("held-width-%d", i)))
	}
	lits := scopeRestorationLiterals(t, s, atoms...)
	smtTestUserPush(t, s)
	widths := []int{3, 2, 4, 5, 8, 9, 16, 17, 32, 33, 64, 65, 129}
	for i, n := range widths {
		astTestMust(t, s.addClause(lits[:n], i%2 != 0, ast.ASTHandle{}))
	}
	first := s.testClauses()[0].clause
	before := s.Clauses()
	wide := s.auxClauses[len(s.auxClauses)-1]
	detail := wide.directoryDetails()
	if detail == nil || len(detail.literals) != 129 {
		t.Fatal("missing wide directory range")
	}
	index := first.literals[1].Negated().Index()
	if !s.watches[index].RemoveClause(first) {
		t.Fatal("missing watch")
	}
	if s.PopScopes(1) == nil {
		t.Fatal("expected refused pop")
	}
	runtime.GC()
	after := s.Clauses()
	if len(after) != len(before) {
		t.Fatal("changed directory length")
	}
	for i, rec := range after {
		if !slices.Equal(rec.Literals, before[i].Literals) || rec.Root != before[i].Root {
			t.Fatalf("retired %d-literal clause changed", widths[i])
		}
	}
	astTestMust(t, s.watches[index].InsertClause(first))
	astTestMust(t, s.PopScopes(1))
	if wide.directoryDetails() != nil || detail.literals != nil || detail.owner != nil || !detail.proof.IsNull() {
		t.Fatal("successfully retired clause retained its directory data")
	}
}

func TestSMTConcreteClauseDirectoryLayout(t *testing.T) {
	var s SMTInternalizerState
	pointer := unsafe.Sizeof(uintptr(0))
	if unsafe.Sizeof(s.auxClauses[0]) != pointer || unsafe.Sizeof(s.lemmas[0]) != pointer {
		t.Fatalf("auxiliary and learned directory entries must fit in %d bytes", pointer)
	}
	if unsafe.Sizeof(uintptr(0)) == 8 && unsafe.Sizeof(SMTClause{}) != 64 {
		t.Fatalf("64-bit clause header is %d bytes, want 64", unsafe.Sizeof(SMTClause{}))
	}
}
