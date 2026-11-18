package smtcore

import (
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
)

// theory_array's read fingerprints are a fingerprint_set: one store with
// many index tuples must not share one linearly scanned bucket.
func TestSMTArrayReadFingerprintsManyIndicesOfOneStore(t *testing.T) {
	m, store, _, nodes := smtManyTupleFixture(t)
	_ = m
	theory := &SMTArrayTheory{readFingerprints: make(map[uint64][]*smtArrayReadFingerprint)}
	start := time.Now()
	for _, n := range nodes {
		if !theory.insertArrayReadFingerprint(store, []*SMTEnode{n}) {
			t.Fatal("distinct read fingerprint rejected")
		}
	}
	for _, n := range nodes {
		if theory.insertArrayReadFingerprint(store, []*SMTEnode{n}) {
			t.Fatal("duplicate read fingerprint inserted")
		}
	}
	if len(theory.readFingerprintTrail) != smtManyTuples {
		t.Fatal("trail", len(theory.readFingerprintTrail))
	}
	t.Logf("%d read fingerprints of one store: %v", smtManyTuples, time.Since(start))
}

// quick_checker's canonize asks get_enode_eq_to(f, args): a cg_table
// lookup, not a scan over every application of f.
func TestSMTQuickCheckerCongruentManyApplicationsOfOneDeclaration(t *testing.T) {
	m, state, _, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	sort, err := m.MkSort(astTestSymbol(t, m, "QuickManySort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	f := smtQuickPredicate(t, m, "quick-many-f", sort, boolean)
	const n = 20000
	args := make([]*SMTEnode, n)
	apps := make([]*SMTEnode, n)
	for i := range args {
		_, args[i] = smtEnodeConstant(t, m, uint32(40000+i), sort, 0)
		astTestMust(t, state.context.RegisterEnode(args[i], true))
		_, apps[i] = smtQuickApplicationNode(t, m, state.context, f, args[i], true)
	}
	q := &SMTQuickChecker{context: state.context}
	start := time.Now()
	for i := range args {
		node, err := q.congruent(f, []*SMTEnode{args[i]})
		astTestMust(t, err)
		if node != apps[i] {
			t.Fatal("congruent application", i)
		}
	}
	t.Logf("%d lookups among %d applications of one declaration: %v", n, n, time.Since(start))
}
