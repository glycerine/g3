package nlsat

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

func clauseStoreFixture(t *testing.T) (*Evaluator, *clauseStore) {
	t.Helper()
	e, s, _ := clauseStoreFixtureWithLimit(t)
	return e, s
}
func clauseStoreFixtureWithLimit(t *testing.T) (*Evaluator, *clauseStore, *control.Reslimit) {
	return clauseStoreFixtureConfigured(t, false)
}
func clauseStoreFixtureConfigured(t *testing.T, factorization bool) (*Evaluator, *clauseStore, *control.Reslimit) {
	t.Helper()
	am, _, limit := assignmentFixtureWithLimit(t)
	q, err := am.RationalManager()
	assignmentMust(t, err)
	var fc polynomial.UPolynomialFactorizationConfig
	if factorization {
		primes, e := primegen.NewGenerator(q.Budget())
		assignmentMust(t, e)
		t.Cleanup(func() { assignmentMust(t, primes.Close()) })
		fc.Primes = primes
	}
	pm, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{Factorization: fc, Core: polynomial.MultivariatePolynomialConfig{Integer: q.Integers(), Rationals: q, Checkpoint: limit.Inc, Cooperation: func() error { return nil }}})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, pm.CloseChecked()) })
	a, err := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: q.Integers().RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: diagnostic.DebugBuild}})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, a.Close()) })
	pc, err := polynomial.NewPolynomialCache(pm)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, pc.Close()) })
	f, err := newAtomFactory(pm, pc, a)
	assignmentMust(t, err)
	t.Cleanup(func() {
		assignmentMust(t, f.cleanup())
		for _, atom := range f.bools.atoms {
			if atom != nil {
				assignmentMust(t, f.deleteAtom(atom))
			}
		}
		assignmentMust(t, f.close())
	})
	as, err := newAssumptionManager(a)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, as.close()) })
	s, err := newClauseStore(f, as)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, s.close()) })
	assignmentMust(t, s.mkTrueBooleanVariable())
	for i := uint32(0); i < 2; i++ {
		x, err := s.mkVar(i == 0)
		assignmentMust(t, err)
		if x != i {
			t.Fatal("source arithmetic-variable order", x, i)
		}
	}
	assignment, err := NewAssignment(am)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, assignment.Close()) })
	e, err := NewEvaluator(assignment, pm, a)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, e.Close()) })
	return e, s, limit
}

func TestClauseStoreArithmeticMetadataTrueClauseAndOwnerLifetimes(t *testing.T) {
	e, s := clauseStoreFixture(t)
	for x := uint32(2); x < 30; x++ {
		got, err := s.mkVar(x%3 == 0)
		assignmentMust(t, err)
		if got != x {
			t.Fatal("actual polynomial/metadata variable identity", got, x)
		}
	}
	n, err := e.core.NumVars()
	assignmentMust(t, err)
	if n != 30 || len(s.vars.isInt) != 30 || len(s.vars.watches) != 30 || len(s.vars.infeasible) != 30 || len(s.vars.equalities) != 30 || len(s.vars.perm) != 30 || len(s.vars.inv) != 30 {
		t.Fatal("source parallel arithmetic extents")
	}
	for x := uint32(0); x < n; x++ {
		wantInt := x%3 == 0
		if x < 2 {
			wantInt = x == 0
		}
		if s.vars.isInt[x] != wantInt || s.vars.perm[x] != x || s.vars.inv[x] != x || s.vars.infeasible[x] != nil || s.vars.equalities[x] != nil || len(s.vars.watches[x]) != 0 {
			t.Fatal("source arithmetic-variable initialization", x)
		}
	}
	if len(s.originals) != 1 || s.originals[0].ID() != 0 || !reflect.DeepEqual(s.originals[0].Literals(), []sat.Literal{TrueLiteral}) || !reflect.DeepEqual(s.atoms.bools.watches[0], s.originals) || s.atoms.bools.values[0] != lbool.Undef {
		t.Fatal("true clause was not constructed and attached independently of propagation")
	}
	if err := s.atoms.close(); err != ErrAtomCacheLive {
		t.Fatal("closed borrowed atom owner", err)
	}
	if err := s.assumptions.close(); err != ErrAssumptionLive {
		t.Fatal("closed borrowed assumption owner", err)
	}
	copy := *s
	if _, err := copy.mkVar(false); err != ErrAtomCacheOwner {
		t.Fatal("copied clause store accepted work", err)
	}
	if _, err := newClauseStore(s.atoms, s.assumptions); err != arena.ErrConfig {
		t.Fatal("duplicated authoritative metadata owner", err)
	}
}

type clauseSortModel struct {
	b           sat.BoolVar
	arithmetic  bool
	max, degree uint32
	equality    bool
}

func clauseStoreModelAtoms(t *testing.T, e *Evaluator, s *clauseStore) []clauseSortModel {
	t.Helper()
	var model []clauseSortModel
	for i := 0; i < 3; i++ {
		b, err := s.atoms.mkBoolVar()
		assignmentMust(t, err)
		model = append(model, clauseSortModel{b: b})
	}
	for x := uint32(0); x < 2; x++ {
		for degree := uint32(1); degree <= 3; degree++ {
			p, err := e.core.MkVariable(x, degree)
			assignmentMust(t, err)
			assignmentMust(t, e.core.IncRef(p))
			t.Cleanup(func() { assignmentMust(t, e.core.DecRef(p)) })
			for _, kind := range []AtomKind{AtomEQ, AtomLT, AtomRootEQ, AtomRootGE} {
				var b sat.BoolVar
				if kind < AtomRootEQ {
					b, err = s.atoms.mkInequality(kind, []polynomial.Polynomial{p}, []bool{degree%2 == 0})
				} else {
					b, err = s.atoms.mkRoot(kind, x, 1, p)
				}
				assignmentMust(t, err)
				model = append(model, clauseSortModel{b, true, x, degree, kind == AtomEQ || kind == AtomRootEQ})
				assignmentMust(t, s.atoms.incRef(b)) // the retained caller's literal lease.
				t.Cleanup(func() { assignmentMust(t, s.atoms.decRef(b)) })
			}
		}
	}
	return model
}

func TestClauseStoreIndependentLiteralOrderWatchesAndReferenceMultiplicity(t *testing.T) {
	e, s := clauseStoreFixture(t)
	model := clauseStoreModelAtoms(t, e, s)
	metadata := map[sat.BoolVar]clauseSortModel{}
	for _, item := range model {
		metadata[item.b] = item
	}
	random := rand.New(rand.NewPCG(816, 621))
	var clauses []*Clause
	wantRefs := map[sat.BoolVar]uint32{}
	for world := 0; world < 100; world++ {
		lits := make([]sat.Literal, 1+random.IntN(15))
		for i := range lits {
			lits[i] = sat.MakeLiteral(model[random.IntN(len(model))].b, random.IntN(2) != 0)
			wantRefs[lits[i].Variable()]++
		}
		want := append([]sat.Literal(nil), lits...)
		sort.Slice(want, func(i, j int) bool {
			a, b := metadata[want[i].Variable()], metadata[want[j].Variable()]
			if a.arithmetic != b.arithmetic {
				return !a.arithmetic
			}
			if a.arithmetic {
				if a.max != b.max {
					return a.max < b.max
				}
				if a.degree != b.degree {
					return a.degree < b.degree
				}
				if a.equality != b.equality {
					return !a.equality
				}
			}
			return uint32(want[i]) < uint32(want[j])
		})
		c, err := s.mkClause(lits, world%2 != 0, nil)
		assignmentMust(t, err)
		if !reflect.DeepEqual(c.Literals(), want) || c.IsLearned() != (world%2 != 0) || c.ID() != uint32(world+1) {
			t.Fatal("independent literal order/header", world, c.Literals(), want)
		}
		wantMax := uint32(NullVariable)
		for _, l := range lits {
			item := metadata[l.Variable()]
			if item.arithmetic && (wantMax == NullVariable || item.max > wantMax) {
				wantMax = item.max
			}
		}
		var watch []*Clause
		if wantMax == NullVariable {
			b := sat.BoolVar(0)
			for _, l := range lits {
				if l.Variable() > b {
					b = l.Variable()
				}
			}
			watch = s.atoms.bools.watches[b]
		} else {
			watch = s.vars.watches[wantMax]
		}
		if len(watch) == 0 || watch[len(watch)-1] != c {
			t.Fatal("actual maximal-variable watch", world)
		}
		clauses = append(clauses, c)
	}
	for _, item := range model {
		if item.arithmetic && arithmeticAtomHeader(s.atoms.bools.atoms[item.b]).refs != wantRefs[item.b]+1 {
			t.Fatal("duplicate/polarity reference multiplicity", item.b)
		}
	}
	wantOriginal := append([]*Clause(nil), s.originals...)
	wantLearned := append([]*Clause(nil), s.learned...)
	wantWatches := make([][]*Clause, len(s.vars.watches))
	for i, watch := range s.vars.watches {
		wantWatches[i] = append([]*Clause(nil), watch...)
	}
	sameSequence := func(a, b []*Clause) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	random.Shuffle(len(clauses), func(i, j int) { clauses[i], clauses[j] = clauses[j], clauses[i] })
	for _, c := range clauses {
		assignmentMust(t, s.deleteClause(c))
		without := func(values []*Clause) []*Clause {
			var result []*Clause
			for _, old := range values {
				if old != c {
					result = append(result, old)
				}
			}
			return result
		}
		wantOriginal, wantLearned = without(wantOriginal), without(wantLearned)
		if !sameSequence(s.originals, wantOriginal) || !sameSequence(s.learned, wantLearned) {
			t.Fatal("clause-list erasure changed surviving source order")
		}
		for i := range wantWatches {
			wantWatches[i] = without(wantWatches[i])
			if !sameSequence(s.vars.watches[i], wantWatches[i]) {
				t.Fatal("watch erasure changed surviving source order", i)
			}
		}
	}
	if len(s.originals) != 1 || len(s.learned) != 0 {
		t.Fatal("clause list retirement")
	}
	for _, watch := range s.vars.watches {
		if len(watch) != 0 {
			t.Fatal("arithmetic watch survived retirement")
		}
	}
	for _, item := range model {
		if item.arithmetic && arithmeticAtomHeader(s.atoms.bools.atoms[item.b]).refs != 1 {
			t.Fatal("clause retirement lost caller lease", item.b)
		}
	}
}

func TestClauseStoreAssumptionsPartialINCAndActualDeletionContinuation(t *testing.T) {
	atomDebugFixture(t)
	e, s := clauseStoreFixture(t)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
	b, err := s.atoms.mkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	assignmentMust(t, s.atoms.incRef(b))
	t.Cleanup(func() { assignmentMust(t, s.atoms.decRef(b)) })
	as, err := s.assumptions.mkLeaf(AttachedAssumption(&AssumptionAttachment{}))
	assignmentMust(t, err)
	assignmentMust(t, s.assumptions.incRef(as))
	t.Cleanup(func() { assignmentMust(t, s.assumptions.decRef(as)) })
	_, err = s.mkClause([]sat.Literal{sat.MakeLiteral(b, false), sat.MakeLiteral(99999, false)}, false, as)
	if !errors.Is(err, arena.ErrBounds) || s.pending == nil || len(s.pending.acquired) != 1 || as.refs != 1 || arithmeticAtomHeader(s.atoms.bools.atoms[b]).refs != 2 {
		t.Fatal("qualified later-literal refusal did not preserve exact INC prefix", err)
	}
	assignmentMust(t, s.cleanup())
	if arithmeticAtomHeader(s.atoms.bools.atoms[b]).refs != 1 || as.refs != 1 {
		t.Fatal("partial construction cleanup dropped unacquired references")
	}
	c, err := s.mkClause([]sat.Literal{sat.MakeLiteral(b, false), sat.MakeLiteral(b, true), sat.MakeLiteral(b, false)}, true, as)
	assignmentMust(t, err)
	if arithmeticAtomHeader(s.atoms.bools.atoms[b]).refs != 4 || as.refs != 2 {
		t.Fatal("owned literal/assumption multiplicity")
	}
	assignmentMust(t, s.deleteClause(c))
	if arithmeticAtomHeader(s.atoms.bools.atoms[b]).refs != 1 || as.refs != 1 {
		t.Fatal("normal retirement decremented references incorrectly")
	}
}

func TestClauseStoreColdAllocationConsumesIDsAndRecovers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 32; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			_, s := clauseStoreFixture(t)
			// A different clause extent forces a cold allocator bucket.
			lits := make([]sat.Literal, 80)
			for i := range lits {
				lits[i] = TrueLiteral
			}
			before := s.ids.Range()
			s.atoms.ineq.budget.FailAfter(stop)
			c, err := s.mkClause(lits, false, nil)
			s.atoms.ineq.budget.FailAfter(-1)
			if s.ids.Range() != before+1 {
				t.Fatal("clause ID was not consumed before allocation")
			}
			assignmentMust(t, s.cleanup())
			var physical umemory.Failure
			if err != nil && !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &physical) && physical.Kind == umemory.FailureMemout) {
				t.Fatal("unexpected allocation refusal", err)
			}
			if err != nil && c != nil {
				t.Fatal("failed clause returned a published owner")
			}
			retry, err := s.mkClause(lits, false, nil)
			assignmentMust(t, err)
			if c == nil && retry.ID() != before+1 {
				t.Fatal("cleanup recycled an ID without a constructed clause", retry.ID(), before)
			}
			success = c != nil
		})
		if success {
			break
		}
	}
	if !success {
		t.Fatal("allocation ladder never reached success")
	}
}

func clauseStoreRightDAG(t *testing.T, s *clauseStore, depth int) *AssumptionSet {
	t.Helper()
	root, err := s.assumptions.mkLeaf(Assumption{})
	assignmentMust(t, err)
	for i := 0; i < depth; i++ {
		left, err := s.assumptions.mkLeaf(Assumption{})
		assignmentMust(t, err)
		root, err = s.assumptions.mkJoin(left, root)
		assignmentMust(t, err)
	}
	return root
}

func TestClauseStoreConsumedAssumptionDECAfterRootRetirement(t *testing.T) {
	success, afterRoot := false, false
	for stop := int64(0); stop < 12; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			_, s := clauseStoreFixture(t)
			root := clauseStoreRightDAG(t, s, 12)
			c, err := s.mkClause([]sat.Literal{TrueLiteral}, false, root)
			assignmentMust(t, err)
			s.atoms.ineq.budget.FailAfter(stop)
			err = s.deleteClause(c)
			s.atoms.ineq.budget.FailAfter(-1)
			if err != nil {
				if !errors.Is(err, arena.ErrInjectedFailure) || s.retiring == nil || !s.retiring.asmPending || s.retiring.asOwned || root.refs != 0 || s.assumptions.mode != 2 || s.retiring.asmReceipt.serial != s.assumptions.activeDelete {
					t.Fatal("stopped assumption DEC lost actual invocation authority", err)
				}
				if root.closed {
					afterRoot = true
				}
			}
			assignmentMust(t, s.cleanup())
			if len(s.assumptions.live) != 0 || s.assumptions.activeDelete != 0 || s.owned[c] != nil {
				t.Fatal("assumption/clause continuation retained ownership")
			}
			success = err == nil
		})
		if success {
			break
		}
	}
	if !success || !afterRoot {
		t.Fatal("did not reach success and a child stop after the root was actually freed", success, afterRoot)
	}
}

func TestClauseStoreStaleAssumptionReceiptPreservesNewerDeletion(t *testing.T) {
	_, s := clauseStoreFixture(t)
	root := clauseStoreRightDAG(t, s, 12)
	c, err := s.mkClause([]sat.Literal{TrueLiteral}, false, root)
	assignmentMust(t, err)
	s.atoms.ineq.budget.FailAfter(0)
	err = s.deleteClause(c)
	s.atoms.ineq.budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || s.retiring == nil || !s.retiring.asmPending {
		t.Fatal("first deletion did not stop", err)
	}
	receipt := s.retiring.asmReceipt
	assignmentMust(t, s.assumptions.cleanup()) // the actual callee owner acknowledges this invocation.
	independent := clauseStoreRightDAG(t, s, 64)
	assignmentMust(t, s.assumptions.incRef(independent))
	s.atoms.ineq.budget.FailAfter(0)
	err = s.assumptions.decRef(independent)
	s.atoms.ineq.budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || s.assumptions.mode != 2 || s.assumptions.activeDelete == receipt.serial {
		t.Fatal("new deletion did not qualify an independent pending invocation", err)
	}
	m := s.assumptions
	serial, current, child, childIndex := m.activeDelete, m.current, m.childToPush, m.childIndex
	todo := append([]*AssumptionSet(nil), m.todo...)
	refs := map[*AssumptionSet]uint32{}
	for d := range m.live {
		refs[d] = d.refs
	}
	stats, used := m.allocator.RuntimeOwner().Stats(), m.budget.Used()
	assignmentMust(t, m.cleanupDeletion(&receipt))
	if m.allocator.RuntimeOwner().Stats() != stats || m.budget.Used() != used {
		t.Fatal("stale acknowledgement changed the newer deletion's memory state")
	}
	assignmentMust(t, s.cleanup())
	if m.mode != 2 || m.activeDelete != serial || m.current != current || m.childToPush != child || m.childIndex != childIndex || !reflect.DeepEqual(m.todo, todo) || len(m.live) != len(refs) {
		t.Fatal("clause cleanup adopted a newer independent deletion")
	}
	for d, count := range refs {
		if d.refs != count || d.closed {
			t.Fatal("newer DAG node changed under stale clause cleanup")
		}
	}
	assignmentMust(t, m.cleanup())
}

func TestClauseStoreConsumedAtomDECAndIDRetirementOrder(t *testing.T) {
	atomDebugFixture(t)
	e, s := clauseStoreFixture(t)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
	b, err := s.atoms.mkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	c, err := s.mkClause([]sat.Literal{sat.MakeLiteral(b, false)}, false, nil)
	assignmentMust(t, err)
	a := s.atoms.bools.atoms[b]
	id := c.ID()
	// This qualified invalid watch reaches the source's empty-watch assertion;
	// it does not claim normal arithmetic clauses use Boolean watch vectors.
	s.atoms.bools.watches[b] = []*Clause{c}
	err = s.deleteClause(c)
	want := ErrAtomCacheLive
	if diagnostic.DebugBuild {
		want = diagnostic.ErrDebugAssertion
	}
	if !errors.Is(err, want) || s.retiring == nil || !s.retiring.literalConsumed || s.retiring.atomPending != a || s.atoms.retiring != a || arithmeticAtomHeader(a).refs != 0 || s.retiring.attached || !reflect.DeepEqual(s.ids.FreeIDs(), []uint32{id}) {
		t.Fatal("atom DEC did not preserve detach/recycle/consumed-DEC order", err)
	}
	s.atoms.bools.watches[b] = nil
	assignmentMust(t, s.cleanup())
	if s.atoms.bools.atoms[b] != nil || s.owned[c] != nil {
		t.Fatal("atom/clause continuation did not retire its actual owners")
	}
	x, err := s.mkClause([]sat.Literal{TrueLiteral}, false, nil)
	assignmentMust(t, err)
	y, err := s.mkClause([]sat.Literal{TrueLiteral}, false, nil)
	assignmentMust(t, err)
	if x.ID() != id || y.ID() == id {
		t.Fatal("continuation replayed clause ID recycling", x.ID(), y.ID())
	}
}

func TestClauseStoreInputAssumptionLeafTransferAndRefusals(t *testing.T) {
	success := false
	leafWithoutClause := false
	for stop := int64(0); stop < 16; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			_, s := clauseStoreFixture(t)
			assumption := AttachedAssumption(&AssumptionAttachment{})
			lits := make([]sat.Literal, 80) // a cold class, beyond the true unit's already-warm bucket.
			for i := range lits {
				lits[i] = TrueLiteral
			}
			s.atoms.ineq.budget.FailAfter(stop)
			err := s.mkInputClause(lits, assumption)
			s.atoms.ineq.budget.FailAfter(-1)
			if err != nil && s.inputLeaf.leaf != nil && s.inputLeaf.leaf.refs == 0 {
				leafWithoutClause = true
			}
			assignmentMust(t, s.cleanup())
			if err != nil {
				var physical umemory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &physical) && physical.Kind == umemory.FailureMemout) {
					t.Fatal("unexpected input clause refusal", err)
				}
				if len(s.assumptions.live) != 0 || len(s.originals) != 1 || s.inputLeaf.owner != nil {
					t.Fatal("failed input clause leaked a zero-reference leaf")
				}
			} else {
				c := s.originals[len(s.originals)-1]
				if c.assumptions == nil || c.assumptions.refs != 1 || c.assumptions.value != assumption || len(s.assumptions.live) != 1 {
					t.Fatal("input clause did not acquire its concrete assumption leaf")
				}
				assignmentMust(t, s.deleteClause(c))
				if len(s.assumptions.live) != 0 {
					t.Fatal("clause retirement retained assumption leaf")
				}
				success = true
			}
		})
		if success {
			break
		}
	}
	if !success || !leafWithoutClause {
		t.Fatal("input campaign did not qualify success and the leaf-before-clause refusal", success, leafWithoutClause)
	}
}

func TestClauseStoreSourceDegreeQueriesAndFreshMetadataGuard(t *testing.T) {
	e, s := clauseStoreFixture(t)
	var lits []sat.Literal
	for _, term := range []evaluatorTerm{{1, 5, 0}, {1, 0, 1}} {
		p := evaluatorPolynomial(t, e, term)
		b, err := s.atoms.mkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false})
		assignmentMust(t, err)
		lits = append(lits, sat.MakeLiteral(b, false))
	}
	d, err := s.clauseDegree(lits)
	assignmentMust(t, err)
	if d != 5 {
		t.Fatal("source degree query incorrectly filtered lower-maximal-variable atoms", d)
	}
	_, f := atomFactoryFixture(t) // this polynomial manager already has two actual variables.
	m, err := newAssumptionManager(f.ineq.allocator)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, m.close()) })
	used := f.ineq.budget.Used()
	if _, err := newClauseStore(f, m); err != arena.ErrConfig || f.clauses != nil || m.clauses != nil || f.ineq.budget.Used() != used {
		t.Fatal("synthesized metadata for an already-used polynomial manager", err)
	}
}

func TestClauseStoreSortsArithmeticWatchesByDegreeAndOriginalIndex(t *testing.T) {
	e, s := clauseStoreFixture(t)
	degrees := []uint32{3, 1, 2, 1}
	clauses := make([]*Clause, len(degrees))
	for i, degree := range degrees {
		p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, degree})
		b, err := s.atoms.mkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false})
		assignmentMust(t, err)
		assignmentMust(t, s.mkInputClause([]sat.Literal{sat.MakeLiteral(b, false)}, Assumption{}))
		clauses[i] = s.originals[len(s.originals)-1]
	}
	assignmentMust(t, s.sortWatchedClauses())
	watched := s.vars.watches[1]
	want := []*Clause{clauses[1], clauses[3], clauses[2], clauses[0]}
	if len(watched) != len(want) {
		t.Fatal("sorted watch count", len(watched), len(want))
	}
	for i := range want {
		if watched[i] != want[i] {
			t.Fatal("sorted arithmetic watches", i, watched[i].ID(), want[i].ID())
		}
	}
}

func TestClauseStoreInputLiteralIdentityAndNullAssumption(t *testing.T) {
	_, s := clauseStoreFixture(t)
	inputs := []sat.Literal{TrueLiteral, TrueLiteral}
	var clauses []*Clause
	for i := range inputs {
		assignmentMust(t, s.mkInputClause([]sat.Literal{inputs[i]}, literalAssumption(&inputs[i])))
		c := s.originals[len(s.originals)-1]
		if c.assumptions == nil || c.assumptions.value.InputLiteral() != &inputs[i] || c.assumptions.refs != 1 {
			t.Fatal("input assumption lost original element identity")
		}
		clauses = append(clauses, c)
	}
	if clauses[0].assumptions.value == clauses[1].assumptions.value {
		t.Fatal("equal input values collapsed distinct assumption identities")
	}
	assignmentMust(t, s.mkInputClause([]sat.Literal{TrueLiteral}, Assumption{}))
	if s.originals[len(s.originals)-1].assumptions != nil || len(s.assumptions.live) != 2 {
		t.Fatal("null assumption constructed a leaf")
	}
	for _, c := range clauses {
		assignmentMust(t, s.deleteClause(c))
	}
	if len(s.assumptions.live) != 0 {
		t.Fatal("input-literal assumption leaves survived their clauses")
	}
}

func TestClauseStoreRefusedLeafAndClausePreserveOlderPhysicalPrefix(t *testing.T) {
	// Actual TLS services qualify the allocator's retained-physical-block path.
	m := assumptionFixtureWithProfile(t, true)
	b := m.budget
	q, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: m.allocator.RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, q.Close()) })
	pm, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{Core: polynomial.MultivariatePolynomialConfig{Integer: q.Manager.Integers(), Rationals: q.Manager, Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }}})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, pm.CloseChecked()) })
	pc, err := polynomial.NewPolynomialCache(pm)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, pc.Close()) })
	f, err := newAtomFactory(pm, pc, m.allocator)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, f.close()) })
	s, err := newClauseStore(f, m)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, s.close()) })
	assignmentMust(t, s.mkTrueBooleanVariable())
	runtime := m.allocator.RuntimeOwner()
	assignmentMust(t, runtime.SetMaxSize(1))
	var older umemory.SmallAllocatorAllocationOutcome
	_, started, err := m.allocator.AllocateWithOutcomeStarted(1000001, &older)
	if err == nil || !started || !m.allocator.MatchesPendingAllocationOutcome(&older) {
		t.Fatal("older physical prefix was not qualified", err)
	}
	stats, used := runtime.Stats(), b.Used()
	before := s.ids.Range()
	err = s.mkInputClause([]sat.Literal{TrueLiteral}, AttachedAssumption(&AssumptionAttachment{}))
	if !errors.Is(err, umemory.ErrAllocatorPending) || s.ids.Range() != before {
		t.Fatal("leaf refusal progressed into clause ID allocation", err)
	}
	assignmentMust(t, s.cleanup())
	if !m.allocator.MatchesPendingAllocationOutcome(&older) || runtime.Stats() != stats || b.Used() != used {
		t.Fatal("refused leaf cleanup changed independently older work")
	}
	_, err = s.mkClause([]sat.Literal{TrueLiteral}, false, nil)
	if !errors.Is(err, umemory.ErrAllocatorPending) || s.ids.Range() != before+1 {
		t.Fatal("clause allocator refusal lost its consumed ID", err)
	}
	assignmentMust(t, s.cleanup())
	if !m.allocator.MatchesPendingAllocationOutcome(&older) || runtime.Stats() != stats || b.Used() != used {
		t.Fatal("refused clause cleanup changed independently older work")
	}
	assignmentMust(t, m.allocator.CleanupPending())
	assignmentMust(t, runtime.SetMaxSize(0))
}
