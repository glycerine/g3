package sat

// Source: src/sat/sat_scc.cpp and src/sat/sat_elim_eqs.cpp.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func satSCCFixture(t *testing.T, n uint32) (*satSCC, *satSolverClauseLifecycle) {
	t.Helper()
	l, p, _, _ := satLifecycleFixture(t, n)
	local, e := p.config.context.NewValue()
	satOK(t, e)
	c, e := newSATSCC(l, &local)
	satOK(t, e)
	satOK(t, p.config.context.Del(&local))
	t.Cleanup(func() { satOK(t, c.Close()) })
	return c, l
}
func TestSATSCCDefaultEquivalenceAndActualConverter(t *testing.T) {
	c, l := satSCCFixture(t, 3)
	p := l.propagation
	satOK(t, l.mkBinClause(1, 2, false))
	satOK(t, l.mkBinClause(3, 0, false))
	n, e := c.run()
	satOK(t, e)
	if n != 1 || c.numElim != 1 {
		t.Fatal(n, c.numElim)
	}
	v, e := p.variables.wasEliminated(1)
	satOK(t, e)
	if !v {
		t.Fatal("actual eliminated column")
	}
	if p.converter.Size() != 1 {
		t.Fatal("actual converter entry", p.converter.Size())
	}
	r, e := p.converter.EntryAt(0)
	satOK(t, e)
	k, vv, e := p.converter.KindAndVar(r)
	satOK(t, e)
	if k != satModelElimVar || vv != 1 {
		t.Fatal(k, vv)
	}
	n, e = c.run()
	satOK(t, e)
	if n != 0 || p.converter.Size() != 1 {
		t.Fatal(n, p.converter.Size())
	}
}

func TestSATSCCInvariantUsesSourceConditionalAssertionTag(t *testing.T) {
	g, _ := satGCFixture(t, 3)
	l := g.lifecycle()
	ctx := l.propagation.config.context
	params, err := ctx.NewValue()
	satOK(t, err)
	c, err := newSATSCC(l, &params)
	satOK(t, ctx.Del(&params))
	satOK(t, err)
	t.Cleanup(func() { satOK(t, c.Close()) })
	debug, _, _ := satIntegrityActionFixture(t, "")
	before := l.propagation.limit.Steps
	_, err = c.run()
	satOK(t, err)
	baselineChecks := l.propagation.limit.Steps - before
	satOK(t, debug.EnableDebug("scc_bug"))
	before = l.propagation.limit.Steps
	_, err = c.run()
	satOK(t, err)
	want := baselineChecks
	if satStorageSourceDebug {
		want += 2 // CASSERT before SCC search and after equivalence elimination.
	}
	if got := l.propagation.limit.Steps - before; got != want {
		t.Fatalf("enabled process assertion consumed %d checks, want %d", got, want)
	}
}
func TestSATSCCExternalRootAndContradiction(t *testing.T) {
	for _, ext := range []bool{false, true} {
		c, l := satSCCFixture(t, 2)
		p := l.propagation
		if ext {
			satOK(t, p.variables.setExternal(BoolVar(1), true))
		}
		satOK(t, l.mkBinClause(1, 2, false))
		satOK(t, l.mkBinClause(3, 0, false))
		n, e := c.run()
		satOK(t, e)
		if n != 1 {
			t.Fatal(n)
		}
		root := BoolVar(0)
		eliminated := BoolVar(1)
		if ext {
			root, eliminated = 1, 0
		}
		yes, e := p.variables.wasEliminated(eliminated)
		satOK(t, e)
		no, e := p.variables.wasEliminated(root)
		satOK(t, e)
		if !yes || no {
			t.Fatal(ext, yes, no)
		}
	}
	c, l := satSCCFixture(t, 1)
	satOK(t, l.mkBinClause(1, 1, false))
	satOK(t, l.mkBinClause(0, 0, false))
	n, e := c.run()
	satOK(t, e)
	if n != 0 || !l.propagation.inconsistent || l.propagation.converter.Size() != 0 {
		t.Fatal(n, l.propagation.inconsistent)
	}
}
func TestSATElimEqsAllClauseAritiesAndModels(t *testing.T) {
	for _, profile := range [3][2]bool{{false, false}, {true, false}, {true, true}} {
		learned, frozen := profile[0], profile[1]
		l, p, _, _ := satLifecycleFixture(t, 5)
		roots := []Literal{0, 0, 4, 6, 8}
		for _, ls := range [][]Literal{{0, 2, 4}, {0, 2, 4, 6}, {0, 2, 4, 6, 8}, {0, 3, 4}} {
			r, e := p.clauses.MkClause(ls, learned)
			satOK(t, e)
			if frozen {
				satOK(t, p.clauses.Freeze(r))
			} else {
				satOK(t, l.attachClause(r))
			}
			satOK(t, l.pushClause(learned, r))
		}
		e, er := newSATElimEqs(l)
		satOK(t, er)
		t.Cleanup(func() { satOK(t, e.Close()) })
		satOK(t, e.run(roots, []BoolVar{1}))
		vec := l.Original
		if learned {
			vec = l.learned
		}
		if len(vec) != 2 {
			t.Fatal("retain only arity3/4", len(vec))
		}
		for i, r := range vec {
			n, er := p.clauses.Size(r)
			satOK(t, er)
			if n != uint32(i+3) {
				t.Fatal(n)
			}
			for j := uint32(0); j < n; j++ {
				lit, er := p.clauses.Literal(r, j)
				satOK(t, er)
				want := Literal(0)
				if j > 0 {
					want = Literal(2 * (j + 1))
				}
				if lit != want {
					t.Fatal(lit, j)
				}
			}
		}
		if p.converter.Size() != 1 {
			t.Fatal(p.converter.Size())
		}
	}
}

func TestSATSCCIndependentBoundedImplicationGraphs(t *testing.T) {
	edges := [6][2]uint32{{0, 1}, {0, 2}, {1, 0}, {1, 2}, {2, 0}, {2, 1}}
	for bits := uint32(0); bits < 64; bits++ {
		c, l := satSCCFixture(t, 3)
		var reach [3][3]bool
		for i := 0; i < 3; i++ {
			reach[i][i] = true
		}
		for j, edge := range edges {
			if bits&(1<<uint(j)) != 0 {
				reach[edge[0]][edge[1]] = true
				satOK(t, l.mkBinClause(Literal(edge[0]*2+1), Literal(edge[1]*2), false))
			}
		}
		for k := 0; k < 3; k++ {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					reach[i][j] = reach[i][j] || (reach[i][k] && reach[k][j])
				}
			}
		}
		components := uint32(0)
		for i := 0; i < 3; i++ {
			first := true
			for j := 0; j < i; j++ {
				if reach[i][j] && reach[j][i] {
					first = false
				}
			}
			if first {
				components++
			}
		}
		n, e := c.run()
		satOK(t, e)
		if n != 3-components {
			t.Fatal(bits, n, components)
		}
		for mask := uint32(0); mask < 8; mask++ {
			model := Model{lbool.False, lbool.False, lbool.False}
			for i := uint32(0); i < 3; i++ {
				if mask&(1<<i) != 0 {
					model[i] = lbool.True
				}
			}
			for i := uint32(0); i < 3; i++ {
				el, er := l.propagation.variables.wasEliminated(BoolVar(i))
				satOK(t, er)
				if el {
					model[i] = lbool.Undef
				}
			}
			satOK(t, l.propagation.converter.Apply(model))
			for j, edge := range edges {
				if bits&(1<<uint(j)) != 0 && reach[edge[0]][edge[1]] && reach[edge[1]][edge[0]] && model[edge[0]] != model[edge[1]] {
					t.Fatal("converter failed equivalence", bits, mask, edge, model)
				}
			}
		}
	}
}
func TestSATElimEqsEmptyUnitUnaffectedAndExternal(t *testing.T) {
	for _, empty := range []bool{false, true} {
		l, p, _, _ := satLifecycleFixture(t, 3)
		e, err := newSATElimEqs(l)
		satOK(t, err)
		t.Cleanup(func() { satOK(t, e.Close()) })
		roots := []Literal{0, 0, 0}
		r, err := p.clauses.MkClause([]Literal{0, 2, 4}, true)
		satOK(t, err)
		satOK(t, p.clauses.Freeze(r))
		satOK(t, l.pushClause(true, r))
		if empty {
			satOK(t, p.assign(1, satJustification{}))
		}
		satOK(t, e.run(roots, nil))
		if empty {
			if !p.inconsistent || len(l.learned) != 1 || !p.clauses.IsLive(r) {
				t.Fatal("empty retains current owner and source oldsize")
			}
			n, err := p.clauses.Size(r)
			satOK(t, err)
			if n != 3 {
				t.Fatal(n)
			}
		} else {
			v, err := p.variables.valueLiteral(0)
			satOK(t, err)
			if v != lbool.True || len(l.learned) != 0 || p.clauses.IsLive(r) {
				t.Fatal("unit assign then delete", v, len(l.learned))
			}
		}
	}
	l, p, _, _ := satLifecycleFixture(t, 3)
	e, err := newSATElimEqs(l)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, e.Close()) })
	r, err := p.clauses.MkClause([]Literal{4, 0, 2}, false)
	satOK(t, err)
	satOK(t, l.pushClause(false, r))
	satOK(t, e.run([]Literal{0, 2, 4}, nil))
	for i, want := range []Literal{4, 0, 2} {
		got, err := p.clauses.Literal(r, uint32(i))
		satOK(t, err)
		if got != want {
			t.Fatal("unaffected skips sort", got, want)
		}
	}
	satOK(t, p.variables.setExternal(BoolVar(1), true))
	satOK(t, e.run([]Literal{0, 0, 4}, []BoolVar{1}))
	elim, err := p.variables.wasEliminated(1)
	satOK(t, err)
	if elim || p.converter.Size() != 0 {
		t.Fatal("external adds real clauses instead of eliminated entry")
	}
}
func TestSATSCCParametersAndNativeLocalStorage(t *testing.T) {
	c, l := satSCCFixture(t, 2)
	p := l.propagation
	ctx := p.config.context
	if !c.enabled {
		t.Fatal("source default true")
	}
	local, err := ctx.NewValue()
	satOK(t, err)
	t.Cleanup(func() { satOK(t, ctx.Del(&local)) })
	p.config.globals.Register = func(r uparams.GlobalParamsRegistrar) error {
		d, err := uparams.NewParamDescrs(p.config.symbols, uparams.ParamDescrsConfig{Budget: p.config.globals.Budget})
		if err != nil {
			return err
		}
		if err = collectSCCParamDescrs(d); err != nil {
			return err
		}
		return r.RegisterModule("sat", d)
	}
	satOK(t, p.config.globals.SetName("sat.scc", "false"))
	satOK(t, c.Update(&local))
	if c.enabled {
		t.Fatal("real module fallback")
	}
	key := "scc"
	satOK(t, ctx.SetBoolName(&local, &key, true))
	satOK(t, c.Update(&local))
	if !c.enabled {
		t.Fatal("local precedes module")
	}
	satOK(t, p.config.globals.SetName("sat.scc", "true"))
	before := satSCCAllocationCount(t, c)
	l.budget.FailAfter(0)
	n, err := c.run()
	l.budget.FailAfter(-1)
	satOK(t, err)
	if n != 0 || satSCCAllocationCount(t, c) != before {
		t.Fatal("native locals do not allocate Runtime blocks", n, before, p.memory.State())
	}
	after := p.memory.Stats()
	used := l.budget.Used()
	before = satSCCAllocationCount(t, c)
	n, err = c.run()
	satOK(t, err)
	if n != 0 || satSCCAllocationCount(t, c) != before || p.memory.Stats() != after || l.budget.Used() != used {
		t.Fatal("native locals preserve exact Runtime plateau", n, p.memory.State(), after, p.memory.Stats(), used, l.budget.Used())
	}
	c.resetStatistics()
	if c.numElim != 0 {
		t.Fatal(c.numElim)
	}
}

func satSCCAllocationCount(t *testing.T, c *satSCC) uint64 {
	t.Helper()
	n, e := c.lifecycle.propagation.memory.AllocationCount()
	satOK(t, e)
	return n
}

func satSCCAudit(t *testing.T, c *satSCC, extra ...StoredClauseRef) {
	t.Helper()
	l := c.lifecycle
	p := l.propagation
	v := p.variables
	a := p.clauses
	refs := append([]StoredClauseRef(nil), extra...)
	addVector := func(x []StoredClauseRef, learned bool) {
		e := &c.eliminator
		if e.interrupted && e.learned == learned {
			refs = append(refs, x[e.drainPrefix:e.out]...)
			start := e.cursor
			if e.drainSuffix > start {
				start = e.drainSuffix
			}
			refs = append(refs, x[start:e.end]...)
		} else {
			refs = append(refs, x...)
		}
	}
	addVector(l.Original, false)
	addVector(l.learned, true)
	if l.pendingCandidate != (StoredClauseRef{}) {
		refs = append(refs, l.pendingCandidate)
	}
	for i, r := range refs {
		if !a.IsLive(r) {
			t.Fatal("named source owner stale", r)
		}
		for j := 0; j < i; j++ {
			if refs[j] == r {
				t.Fatal("duplicate scalar owner", r)
			}
		}
	}
	small := a.storage.Stats()
	if small.Live != uint32(len(refs)) {
		t.Fatal("unnamed physical clause", small.Live, len(refs))
	}
	want := c.header + c.eliminator.header + l.ownerBytes
	want += p.ownerBytes + v.Stats().TotalBytes
	todo := v.elimTodo
	want += todo.ownerBytes
	want += a.ownerBytes + a.largeBytes + a.largeDirectory + small.OwnerBytes + small.PayloadBytes + small.MetadataBytes + small.DirectoryBytes
	mc := p.converter
	want += mc.ownerBytes
	r := p.limit
	want += uint64(reflect.TypeOf(control.Reslimit{}).Size()) + 8*uint64(cap(r.Limits)) + uint64(reflect.TypeOf((*control.Reslimit)(nil)).Size())*uint64(cap(r.Children)) + p.memory.Stats().TotalBytes
	if want != l.budget.Used() {
		t.Fatal("whole named once-only physical graph", want, l.budget.Used())
	}
	satScalarAuditRoots(t, v)
}

func TestSATSCCActualAllocationPrefixSweep(t *testing.T) {
	failures := 0
	success := false
	var successOrdinal int64
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		c, l := satSCCFixture(t, 5)
		p := l.propagation
		satOK(t, l.mkBinClause(1, 2, false))
		satOK(t, l.mkBinClause(3, 0, false))
		satOK(t, l.mkBinClause(3, 4, false))
		satOK(t, l.mkBinClause(5, 2, false))
		for _, ls := range [][]Literal{{0, 2, 6}, {0, 2, 6, 8}, {1, 3, 6, 8}} {
			_, err := l.MkClauseCore(uint32(len(ls)), ls, false)
			satOK(t, err)
		}
		l.budget.FailAfter(ordinal)
		n, err := c.run()
		satSCCAudit(t, c)
		if err != nil {
			if !errors.Is(err, arena.ErrInjectedFailure) {
				var mf umemory.Failure
				if !errors.As(err, &mf) || mf.Kind != umemory.FailureMemout || !p.memory.State().OutOfMemory {
					t.Fatal(ordinal, err)
				}
			}
			failures++
		} else {
			if n != 2 {
				t.Fatal(ordinal, n)
			}
			success = true
			successOrdinal = ordinal
		}
		l.budget.FailAfter(-1)
		satOK(t, c.Close())
		satLifecycleAssertOwned(t, l)
		satOK(t, l.Close())
		if success {
			break
		}
	}
	if !success {
		t.Fatal(failures, success)
	}
	t.Logf("actual refusal ordinals0..%d, first mathematical success %d", failures-1, successOrdinal)
}

func TestSATSCCCancelBeforeNodeAndCapturedPhysicalFrame(t *testing.T) {
	c, l := satSCCFixture(t, 2)
	p := l.propagation
	satOK(t, l.mkBinClause(1, 2, false))
	satOK(t, l.mkBinClause(3, 0, false))
	p.limit.SetCancel(1)
	before := p.limit.Count()
	n, err := c.run()
	if !errors.Is(err, ErrSATSolverCanceled) || n != 0 || c.nextIndex != 0 || p.converter.Size() != 0 || p.limit.Count() != before+1 {
		t.Fatal(n, err, c.nextIndex, p.limit.Count())
	}
	satSCCAudit(t, c)
	p.limit.ResetCancel()
	// A source frame holds raw watch pointers, not a movable watch-list reference.
	c.active = true
	c.index = []uint32{^uint32(0), ^uint32(0), ^uint32(0), ^uint32(0)}
	c.lowlink = append([]uint32(nil), c.index...)
	c.inStack = make([]bool, 4)
	satOK(t, c.newNode(0))
	_, _, it, err := c.frame(0)
	satOK(t, err)
	list, err := p.variables.watches.Get(0)
	satOK(t, err)
	satOK(t, list.Finalize())
	_, err = satPropagationCursorRead(&it, 0)
	if !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("captured physical backing cannot follow replaced list", err)
	}
	satSCCAudit(t, c)
	satOK(t, c.closeVectors())
	if c.report.Running() {
		t.Fatal("report completed only after locals")
	}
}
func TestSATSCCColdConstructionAndSourceStatWrap(t *testing.T) {
	l, p, _, b := satLifecycleFixture(t, 2)
	local, err := p.config.context.NewValue()
	satOK(t, err)
	t.Cleanup(func() { satOK(t, p.config.context.Del(&local)) })
	used := b.Used()
	b.FailAfter(0)
	c, err := newSATSCC(l, &local)
	if c != nil || !errors.Is(err, arena.ErrInjectedFailure) || b.Used() != used {
		t.Fatal(c, err, b.Used(), used)
	}
	b.FailAfter(-1)
	satLifecycleAssertOwned(t, l)
	c, err = newSATSCC(l, &local)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, c.Close()) })
	satOK(t, l.mkBinClause(1, 2, false))
	satOK(t, l.mkBinClause(3, 0, false))
	c.numElim = ^uint32(0)
	n, err := c.run()
	satOK(t, err)
	if n != 1 || c.numElim != 0 {
		t.Fatal(n, c.numElim)
	}
	copy := *c
	if _, err = copy.run(); !errors.Is(err, ErrSATOwner) {
		t.Fatal(err)
	}
	satOK(t, c.Close())
	if _, err = c.run(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}
func TestSATElimEqsBinaryUnitConflictKeepsSourcePrefix(t *testing.T) {
	l, p, _, _ := satLifecycleFixture(t, 2)
	e, err := newSATElimEqs(l)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, e.Close()) })
	satOK(t, l.mkBinClause(0, 2, false))
	satOK(t, p.assign(1, satJustification{}))
	// Cleanup reaches conflicting assign before source wlist.set_end. No invented
	// early return in operator(): original clauses are still visited as source.
	list, err := p.variables.watches.Get(1)
	satOK(t, err)
	before, err := list.Size()
	satOK(t, err)
	satOK(t, e.run([]Literal{0, 0}, nil))
	after, err := list.Size()
	satOK(t, err)
	if !p.inconsistent || before != after || p.converter.Size() != 0 {
		t.Fatal("unit conflict before set_end", before, after, p.inconsistent)
	}
}

func TestSATElimEqsOwningIdentityGuard(t *testing.T) {
	_, l := satSCCFixture(t, 1)
	e, err := newSATElimEqs(l)
	satOK(t, err)
	copied := *e
	before := l.budget.Used()
	if err = copied.Close(); !errors.Is(err, ErrSATOwner) {
		t.Fatal("copied owning helper", err)
	}
	if l.budget.Used() != before {
		t.Fatal("copied Close refunded owner")
	}
	var zero satElimEqs
	if err = zero.Close(); !errors.Is(err, ErrSATOwner) {
		t.Fatal("zero owning helper", err)
	}
	satOK(t, e.Close())
	satOK(t, e.Close())
}

func TestSATElimEqsSortActualLargeBacking(t *testing.T) {
	c, l := satSCCFixture(t, 16)
	a := l.propagation.clauses
	// Test-control input is deliberately raw, not a pre-simplified solver clause.
	// Descending repeated literals includes adjacent complementary polarities.
	input := make([]Literal, 512)
	for i := range input {
		input[i] = Literal(31 - i/16)
	}
	clause := satMk(t, a, input, true)
	satOK(t, c.eliminator.sortClause(clause, 512))
	for i := uint32(0); i < 512; i++ {
		x, e := a.Literal(clause, i)
		satOK(t, e)
		if x != Literal(i/16) {
			t.Fatal(i, x)
		}
	}
	satSCCAudit(t, c, clause)
	satOK(t, a.Delete(clause))
}

func TestSATSCCNativeFrameGrowthAndCleanup(t *testing.T) {
	const n = 257
	c, l := satSCCFixture(t, n)
	p := l.propagation
	// Each positive literal has a real binary watch, captured before native growth.
	for i := uint32(0); i < n; i++ {
		satOK(t, l.mkBinClause(Literal(2*i+1), Literal(2*((i+1)%n)), false))
	}
	c.index, c.lowlink = make([]uint32, 2*n), make([]uint32, 2*n)
	c.inStack = make([]bool, 2*n)
	c.active = true
	before, blocks := l.budget.Used(), p.memory.Stats().LiveBlocks
	l.budget.FailAfter(0)
	for i := uint32(0); i < n; i++ {
		satOK(t, c.newNode(2*i))
	}
	l.budget.FailAfter(-1)
	if l.budget.Used() != before || p.memory.Stats().LiveBlocks != blocks {
		t.Fatal("native SCC growth reserved Runtime backing")
	}
	if len(c.frames) != n || len(c.stack) != n || c.nextIndex != n {
		t.Fatal("native traversal publication")
	}
	for i := uint32(0); i < n; i++ {
		li, first, it, err := c.frame(i)
		satOK(t, err)
		if li != 2*i || !first || c.index[li] != i || c.lowlink[li] != i || !c.inStack[li] || c.stack[i] != li {
			t.Fatal(i, li, first)
		}
		w, err := satPropagationCursorRead(&it, 0)
		satOK(t, err)
		lit, err := w.Literal()
		satOK(t, err)
		if lit != Literal(2*((i+1)%n)) {
			t.Fatal("typed frame lost borrowed watch", i, lit)
		}
	}
	satOK(t, c.closeVectors())
	if c.index != nil || c.lowlink != nil || c.stack != nil || c.inStack != nil || c.frames != nil || c.roots != nil || c.toElim != nil || c.active {
		t.Fatal("native traversal cleanup retained storage")
	}
	if l.budget.Used() != before || p.memory.Stats().LiveBlocks != blocks {
		t.Fatal("native cleanup changed Runtime ownership")
	}
	satSCCAudit(t, c)
}

func TestSATSCCNativeDeepComponentsAndRepeat(t *testing.T) {
	const n, group = 257, 17
	c, l := satSCCFixture(t, n)
	p := l.propagation
	// Disjoint rings joined in one direction have exactly ceil(n/group) SCCs.
	// The chain traverses all nodes before unwinding the first DFS stack.
	for start := uint32(0); start < n; start += group {
		end := start + group
		if end > n {
			end = n
		}
		for i := start; i < end; i++ {
			next := i + 1
			if next == end {
				next = start
			}
			satOK(t, l.mkBinClause(Literal(2*i+1), Literal(2*next), false))
		}
		if end < n {
			satOK(t, l.mkBinClause(Literal(2*(end-1)+1), Literal(2*end), false))
		}
	}
	count, err := c.run()
	satOK(t, err)
	if count != n-(n+group-1)/group || p.inconsistent {
		t.Fatal(count, p.inconsistent)
	}
	for _, value := range []lbool.LBool{lbool.False, lbool.True} {
		model := make(Model, n)
		for i := range model {
			eliminated, err := p.variables.wasEliminated(BoolVar(i))
			satOK(t, err)
			if eliminated {
				model[i] = lbool.Undef
			} else {
				model[i] = value
			}
		}
		satOK(t, p.converter.Apply(model))
		for i, x := range model {
			if x != value {
				t.Fatal("converter lost ring equivalence", i, x, value)
			}
		}
	}
	count, err = c.run()
	satOK(t, err)
	if count != 0 || c.active || c.reportActive || c.frames != nil || c.roots != nil {
		t.Fatal("repeat native traversal", count)
	}
	satSCCAudit(t, c)
}
