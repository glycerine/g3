package api

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func checkAfterBatchProof(s *APISimpleSolver, batch *OrderProofBatch) (lbool.LBool, error) {
	yes, err := batch.Prove()
	if err != nil {
		return lbool.Undef, err
	}
	if yes {
		return lbool.False, nil
	}
	return s.Check()
}

func TestRecentRefutedConjunctTruthTable(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	sort := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("p", sort))
	q := f.pin(f.c.Constant("q", sort))
	notP := f.pin(f.c.Not(p))
	notQ := f.pin(f.c.Not(q))
	and := f.pin(f.c.And(p, q))
	or := f.pin(f.c.Or(p, q))
	implies := f.pin(f.c.Implies(p, q))
	notImplies := f.pin(f.c.Not(implies))
	notOr := f.pin(f.c.Not(or))
	doubleNotP := f.pin(f.c.Not(notP))
	// Bits enumerate the four assignments to (p,q), starting at (false,false).
	masks := map[ast.ASTHandle]uint8{
		p: 0b1100, q: 0b1010, notP: 0b0011, notQ: 0b0101,
		and: 0b1000, or: 0b1110, implies: 0b1011, notImplies: 0b0100,
		notOr: 0b0001, doubleNotP: 0b1100,
	}
	required := map[[2]ast.ASTHandle]bool{
		{p, and}: true, {q, and}: true, {p, notImplies}: true,
		{notQ, notImplies}: true, {notP, notOr}: true, {notQ, notOr}: true,
		{doubleNotP, and}: true,
	}
	for old, oldMask := range masks {
		premise := f.pin(f.c.Not(old))
		var recent recentRefutations
		if recent.lookup(f.m, []ast.ASTHandle{premise, old}) {
			t.Fatal("empty history reported a proof")
		}
		// The premise and the whole old goal contradict by construction.
		recent.remember()
		for goal, mask := range masks {
			hit := recent.lookup(f.m, []ast.ASTHandle{premise, goal})
			if hit && mask&^oldMask != 0 {
				t.Fatalf("remembered %04b refuted satisfiable goal %04b", oldMask, mask)
			}
			if required[[2]ast.ASTHandle{old, goal}] && !hit {
				t.Fatalf("missed required conjunct %04b in goal %04b", oldMask, mask)
			}
		}
		// p and q can both hold once the old premise is gone.
		if recent.lookup(f.m, []ast.ASTHandle{and}) {
			t.Fatal("reused a signed conjunct after dropping the premise")
		}
	}
}

func TestOrderProofBatchReusesARefutedConjunct(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	sort := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("p", sort))
	q := f.pin(f.c.Constant("q", sort))
	notP := f.pin(f.c.Not(p))
	goal := f.pin(f.c.Not(f.pin(f.c.Implies(q, p))))
	astTestMust(t, s.Assert(p))
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(notP))
	answer, err := checkAfterBatchProof(s, batch)
	if err != nil || answer != lbool.False {
		t.Fatal("initial contradiction", answer, err)
	}
	astTestMust(t, s.Pop(1))
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(goal))
	if proved, err := batch.Prove(); err != nil || !proved {
		t.Fatal("the new goal requires the previously refuted literal", proved, err)
	}
	astTestMust(t, s.Reset())
	astTestMust(t, s.Assert(q))
	astTestMust(t, s.Assert(goal))
	answer, err = checkAfterBatchProof(s, batch)
	if err != nil || answer != lbool.True {
		t.Fatal("the changed premises allow q=true and p=false", answer, err)
	}
}

func BenchmarkRecentRefutedConjunct(b *testing.B) {
	c, s := orderSolver(b, `(declare-const p Bool)(declare-const q Bool)
		(assert p)(assert (not p))(assert (not (=> q p)))(assert (not q))`)
	m, err := c.Manager()
	astTestMust(b, err)
	forms, err := solver.Assertions(s.solver)
	astTestMust(b, err)
	for i, name := range []string{"Exact", "NegatedImplication", "Independent"} {
		b.Run(name, func(b *testing.B) {
			var recent recentRefutations
			if recent.lookup(m, forms[:2]) {
				b.Fatal("unexpected initial hit")
			}
			recent.remember()
			query := []ast.ASTHandle{forms[0], forms[i+1]}
			want := i != 2
			b.ReportAllocs()
			for b.Loop() {
				if recent.lookup(m, query) != want {
					b.Fatalf("wrong lookup for %s", name)
				}
			}
		})
	}
}

func TestOrderProofBatchReusesOnlyIdenticalPremises(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	b := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("recent-p", b))
	q := f.pin(f.c.Constant("recent-q", b))
	notP := f.pin(f.c.Not(p))
	check := func(want lbool.LBool) {
		t.Helper()
		got, err := checkAfterBatchProof(s, batch)
		if err != nil || got != want {
			t.Fatalf("got %v, want %v, error %v", got, want, err)
		}
	}
	astTestMust(t, s.Assert(p))
	for i := 0; i < 2; i++ {
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(notP))
		check(lbool.False)
		if r := batch.state.recent; r == nil || r.goals[0] != notP || r.next != 1 {
			t.Fatalf("expected one concrete result after check %d", i+1)
		}
		astTestMust(t, s.Pop(1))
	}
	astTestMust(t, s.Reset())
	astTestMust(t, s.Assert(q))
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(notP))
	check(lbool.True)
	model, err := s.Model()
	astTestMust(t, err)
	value, err := model.Evaluate(p, false)
	astTestMust(t, err)
	h, err := f.m.RefGet(value)
	astTestMust(t, err)
	if !f.m.IsFalse(h) {
		t.Fatal("counterexample should assign recent-p to false")
	}
	astTestMust(t, f.m.DelRef(&value))
	astTestMust(t, model.Close())
	astTestMust(t, s.Pop(1))
	astTestMust(t, s.AssertTracked(p, f.pin(f.c.Constant("recent-label", b))))
	astTestMust(t, s.Assert(notP))
	check(lbool.False)
	if batch.state.recent.next != 0 {
		t.Fatal("recorded a tracked or satisfiable check")
	}
	astTestMust(t, batch.Close())
	if _, err := batch.Prove(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed batch", err)
	}
}

func TestOrderProofBatchCheckHonorsResourceLimits(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	b := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("limited-p", b))
	astTestMust(t, s.Assert(p))
	astTestMust(t, s.Assert(f.pin(f.c.Not(p))))
	if got, err := checkAfterBatchProof(s, batch); err != nil || got != lbool.False || batch.state.recent == nil {
		t.Fatal("initial concrete contradiction", got, err)
	}
	// An identical query with an active limit must reach the original solver.
	limit := f.m.ResourceLimit()
	astTestMust(t, limit.Push(1))
	yes, err := batch.Prove()
	limit.Pop()
	if err != nil || yes || s.pendingRefutation != nil {
		t.Fatal("reused or scheduled a refutation under a limit", yes, err)
	}
}

func TestOrderProofBatchObservesOnlyTheActualCheck(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	b := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	st := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("observed-p", st))
	q := f.pin(f.c.Constant("observed-q", st))
	notP := f.pin(f.c.Not(p))
	tag := f.pin(f.c.Constant("observed-tag", st))
	prepare := func(t *testing.T) {
		t.Helper()
		yes, e := b.Prove()
		if yes || e != nil || s.pendingRefutation == nil {
			t.Fatalf("expected an eligible unproved query, got %v, %v", yes, e)
		}
	}
	concrete := func(t *testing.T, want lbool.LBool, assumptions ...ast.ASTHandle) {
		t.Helper()
		got, e := s.Check(assumptions...)
		if got != want || e != nil {
			t.Fatalf("check got %v, want %v, error %v", got, want, e)
		}
	}
	// q and not-p have a concrete model. Each change below creates a
	// different contradiction just for Check. Restore the original query
	// and require both an inconclusive Prove and an actual model.
	for _, mode := range []string{"assumptions", "tracked", "goal", "premise", "reset", "closed batch", "superseded batch"} {
		t.Run(mode, func(t *testing.T) {
			astTestMust(t, s.Reset())
			astTestMust(t, s.Push())
			astTestMust(t, s.Assert(q))
			astTestMust(t, s.Push())
			astTestMust(t, s.Assert(notP))
			prepare(t)
			switch mode {
			case "assumptions":
				concrete(t, lbool.False, p)
			case "tracked":
				astTestMust(t, s.AssertTracked(p, tag))
				concrete(t, lbool.False)
			case "goal":
				astTestMust(t, s.Pop(1))
				astTestMust(t, s.Push())
				astTestMust(t, s.Assert(f.pin(f.c.Not(q))))
				concrete(t, lbool.False)
			case "premise":
				astTestMust(t, s.Pop(2))
				astTestMust(t, s.Assert(p))
				astTestMust(t, s.Push())
				astTestMust(t, s.Assert(notP))
				concrete(t, lbool.False)
			case "reset":
				astTestMust(t, s.Reset())
				astTestMust(t, s.Assert(p))
				astTestMust(t, s.Assert(notP))
				concrete(t, lbool.False)
			case "closed batch":
				astTestMust(t, b.Close())
				if s.pendingRefutation != nil {
					t.Fatal("closed batch still registered")
				}
				b = s.BeginOrderProofBatch()
				concrete(t, lbool.True)
			case "superseded batch":
				other := s.BeginOrderProofBatch()
				if yes, e := other.Prove(); yes || e != nil || s.pendingRefutation == b.state.recent {
					t.Fatal("second batch did not supersede the first", yes, e)
				}
				astTestMust(t, b.Close())
				if s.pendingRefutation != other.state.recent {
					t.Fatal("closing first batch removed second")
				}
				astTestMust(t, other.Close())
				b = s.BeginOrderProofBatch()
				concrete(t, lbool.True)
			}
			astTestMust(t, s.Reset())
			astTestMust(t, s.Assert(q))
			astTestMust(t, s.Assert(notP))
			prepare(t)
			concrete(t, lbool.True)
		})
	}
}

// Each complete assignment fixes four independent variables. Exactly four
// signed queries contradict it. Repeat them in the same batch, then change
// the premises while preserving their count and the goal handles.
func TestOrderProofBatchTruthTable(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	b := f.pin(f.c.BoolSort())
	var literals [4][2]ast.ASTHandle
	for i := range literals {
		atom := f.pin(f.c.Constant("recent-table-"+strconv.Itoa(i), b))
		literals[i] = [2]ast.ASTHandle{f.pin(f.c.Not(atom)), atom}
	}
	for mask := 0; mask < 16; mask++ {
		astTestMust(t, s.Reset())
		for i, pair := range literals {
			astTestMust(t, s.Assert(pair[(mask>>i)&1]))
		}
		for pass := 0; pass < 2; pass++ {
			for i, pair := range literals {
				for sign, goal := range pair {
					astTestMust(t, s.Push())
					astTestMust(t, s.Assert(goal))
					conflict := ((mask >> i) & 1) != sign
					proved, err := batch.Prove()
					astTestMust(t, err)
					if proved != (pass == 1 && conflict) {
						t.Fatalf("assignment %04b, pass %d, variable %d, sign %d: proved %v", mask, pass, i, sign, proved)
					}
					if !proved {
						answer, err := s.Check()
						astTestMust(t, err)
						want := lbool.True
						if conflict {
							want = lbool.False
						}
						if answer != want {
							t.Fatalf("assignment %04b, variable %d, sign %d: got %v, want %v", mask, i, sign, answer, want)
						}
					}
					astTestMust(t, s.Pop(1))
				}
			}
		}
	}
}

func TestOrderProofBatchBoundsConcreteHistory(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	defer s.Close()
	batch := s.BeginOrderProofBatch()
	defer batch.Close()
	b := f.pin(f.c.BoolSort())
	var seen [9]ast.ASTHandle
	for i := range seen {
		v := f.pin(f.c.Constant("bounded-"+strconv.Itoa(i), b))
		astTestMust(t, s.Assert(v))
		seen[i] = f.pin(f.c.Not(v))
	}
	for _, goal := range seen {
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(goal))
		yes, err := batch.Prove()
		if err != nil || yes {
			t.Fatal("unexpected match on first visit", yes, err)
		}
		got, err := s.Check()
		if err != nil || got != lbool.False {
			t.Fatal(got, err)
		}
		astTestMust(t, s.Pop(1))
	}
	for i, goal := range seen {
		astTestMust(t, s.Push())
		astTestMust(t, s.Assert(goal))
		if b, err := batch.state.prove(1, true); err != nil || b {
			t.Fatal("used ordinary result at private budget 1", b, err)
		}
		yes, err := batch.Prove()
		if err != nil || yes != (i > 0) {
			t.Fatalf("recent goal %d proved %v, error %v", i, yes, err)
		}
		astTestMust(t, s.Pop(1))
	}
	// None of the nine goals belongs to this new, longer premise sequence.
	// The cap applies to the prefix, excluding the last assertion.
	for i := len(seen); i < maxRecentPremises; i++ {
		astTestMust(t, s.Assert(seen[0]))
	}
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(seen[1]))
	if _, err := batch.Prove(); err != nil || len(batch.state.recent.premises) != maxRecentPremises {
		t.Fatal("expected maximum-length prefix", err)
	}
	astTestMust(t, s.Assert(seen[2]))
	if _, err := batch.Prove(); err != nil || s.pendingRefutation != nil {
		t.Fatal("observed an oversized prefix", err)
	}
}

func TestOrderProofBatchRequiresOriginalAssertionVector(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	for _, mode := range []string{"preprocessed", "failed mutation"} {
		t.Run(mode, func(t *testing.T) {
			makeSolver := f.c.NewSimpleSolver
			if mode == "failed mutation" {
				makeSolver = f.c.NewSolver
			}
			s, err := makeSolver()
			astTestMust(t, err)
			defer s.Close()
			b := s.BeginOrderProofBatch()
			defer b.Close()
			p := f.pin(f.c.Constant("original-assertions", f.pin(f.c.BoolSort())))
			astTestMust(t, s.Assert(p))
			astTestMust(t, s.Assert(f.pin(f.c.Not(p))))
			if mode == "failed mutation" {
				if err := s.Pop(1); err == nil {
					t.Fatal("invalid pop succeeded")
				}
			}
			for i := 0; i < 2; i++ {
				if yes, err := b.Prove(); err != nil || yes || s.pendingRefutation != nil {
					t.Fatal("reused a solver with an incomplete assertion history", yes, err)
				}
				if answer, err := s.Check(); err != nil || answer != lbool.False {
					t.Fatal(answer, err)
				}
			}
		})
	}
}

func TestOrderProofBatchRejectsRecycledArenaSlots(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	st := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("slot-p", st))
	q := f.pin(f.c.Constant("slot-q", st))
	// Retain the declaration so deleting the first application frees only its slot.
	f.pin(f.m.MkNotDecl())
	old, err := f.m.MkNot(p)
	astTestMust(t, err)
	astTestMust(t, f.m.IncRef(old))
	var r recentRefutations
	if r.lookup(f.m, []ast.ASTHandle{p, old}) {
		t.Fatal("unexpected hit in new batch")
	}
	r.remember()
	astTestMust(t, f.m.DecRef(old))
	now := f.pin(f.m.MkNot(q))
	if old.Slot() != now.Slot() || now.Generation() == old.Generation() {
		t.Fatal("control requires recycled slot", old, now)
	}
	if r.lookup(f.m, []ast.ASTHandle{p, now}) {
		t.Fatal("reused the old goal after reclamation")
	}
}

func BenchmarkAppleRepeatedVerification(b *testing.B) {
	input := appleBenchmarkInput(b, "cf_live-42")
	for _, mode := range []string{"Concrete", "Batch"} {
		b.Run(mode, func(b *testing.B) {
			c, err := NewSMTContext()
			if err != nil {
				b.Fatal(err)
			}
			defer c.Close()
			inputSolver, err := c.NewSolver()
			if err != nil {
				b.Fatal(err)
			}
			defer inputSolver.Close()
			if err = inputSolver.LoadSMT2String(input); err != nil {
				b.Fatal(err)
			}
			roots, err := solver.Assertions(inputSolver.solver)
			if err != nil || len(roots) != 2 {
				b.Fatal("expected base and goal", len(roots), err)
			}
			s, err := c.NewSolver()
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			if err = s.Assert(roots[0]); err != nil {
				b.Fatal(err)
			}
			batch := s.BeginOrderProofBatch()
			defer batch.Close()
			run := func() {
				if err := s.Push(); err != nil {
					b.Fatal(err)
				}
				if err := s.Assert(roots[1]); err != nil {
					b.Fatal(err)
				}
				var result lbool.LBool
				var err error
				if mode == "Batch" {
					result, err = checkAfterBatchProof(s, batch)
				} else {
					result, err = s.Check()
				}
				if err != nil || result != lbool.False {
					b.Fatal("expected UNSAT", result, err)
				}
				if err := s.Pop(1); err != nil {
					b.Fatal(err)
				}
			}
			run()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				run()
			}
			b.StopTimer()
		})
	}
}
