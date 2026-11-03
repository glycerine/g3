package ast

// Additional finite tests; no registered hyper-resolution source test exists.

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func hyperTestInputs(t *testing.T, m *ASTManager) ([]ASTHandle, ASTHandle, [][]ASTHandle) {
	t.Helper()
	a, b, c := proofTestBool(t, m, "hyper-a"), proofTestBool(t, m, "hyper-b"), proofTestBool(t, m, "hyper-c")
	pa, e := m.MkProofHypothesis(a)
	astTestMust(t, e)
	pb, e := m.MkProofHypothesis(b)
	astTestMust(t, e)
	for _, h := range []ASTHandle{a, b, c} {
		astTestMust(t, m.IncRef(h))
	}
	t.Cleanup(func() {
		for _, h := range []ASTHandle{a, b, c} {
			astTestMust(t, m.DecRef(h))
		}
	})
	return []ASTHandle{pa, pb}, c, [][]ASTHandle{{a, b}, {c}}
}

func TestASTProofHyperActualParametersAppendAndDisabled(t *testing.T) {
	m := astTestManager(t)
	// Source hyper-resolution directly calls mk_app even with proofs disabled.
	ps, f, substs := hyperTestInputs(t, m)
	positions := []ASTProofPosition{{First: math.MaxUint32, Second: 1 << 31}}
	p, e := m.MkProofHyperResolve(ps, f, positions, substs)
	astTestMust(t, e)
	if p.IsNull() {
		t.Fatal("source has no disabled guard")
	}
	proofTestFact(t, m, p, f)
	d, e := m.GetDecl(p)
	astTestMust(t, e)
	info := m.infoAt(m.declAt(d.slot).info.slot)
	if info.count != 5 {
		t.Fatal("flattened parameter count", info.count)
	}
	for i, want := range []int32{-1, math.MinInt32} {
		param, e := m.infoParameter(info, uint32(i+2))
		astTestMust(t, e)
		record, e := m.parameter(param)
		astTestMust(t, e)
		if record.Kind != ASTParameterInt || record.integer != want {
			t.Fatal("uint32-to-int32 source position conversion", i, record.integer)
		}
	}
	out, e := NewASTProofHyperOutput(m)
	astTestMust(t, e)
	defer out.Close()
	var premises []ASTHandle
	defer func() {
		for _, h := range premises {
			astTestMust(t, m.DecRef(h))
		}
	}()
	conclusion, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	defer m.DelRef(&conclusion)
	for iteration := uint32(0); iteration < 2; iteration++ {
		yes, e := m.MatchProofHyperResolve(p, &premises, conclusion, out)
		astTestMust(t, e)
		if !yes || out.PositionsLen() != iteration+1 || out.SubstitutionsLen() != 2*(iteration+1) {
			t.Fatal("append-only outputs", yes, out.PositionsLen(), out.SubstitutionsLen())
		}
		pos, e := out.Position(iteration)
		astTestMust(t, e)
		if pos != positions[0] {
			t.Fatal("signed integer decode", pos)
		}
		for j, input := range substs {
			v, e := out.Substitution(2*iteration + uint32(j))
			astTestMust(t, e)
			n, gotN := len(input), len(v)
			if gotN != n {
				t.Fatal("substitution lengths")
			}
			for k := 0; k < n; k++ {
				want, got := input[k], v[k]
				if want != got {
					t.Fatal("substitution order")
				}
			}
		}
	}
	n := len(premises)
	if n != 4 {
		t.Fatal("premises append", n)
	}
	got, e := m.RefGet(conclusion)
	astTestMust(t, e)
	if got != f {
		t.Fatal("conclusion assignment")
	}
	yes, e := m.MatchProofHyperResolve(f, &premises, conclusion, nil)
	astTestMust(t, e)
	if yes {
		t.Fatal("unmatched should not touch outputs")
	}
}

func TestASTProofHyperPremiseValidationBeforeParameterAllocation(t *testing.T) {
	m := astTestManager(t)
	ps, f, substs := hyperTestInputs(t, m)
	ps[1] = ASTHandle{}
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	_, e := m.MkProofHyperResolve(ps, f, []ASTProofPosition{{}}, substs)
	m.BudgetField.FailAfter(-1)
	if !errors.Is(e, ErrASTHandle) || m.BudgetField.Used() != before {
		t.Fatal("premise validation must precede actual parameter allocation", e, m.BudgetField.Used(), before)
	}
}

func TestASTProofHyperNativeArgumentsAndCache(t *testing.T) {
	m := astTestManager(t)
	ps, f, _ := hyperTestInputs(t, m)
	p, e := m.MkProofHyperResolve(ps[:1], f, nil, [][]ASTHandle{nil})
	astTestMust(t, e)
	before := m.BudgetField.Used()
	// Debug builds still execute the actual factory's well-sorted validation,
	// which has its own allocations independently of proof scratch storage.
	if !ASTSourceDebug {
		m.BudgetField.FailAfter(0)
	}
	q, e := m.MkProofHyperResolve(ps[:1], f, nil, [][]ASTHandle{nil})
	m.BudgetField.FailAfter(-1)
	astTestMust(t, e)
	if p != q || !ASTSourceDebug && m.BudgetField.Used() != before {
		t.Fatal("actual structural interning")
	}
	// Zero-parameter hyper declarations use the source fixed cache, whose
	// first parent count remains fixed; no invented variable-arity repair.
	_, e = m.MkProofHyperResolve(ps, f, nil, [][]ASTHandle{nil})
	if !errors.Is(e, ErrASTDomain) {
		t.Fatal("source zero-parameter cache arity", e)
	}
}

func TestASTProofHyperOutputNativeStorageAndEscapedCleanup(t *testing.T) {
	m := astTestManager(t)
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	out, e := NewASTProofHyperOutput(m)
	m.BudgetField.FailAfter(-1)
	if out != nil || !errors.Is(e, arena.ErrInjectedFailure) || m.BudgetField.Used() != before {
		t.Fatal("actual output owner-header refusal", out, e)
	}
	out, e = NewASTProofHyperOutput(m)
	astTestMust(t, e)
	if m.BudgetField.Used()-before != out.ownerBytes {
		t.Fatal("cold header charge")
	}
	copyOwner := *out
	if !errors.Is(copyOwner.Close(), ErrASTHandle) {
		t.Fatal("copied cold owner accepted")
	}
	astTestMust(t, out.Close())
	if m.BudgetField.Used() != before {
		t.Fatal("output owner not refunded")
	}
	escaped, e := NewASTProofHyperOutput(m)
	astTestMust(t, e)
	h, e := m.MkVar(700, m.external(m.boolSort))
	astTestMust(t, e)
	astTestMust(t, m.IncRef(h))
	escaped.substitutions = [][]ASTHandle{{h}}
	retained := escaped.ownerBytes
	borrowedOwners := m.BudgetField.Used() - m.Stats().AllocatedBytes - retained
	astTestMust(t, m.Close())
	if m.BudgetField.Used() != borrowedOwners+retained {
		t.Fatal("manager frees its own graph only", m.BudgetField.Used(), retained)
	}
	astTestMust(t, escaped.Close())
	if m.BudgetField.Used() != borrowedOwners || escaped.substitutions != nil {
		t.Fatal("escaped output releases native storage and owner")
	}
}

func TestASTProofHyperOutputAscendingDestructionAndIDReuse(t *testing.T) {
	m := astTestManager(t)
	s, e := m.MkBoolSort()
	astTestMust(t, e)
	a, e := m.MkVar(701, s)
	astTestMust(t, e)
	b, e := m.MkVar(702, s)
	astTestMust(t, e)
	c, e := m.MkVar(703, s)
	astTestMust(t, e)
	ids := [3]uint32{}
	for i, h := range []ASTHandle{a, b, c} {
		ids[i], e = m.GetASTID(h)
		astTestMust(t, e)
	}
	out, e := NewASTProofHyperOutput(m)
	astTestMust(t, e)
	for _, h := range []ASTHandle{a, b, c} {
		astTestMust(t, m.IncRef(h))
	}
	out.substitutions = [][]ASTHandle{{a, b}, {c}}
	astTestMust(t, out.Close())
	// Source outer vector destroys elements ascending; each inner owning
	// ref-vector decrements values ascending. Thus the ID recycle stack is
	// a,b,c and reuse is c,b,a; this probes actual production destruction.
	for i := uint32(0); i < 3; i++ {
		h, e := m.MkVar(710+i, s)
		astTestMust(t, e)
		id, e := m.GetASTID(h)
		astTestMust(t, e)
		if id != ids[2-i] {
			t.Fatal("source lexical destruction/ID recycle order", i, id, ids)
		}
	}
}

func TestASTProofHyperMalformedTrailingPairRetainsPriorOutputs(t *testing.T) {
	m := astTestManager(t)
	ps, f, substs := hyperTestInputs(t, m)
	a := substs[0][0]
	pa, e := m.NewParameterAST(a)
	astTestMust(t, e)
	pi, e := m.NewParameterInt(-1)
	astTestMust(t, e)
	info, e := m.NewFuncDeclInfo(0, ASTPrHyperResolve, []ASTParameter{pa, pi}, ASTFuncFlags{}, false)
	astTestMust(t, e)
	proofSort := m.external(m.proofSort)
	boolSort, e := m.MkBoolSort()
	astTestMust(t, e)
	d, e := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "malformed-hyper"), []ASTHandle{proofSort, boolSort}, proofSort, info)
	astTestMust(t, e)
	p, e := m.MkApp(d, []ASTHandle{ps[0], f})
	astTestMust(t, e)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	astTestMust(t, m.DelParameter(&pa))
	astTestMust(t, m.DelParameter(&pi))
	out, e := NewASTProofHyperOutput(m)
	astTestMust(t, e)
	defer out.Close()
	var premises []ASTHandle
	defer func() {
		for _, h := range premises {
			astTestMust(t, m.DecRef(h))
		}
	}()
	conclusion, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	defer m.DelRef(&conclusion)
	yes, err := m.MatchProofHyperResolve(p, &premises, conclusion, out)
	wantErr := arena.ErrBounds
	if ASTSourceDebug {
		wantErr = ErrASTDomain
	}
	if yes || !errors.Is(err, wantErr) {
		t.Fatal("trailing pair source assertion/OOB adapter", yes, err)
	}
	n := len(premises)
	got, e := m.RefGet(conclusion)
	astTestMust(t, e)
	v, e := out.Substitution(0)
	astTestMust(t, e)
	vn := len(v)
	value := v[0]
	if n != 1 || got != f || out.SubstitutionsLen() != 1 || out.PositionsLen() != 0 || vn != 1 || value != a {
		t.Fatal("reader must not prevalidate/rollback parameter suffix")
	}
}

func TestASTProofHyperBuilderAllFailurePrefixesImmediateOwnership(t *testing.T) {
	failures, successes := 0, 0
	for after := int64(0); after < 64; after++ {
		m := astTestManager(t)
		ps, f, substs := hyperTestInputs(t, m)
		// Warm the source declaration spelling so its legitimate interned
		// lifetime cannot be mistaken for an unaccounted numeric temporary.
		_, e := m.Symbols.Intern("hyper-res")
		astTestMust(t, e)
		borrowed := m.BudgetField.Used() - m.Stats().AllocatedBytes
		qBefore, e := m.numerals.Stats()
		astTestMust(t, e)
		m.BudgetField.FailAfter(after)
		_, err := m.MkProofHyperResolve(ps, f, []ASTProofPosition{{7, 9}}, substs)
		m.BudgetField.FailAfter(-1)
		if err != nil {
			failures++
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(after, err)
			}
		} else {
			successes++
		}
		// Inspect immediately, before Del/Close/String, so later cleanup cannot
		// hide a temporary left behind by a failed application construction.
		// Every surviving parameter must belong to a published node's info;
		// lexical parameter ownership must already be retired; argument slices
		// create no numeric-arena reservations.
		ownedInfos, ownedParameters := uint32(0), uint32(0)
		for slot := uint32(1); slot < m.nextNode; slot++ {
			r := m.nodeAt(slot)
			if !r.live {
				continue
			}
			var infoSlot uint32
			if r.kind == ASTSort {
				infoSlot = m.sortAt(slot).info.slot
			} else if r.kind == ASTFuncDecl {
				infoSlot = m.declAt(slot).info.slot
			}
			if infoSlot != 0 {
				ownedInfos++
				ownedParameters += m.infoAt(infoSlot).count
			}
		}
		qAfter, e := m.numerals.Stats()
		astTestMust(t, e)
		if m.liveInfos != ownedInfos || m.liveParameters != ownedParameters || qAfter.LiveCells != qBefore.LiveCells || qAfter.LivePayloads != qBefore.LivePayloads || m.BudgetField.Used() != borrowed+m.Stats().AllocatedBytes {
			t.Fatal("unreachable temporary before cleanup", after, err, m.liveInfos, ownedInfos, m.liveParameters, ownedParameters, m.BudgetField.Used(), borrowed+m.Stats().AllocatedBytes)
		}
	}
	if failures == 0 || successes == 0 {
		t.Fatal("did not traverse failure and success boundaries", failures, successes)
	}
}

func TestASTProofHyperZeroPremisesAndDebugGuardOrder(t *testing.T) {
	m := astTestManager(t)
	f := m.external(m.trueApp)
	p, e := m.MkProofHyperResolve(nil, f, nil, [][]ASTHandle{nil})
	astTestMust(t, e)
	proofTestFact(t, m, p, f)
	out, e := NewASTProofHyperOutput(m)
	astTestMust(t, e)
	defer out.Close()
	var premises []ASTHandle
	defer func() {
		for _, h := range premises {
			astTestMust(t, m.DecRef(h))
		}
	}()
	conclusion, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	defer m.DelRef(&conclusion)
	yes, e := m.MatchProofHyperResolve(p, &premises, conclusion, out)
	astTestMust(t, e)
	n := len(premises)
	if !yes || n != 0 || out.SubstitutionsLen() != 1 {
		t.Fatal("source zero-premise app and initial empty substitution")
	}
	if ASTSourceDebug {
		ps, _, _ := hyperTestInputs(t, m)
		before := m.BudgetField.Used()
		m.BudgetField.FailAfter(0)
		_, e = m.MkProofHyperResolve(ps, f, nil, nil)
		m.BudgetField.FailAfter(-1)
		if !errors.Is(e, ErrASTDomain) || m.BudgetField.Used() != before {
			t.Fatal("position count assertion precedes parameter construction", e)
		}
	}
}

func TestASTProofHyperNativeWideReaderExactOwnershipAndIndependentOutputs(t *testing.T) {
	m := astTestManager(t)
	ps, f, small := hyperTestInputs(t, m)
	atoms := []ASTHandle{small[0][0], small[0][1], f}
	substs := make([][]ASTHandle, 40)
	positions := make([]ASTProofPosition, len(substs)-1)
	for i := range substs {
		for j := 0; j < i%4; j++ {
			substs[i] = append(substs[i], atoms[(i+j)%len(atoms)])
		}
		if i < len(positions) {
			positions[i] = ASTProofPosition{uint32(i * 7), math.MaxUint32 - uint32(i)}
		}
	}
	p, e := m.MkProofHyperResolve(ps, f, positions, substs)
	astTestMust(t, e)
	astTestMust(t, m.IncRef(p))
	out, e := NewASTProofHyperOutput(m)
	astTestMust(t, e)
	defer out.Close()
	var premises []ASTHandle
	defer func() {
		for _, h := range premises {
			astTestMust(t, m.DecRef(h))
		}
	}()
	conclusion, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	defer m.DelRef(&conclusion)
	before := map[ASTHandle]uint32{}
	for _, h := range append(append([]ASTHandle(nil), ps...), atoms...) {
		before[h], e = m.GetRefCount(h)
		astTestMust(t, e)
	}
	used := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	t.Cleanup(func() { m.BudgetField.FailAfter(-1) })
	for iteration := 0; iteration < 2; iteration++ {
		yes, e := m.MatchProofHyperResolve(p, &premises, conclusion, out)
		astTestMust(t, e)
		if !yes || out.PositionsLen() != uint32((iteration+1)*len(positions)) || out.SubstitutionsLen() != uint32((iteration+1)*len(substs)) {
			t.Fatal("native append-only output sizes", iteration)
		}
	}
	if m.BudgetField.Used() != used || len(premises) != 4 {
		t.Fatal("native output growth charged the arena", used, m.BudgetField.Used(), premises)
	}
	expected := map[ASTHandle]uint32{f: 1} // conclusion replacement retains once
	for i, h := range premises {
		if h != ps[i%len(ps)] {
			t.Fatal("premise append order", i)
		}
		expected[h]++
	}
	for i := 0; i < int(out.SubstitutionsLen()); i++ {
		row, e := out.Substitution(uint32(i))
		astTestMust(t, e)
		want := substs[i%len(substs)]
		if len(row) != len(want) {
			t.Fatal("empty/nonempty substitution row", i)
		}
		for j, h := range row {
			if h != want[j] {
				t.Fatal("substitution flatten/decode order", i, j)
			}
			expected[h]++
		}
	}
	for i := 0; i < int(out.PositionsLen()); i++ {
		got, e := out.Position(uint32(i))
		astTestMust(t, e)
		if got != positions[i%len(positions)] {
			t.Fatal("position signed conversion/order", i, got)
		}
	}
	for h, refs := range before {
		got, e := m.GetRefCount(h)
		astTestMust(t, e)
		if got != refs+expected[h] {
			t.Fatal("unpublished or missing reference", h, refs, got, expected[h])
		}
	}
	m.BudgetField.FailAfter(-1)
	astTestMust(t, m.DecRef(p))
	if _, e = m.GetKind(p); !errors.Is(e, ErrASTHandle) {
		t.Fatal("proof owner did not end", e)
	}
	for _, h := range premises {
		_, e = m.GetKind(h)
		astTestMust(t, e)
	}
	for _, row := range out.substitutions {
		for _, h := range row {
			_, e = m.GetKind(h)
			astTestMust(t, e)
		}
	}
	got, e := m.RefGet(conclusion)
	astTestMust(t, e)
	if got != f {
		t.Fatal("source proof destruction changed independent conclusion")
	}
}

func TestASTProofHyperNativeOutputGuardsFollowPublishedPrefix(t *testing.T) {
	m := astTestManager(t)
	ps, f, substs := hyperTestInputs(t, m)
	p, e := m.MkProofHyperResolve(ps, f, []ASTProofPosition{{1, 2}}, substs)
	astTestMust(t, e)
	var premises []ASTHandle
	defer func() {
		for _, h := range premises {
			astTestMust(t, m.DecRef(h))
		}
	}()
	conclusion, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	defer m.DelRef(&conclusion)
	before, e := m.GetRefCount(ps[0])
	astTestMust(t, e)
	yes, e := m.MatchProofHyperResolve(p, nil, conclusion, nil)
	after, read := m.GetRefCount(ps[0])
	astTestMust(t, read)
	if yes || !errors.Is(e, ErrASTHandle) || before != after {
		t.Fatal("nil premise slice acquired an unpublished reference", yes, e, before, after)
	}
	yes, e = m.MatchProofHyperResolve(p, &premises, conclusion, nil)
	got, read := m.RefGet(conclusion)
	astTestMust(t, read)
	if yes || !errors.Is(e, ErrASTHandle) || len(premises) != len(ps) || got != f {
		t.Fatal("output owner guard hides earlier publications", yes, e, premises, got)
	}
	other := astTestManager(t)
	foreign, e := NewASTProofHyperOutput(other)
	astTestMust(t, e)
	defer foreign.Close()
	yes, e = m.MatchProofHyperResolve(p, &premises, conclusion, foreign)
	if yes || !errors.Is(e, ErrASTHandle) || len(premises) != 2*len(ps) || foreign.SubstitutionsLen() != 0 {
		t.Fatal("foreign output guard hides source append prefix", yes, e, premises)
	}
}
