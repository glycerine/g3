package numeral

// Source: src/util/mpq.cpp, src/util/rational.cpp, and infinitesimal headers.
// Reverse member destruction and separate owning cleanup.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func transitiveSourceOwner(t *testing.T, service bool, profile umemory.SmallAllocatorProfile) (*RationalManager, *RationalService, *umemory.Runtime, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(1 << 24)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	cfg := IntegerConfig{Memory: rt, Profile: profile, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}
	var m *RationalManager
	var s *RationalService
	if service {
		s, e = NewRationalService(RationalServiceConfig{Integers: cfg})
		if e == nil {
			m = s.Manager
		}
	} else {
		m, e = NewRationalManager(RationalConfig{Integers: cfg})
	}
	if e != nil {
		t.Fatal(e)
	}
	return m, s, rt, b
}
func transitiveSourceRun(m *RationalManager, s *RationalService) error {
	if s != nil {
		return s.SourceFinalize()
	}
	return m.SourceFinalize()
}
func transitiveSourceClose(t *testing.T, m *RationalManager, s *RationalService, rt *umemory.Runtime, b *arena.Budget) {
	t.Helper()
	var e error
	if s != nil {
		e = s.CloseChecked()
	} else {
		e = m.CloseChecked()
	}
	if e != nil {
		t.Fatal(e)
	}
	if rt.Stats().LiveBlocks != 0 || b.Used() != rt.Stats().TotalBytes {
		t.Fatal("owning cleanup physical/cold graph", rt.Stats(), b.Used())
	}
	if e = rt.CheckOpen(); e != nil {
		t.Fatal("borrowed runtime closed", e)
	}
	if e = rt.Close(); e != nil || b.Used() != 0 {
		t.Fatal("final borrowed runtime cleanup", e, b.Used())
	}
}
func transitiveSourceDigits(n int) []uint32 { d := make([]uint32, n); d[0], d[n-1] = 7, 1; return d }

func TestTransitiveSourceFinalizeCallerPhysicalLifetimes(t *testing.T) {
	for _, service := range []bool{false, true} {
		for _, direct := range []bool{false, true} {
			m, s, rt, b := transitiveSourceOwner(t, service, umemory.SmallAllocatorProfile{PointerBits: 64})
			caller := NewRational()
			n := 6
			if direct {
				n = 60
			}
			if e := m.Z.SetLimbs(&caller.Num, transitiveSourceDigits(n)); e != nil {
				t.Fatal(e)
			}
			raw, offset := m.Z.Cell(caller.Num.Cell).Backing.Raw, m.Z.Cell(caller.Num.Cell).Backing.Offset
			object, allocator := m.Z.Cell(caller.Num.Cell).Backing.Object, m.Z.Allocator
			if e := transitiveSourceRun(m, s); e != nil {
				t.Fatal(e)
			}
			if !m.SourceFinalized || !m.Z.SourceFinalized || (s != nil && !s.sourceFinalized) {
				t.Fatal("child source finalizer omitted")
			}
			if _, e := m.String(&caller); !errors.Is(e, arena.ErrClosed) {
				t.Fatal("ordinary rational read after source destruction", e)
			}
			if s != nil {
				if _, e := s.NewValue(); !errors.Is(e, arena.ErrClosed) {
					t.Fatal("ordinary service use after source destruction", e)
				}
			}
			if direct {
				if rt.Stats().LiveBlocks != 1 || rt.Stats().PhysicalBytes != 256 {
					t.Fatal("caller direct source lifetime", rt.Stats())
				}
				if v, e := sourceCellWord(rt, allocator, object, raw, offset, 8); e != nil || v != 7 {
					t.Fatal(v, e)
				}
			} else {
				if rt.Stats().LiveBlocks != 0 {
					t.Fatal("pool source lifetime", rt.Stats())
				}
				if _, e := rt.ReadUint32(raw, offset+8); !errors.Is(e, umemory.ErrRuntimeOwner) {
					t.Fatal("destroyed pooled bytes", e)
				}
			}
			transitiveSourceAudit(t, m, s, b, &caller.Num, &caller.Den)
			state, used := rt.State(), b.Used()
			if e := transitiveSourceRun(m, s); e != nil || rt.State() != state || b.Used() != used || m.HeaderBytes == 0 || (s != nil && s.HeaderBytes == 0) {
				t.Fatal("source retry/cold ownership", e)
			}
			transitiveSourceClose(t, m, s, rt, b)
		}
	}
}

func TestTransitiveSourceFinalizeActualDiagnosticPrefix(t *testing.T) {
	for _, service := range []bool{false, true} {
		for _, retry := range []bool{false, true} {
			m, s, rt, b := transitiveSourceOwner(t, service, umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true, Windows: true})
			caller := NewRational()
			if e := m.Z.SetLimbs(&caller.Num, transitiveSourceDigits(60)); e != nil {
				t.Fatal(e)
			}
			if e := transitiveSourceRun(m, s); !errors.Is(e, umemory.ErrAllocatorIO) {
				t.Fatal("real post-pool-free diagnostic refusal", e)
			}
			if m.cleanupStage != 9 || m.SourceFinalized || m.Z.SourceFinalizeStage != 7 || rt.Stats().LiveBlocks != 1 || (m.Z.Natural == nil || !m.Z.Natural.sourceFinalized || m.Z.Natural.SourceStage != 4 || m.Z.Natural.HeaderBytes == 0) || m.HeaderBytes == 0 {
				t.Fatal("source diagnostic prefix lost")
			}
			if s != nil && (s.cleanupStage != 25 || s.sourceFinalized) {
				t.Fatal("service outer prefix lost")
			}
			transitiveSourceAudit(t, m, s, b, &caller.Num, &caller.Den)
			if retry {
				state, used := rt.State(), b.Used()
				if e := transitiveSourceRun(m, s); e != nil || rt.State() != state || b.Used() != used {
					t.Fatal("diagnostic replay", e)
				}
			}
			transitiveSourceClose(t, m, s, rt, b)
		}
	}
}

func TestRationalSourceFinalizeActualPrivateGuardOrder(t *testing.T) {
	for stop := 0; stop < 9; stop++ {
		m, s, rt, b := transitiveSourceOwner(t, false, umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true})
		roots := [9]*Int{&m.NTmp, &m.AddTmp1, &m.AddTmp2, &m.LtTmp1.Num, &m.LtTmp1.Den, &m.LtTmp2.Num, &m.LtTmp2.Den, &m.AddMulTmp.Num, &m.AddMulTmp.Den}
		// Reverse physical allocation order prevents descriptor scanning from
		// accidentally satisfying source destructor-order assertions.
		var raws [9]umemory.Block
		for i := 8; i >= 0; i-- {
			if e := m.Z.SetLimbs(roots[i], transitiveSourceDigits(60)); e != nil {
				t.Fatal(e)
			}
			raws[i] = m.Z.Cell(roots[i].Cell).Backing.Raw
		}
		var input Int
		if e := m.Z.SetLimbs(&input, transitiveSourceDigits(120)); e != nil {
			t.Fatal(e)
		}
		b.FailAfter(0)
		e := m.Z.Copy(roots[stop], &input)
		if !numericPhysicalReservationRefusal(e) || roots[stop].Cell != m.Z.FailedRef {
			t.Fatal("real private setter failure prefix", e)
		}
		b.FailAfter(-1)
		if e = m.SourceFinalize(); !errors.Is(e, arena.ErrInvalidSlot) {
			t.Fatal("invalid private capacity access", e)
		}
		if !m.sourceGuard || m.cleanupStage != uint8(stop) || m.SourceFinalized || m.Z.SourceFinalizing {
			t.Fatal("undefined leaf silently completed", stop)
		}
		for i := range raws {
			_, e = rt.ReadUint32(raws[i], 4) // m_capacity: digits are a native payload
			if i <= stop {
				if !errors.Is(e, umemory.ErrRuntimeOwner) {
					t.Fatal("prior source release missing", stop, i, e)
				}
			} else if e != nil {
				t.Fatal("later source action executed", stop, i, e)
			}
		}
		transitiveSourceAudit(t, m, s, b, &input)
		q := NewRational()
		if e = m.check(&q); !errors.Is(e, arena.ErrClosed) {
			t.Fatal("ordinary read at outer source guard", e)
		}
		state, used := rt.State(), b.Used()
		if e = m.SourceFinalize(); !errors.Is(e, arena.ErrInvalidSlot) || rt.State() != state || b.Used() != used {
			t.Fatal("guard repeated source actions", e)
		}
		transitiveSourceClose(t, m, s, rt, b)
		if !m.sourceGuard || m.cleanupStage != uint8(stop) || m.SourceFinalized {
			t.Fatal("owning abort rewrote stopped source history")
		}
	}
}

func TestServiceSourceFinalizeActualCacheDenominatorGuard(t *testing.T) {
	m, s, rt, b := transitiveSourceOwner(t, true, umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true})
	if _, e := s.PowerOfTwo(1); e != nil {
		t.Fatal(e)
	}
	prior := [3]*Int{&s.Cache[0].Num, &s.Cache[0].Den, &s.Cache[1].Num}
	var raws [3]umemory.Block
	for i, p := range prior {
		if e := m.Z.SetLimbs(p, transitiveSourceDigits(60)); e != nil {
			t.Fatal(e)
		}
		raws[i] = m.Z.Cell(p.Cell).Backing.Raw
	}
	if e := m.Z.SetLimbs(&s.Cache[1].Den, transitiveSourceDigits(6)); e != nil {
		t.Fatal(e)
	}
	caller := Int{}
	if e := m.Z.SetLimbs(&caller, transitiveSourceDigits(65)); e != nil {
		t.Fatal(e)
	}
	b.FailAfter(0)
	e := m.Z.Copy(&s.Cache[1].Den, &caller)
	if !numericPhysicalReservationRefusal(e) || s.Cache[1].Den.Cell != m.Z.FailedRef {
		t.Fatal(e)
	}
	transitiveSourceAudit(t, m, s, b, &caller)
	b.FailAfter(-1)
	if e = s.SourceFinalize(); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal(e)
	}
	if !s.sourceGuard || s.cleanupStage != 18 || s.cleanupCacheIndex != 1 || s.cleanupComponent != 1 || m.SourceFinalizing {
		t.Fatal("cache NUM/DEN source prefix")
	}
	for _, raw := range raws {
		if _, e = rt.ReadUint32(raw, 8); !errors.Is(e, umemory.ErrRuntimeOwner) {
			t.Fatal("prior cache source release", e)
		}
	}
	transitiveSourceAudit(t, m, s, b, &caller)
	if _, e = s.NewValue(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("service read at cache guard", e)
	}
	if e = m.Z.Del(&caller); e != nil {
		t.Fatal(e)
	}
	transitiveSourceClose(t, m, s, rt, b)
	if !s.sourceGuard || s.cleanupStage != 18 || s.cleanupCacheIndex != 1 || s.cleanupComponent != 1 || s.sourceFinalized {
		t.Fatal("owning cache abort rewrote source history")
	}
}

func TestServiceSourceFinalizeFreshCacheDenominatorNull(t *testing.T) {
	m, s, rt, b := transitiveSourceOwner(t, true, umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true})
	if _, e := s.PowerOfTwo(1); e != nil {
		t.Fatal(e)
	}
	if s.Cache[1].Den.Cell != 0 {
		t.Fatal("source destination is not fresh")
	}
	b.FailAfter(0)
	e := m.Z.Copy(&s.Cache[1].Den, &m.Z.Two64)
	if !numericPhysicalReservationRefusal(e) || s.Cache[1].Den.Cell != m.Z.FailedNullRef {
		t.Fatal("actual fresh NULL publication", e, s.Cache[1].Den)
	}
	transitiveSourceAudit(t, m, s, b)
	b.FailAfter(-1)
	if e = s.SourceFinalize(); e != nil {
		t.Fatal("defined source NULL destructor", e)
	}
	if !s.sourceFinalized || !m.SourceFinalized || !m.Z.SourceFinalized || s.sourceGuard {
		t.Fatal("defined source continuation did not finish")
	}
	transitiveSourceAudit(t, m, s, b)
	transitiveSourceClose(t, m, s, rt, b)
}

func TestTransitiveSourceFinalizeActualChildScratchHole(t *testing.T) {
	for _, service := range []bool{false, true} {
		m, s, rt, b := transitiveSourceOwner(t, service, umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true})
		old := m.Z.Tmp[0]
		b.FailAfter(0)
		e := m.Z.EnsureTmp(0, 33)
		if !numericPhysicalReservationRefusal(e) || m.Z.Tmp[0] != 0 || ValidNumericRef(m.Z.Cells, old) {
			t.Fatal("real free-before-replacement scratch hole", e)
		}
		b.FailAfter(-1)
		if e = transitiveSourceRun(m, s); !errors.Is(e, arena.ErrInvalidSlot) {
			t.Fatal(e)
		}
		if !m.sourceGuard || m.cleanupStage != 9 || m.SourceFinalized || !m.Z.sourceGuard || m.Z.SourceFinalizeStage != 2 {
			t.Fatal("child source guard propagation")
		}
		if s != nil && (!s.sourceGuard || s.cleanupStage != 25 || s.sourceFinalized) {
			t.Fatal("service child source guard")
		}
		transitiveSourceAudit(t, m, s, b)
		transitiveSourceClose(t, m, s, rt, b)
		if m.Z.Tmp[0] != 0 || m.Z.SourceFinalizeStage != 2 || m.Z.SourceFinalized || m.SourceFinalized || (s != nil && s.sourceFinalized) {
			t.Fatal("owning cleanup repaired or falsely finalized child")
		}
	}
}

// Independent retained graph audit works both before child destruction and
// after pooled payloads have died while charged logical descriptors survive.
func transitiveSourceAudit(t *testing.T, m *RationalManager, s *RationalService, b *arena.Budget, public ...*Int) {
	t.Helper()
	z := m.Z
	refs := map[NumericCellRef]bool{}
	add := func(p *Int) {
		if p == nil || p.Cell == 0 || z.IsFailureRef(p.Cell) {
			return
		}
		if !ValidNumericRef(z.Cells, p.Cell) {
			t.Fatal("invalid named retained descriptor", p.Cell)
		}
		refs[p.Cell] = true
	}
	for i := range z.Tmp {
		r := Int{Cell: z.Tmp[i]}
		add(&r)
		r.Cell = z.Arg[i]
		add(&r)
	}
	add(&z.Two64)
	add(&z.IntMin)
	for _, p := range []*Int{&m.NTmp, &m.AddTmp1, &m.AddTmp2, &m.LtTmp1.Num, &m.LtTmp1.Den, &m.LtTmp2.Num, &m.LtTmp2.Den, &m.AddMulTmp.Num, &m.AddMulTmp.Den} {
		add(p)
	}
	if s != nil {
		for _, r := range []*Rational{&s.InfZeroField.First, &s.InfZeroField.Second, &s.InfOneField.First, &s.InfOneField.Second, &s.InfMinusOneField.First, &s.InfMinusOneField.Second, &s.InfIntZeroField.First, &s.InfIntOneField.First, &s.InfIntMinusOneField.First, &s.ZeroField, &s.OneField, &s.MinusOneField} {
			add(&r.Num)
			add(&r.Den)
		}
		for i := range s.Cache {
			add(&s.Cache[i].Num)
			add(&s.Cache[i].Den)
		}
	}
	for _, p := range public {
		add(p)
	}
	for i := uint64(1); i <= uint64(z.Cells.Stats().Slots); i++ {
		c, e := z.Cells.Read(arena.Slot(i))
		if e == nil && (c.Unpublished || c.CleanupPending) {
			p := Int{Cell: MakeNumericRef(arena.Slot(i), z.Cells.State(arena.Slot(i)).Generation)}
			add(&p)
		}
	}
	cs := z.Cells.Stats()
	if uint32(len(refs)) != cs.Live {
		t.Fatal("unnamed source lifetime descriptor", len(refs), cs.Live)
	}
	direct := map[umemory.Block]bool{}
	for ref := range refs {
		c := z.Cell(ref)
		v, e := z.Memory.ReadUint32(c.Backing.Raw, c.Backing.Offset+4)
		if c.Released {
			if !errors.Is(e, umemory.ErrRuntimeOwner) {
				t.Fatal("released pooled raw remains accessible", e)
			}
			continue
		}
		if e != nil || v != c.Capacity {
			t.Fatal("retained physical header", v, c.Capacity, e)
		}
		if c.Backing.Offset == 0 {
			if direct[c.Backing.Raw] {
				t.Fatal("duplicate direct physical root")
			}
			direct[c.Backing.Raw] = true
		}
	}
	if z.PendingCell.Object != (umemory.ObjectRef{}) {
		if z.PendingCell.Offset == 0 {
			direct[z.PendingCell.Raw] = true
		}
	}
	a := z.Allocator.Stats()
	rawBuffers := map[umemory.Block]bool{}
	owned := m.HeaderBytes + z.HeaderBytes + z.Memory.Stats().TotalBytes + a.TotalBytes + cs.OwnerBytes + cs.RecordBytes + cs.MetadataBytes + cs.DirectoryBytes

	if z.Natural != nil {
		n := z.Natural
		owned += n.HeaderBytes

	}
	if s != nil {
		owned += s.HeaderBytes
	}
	if b.Used() != owned {
		t.Fatal("retained complete physical/cold graph", b.Used(), owned)
	}
	if z.Memory.Stats().LiveBlocks != uint32(len(direct)+len(rawBuffers))+a.Chunks+a.PendingBlocks {
		t.Fatal("unnamed actual Runtime physical root", z.Memory.Stats(), a, len(direct), len(rawBuffers))
	}
}

func TestTransitiveSourceFinalizeColdOwnerAndReadinessGuards(t *testing.T) {
	var zeroM RationalManager
	var zeroS RationalService
	if e := zeroM.SourceFinalize(); !errors.Is(e, umemory.ErrRuntimeOwner) {
		t.Fatal(e)
	}
	if e := zeroS.SourceFinalize(); !errors.Is(e, ErrRationalServiceCopy) {
		t.Fatal(e)
	}
	m, s, rt, b := transitiveSourceOwner(t, true, umemory.SmallAllocatorProfile{PointerBits: 64})
	copiedM := *m
	copiedS := RationalService{self: s, Manager: m, ownerID: s.ownerID}
	state, used := rt.State(), b.Used()
	if e := copiedM.SourceFinalize(); !errors.Is(e, umemory.ErrRuntimeOwner) {
		t.Fatal(e)
	}
	if e := copiedS.SourceFinalize(); !errors.Is(e, ErrRationalServiceCopy) {
		t.Fatal(e)
	}
	// Cold readiness unit test, not a fabricated physical free refusal. The
	// malformed numeral must remain unread at the earlier owner-state guard.
	bad := Rational{Num: Int{Cell: NumericCellRef(^uint32(0))}, Den: NewInt(1)}
	m.Closing = true
	if e := m.checkOwnerReady(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
	if e := m.check(&bad); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closing guard read numeral", e)
	}
	m.Closing = false
	s.closing = true
	if _, e := s.NewValue(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
	s.closing = false
	if rt.State() != state || b.Used() != used {
		t.Fatal("guard performed source actions")
	}
	transitiveSourceAudit(t, m, s, b)
	if e := s.SourceFinalize(); e != nil {
		t.Fatal(e)
	}
	transitiveSourceClose(t, m, s, rt, b)
	if e := m.SourceFinalize(); e != nil {
		t.Fatal(e)
	}
	if e := s.SourceFinalize(); e != nil {
		t.Fatal(e)
	}
	if e := m.checkOwnerReady(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}

// The selected service constants are private scalar fields in Go. Give each
// distinct actual backing, in reverse allocation order, and provoke the real
// source setter failure at every hook/member boundary before child destruction.
func TestServiceSourceFinalizeActualHookAndConstantOrder(t *testing.T) {
	for stop := 0; stop < 24; stop++ {
		m, s, rt, b := transitiveSourceOwner(t, true, umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true})
		if _, e := s.PowerOfTwo(1); e != nil {
			t.Fatal(e)
		}
		roots := [24]*Int{
			&s.InfZeroField.Second.Num, &s.InfZeroField.Second.Den, &s.InfZeroField.First.Num, &s.InfZeroField.First.Den,
			&s.InfOneField.Second.Num, &s.InfOneField.Second.Den, &s.InfOneField.First.Num, &s.InfOneField.First.Den,
			&s.InfMinusOneField.Second.Num, &s.InfMinusOneField.Second.Den, &s.InfMinusOneField.First.Num, &s.InfMinusOneField.First.Den,
			&s.InfIntZeroField.First.Num, &s.InfIntZeroField.First.Den, &s.InfIntOneField.First.Num, &s.InfIntOneField.First.Den, &s.InfIntMinusOneField.First.Num, &s.InfIntMinusOneField.First.Den,
			&s.ZeroField.Num, &s.ZeroField.Den, &s.OneField.Num, &s.OneField.Den, &s.MinusOneField.Num, &s.MinusOneField.Den}
		var raws [24]umemory.Block
		for i := 23; i >= 0; i-- {
			if e := m.Z.SetLimbs(roots[i], transitiveSourceDigits(60)); e != nil {
				t.Fatal(e)
			}
			raws[i] = m.Z.Cell(roots[i].Cell).Backing.Raw
		}
		var input Int
		if e := m.Z.SetLimbs(&input, transitiveSourceDigits(120)); e != nil {
			t.Fatal(e)
		}
		b.FailAfter(0)
		e := m.Z.Copy(roots[stop], &input)
		if !numericPhysicalReservationRefusal(e) || roots[stop].Cell != m.Z.FailedRef {
			t.Fatal("actual hook setter refusal", stop, e)
		}
		b.FailAfter(-1)
		if e = s.SourceFinalize(); !errors.Is(e, arena.ErrInvalidSlot) {
			t.Fatal(stop, e)
		}
		stage := uint8(stop)
		if stop >= 18 {
			stage++
		} // cache is source stage18.
		if !s.sourceGuard || s.cleanupStage != stage || s.sourceFinalized || m.SourceFinalizing {
			t.Fatal("hook/constant source boundary", stop, s.cleanupStage)
		}
		for i, raw := range raws {
			_, e = rt.ReadUint32(raw, 4) // m_capacity: digits are a native payload
			if i <= stop {
				if !errors.Is(e, umemory.ErrRuntimeOwner) {
					t.Fatal("prior hook/member release missing", stop, i, e)
				}
			} else if e != nil {
				t.Fatal("later hook/member release occurred", stop, i, e)
			}
		}
		transitiveSourceAudit(t, m, s, b, &input)
		transitiveSourceClose(t, m, s, rt, b)
		if !s.sourceGuard || s.cleanupStage != stage || s.sourceFinalized {
			t.Fatal("owning abort changed source hook history", stop)
		}
	}
}
