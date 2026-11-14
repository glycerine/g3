package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:24-51,87-103,246-392.

import (
	"errors"
	"math/bits"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func bvTrailingFixture(t *testing.T) (*ast.ASTManager, *BVTrailing, ast.FamilyID) {
	t.Helper()
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	satOK(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	satOK(t, e)
	extract, e := NewMkExtractProc(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, extract.Close()) })
	p, e := NewBVTrailing(extract)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, p.Close()) })
	return m, p, fid
}

func TestBVTrailingOriginalCountingAlgorithms(t *testing.T) {
	m, p, fid := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	x, e := m.MkVar(0, sort)
	satOK(t, e)
	astTestPin(t, m, x)
	makeNum := func(v, w uint32) ast.ASTHandle {
		h, e := m.BitVectorNumeralUnsigned(v, w)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	odd, even, zero := makeNum(3, 8), makeNum(12, 8), makeNum(0, 8)
	add := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkBitVectorAdd(a, b)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	mul := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkBitVectorMul(a, b)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	concat := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkBitVectorConcatPair(a, b)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	for _, row := range []struct {
		arg      ast.ASTHandle
		min, max uint32
	}{
		{x, 0, 8}, {odd, 0, 0}, {even, 2, 2}, {zero, 8, 8},
		{add(odd, even), 0, 0}, {add(odd, odd), 0, 8}, {add(even, even), 2, 8},
		{mul(even, x), 2, 8}, {mul(x, even), 0, 8},
		{concat(odd, makeNum(0, 4)), 4, 4}, {concat(x, makeNum(0, 4)), 4, 12},
		{concat(odd, x), 0, 8}, {concat(zero, x), 0, 16},
	} {
		min, max, e := p.Count(row.arg)
		satOK(t, e)
		if min != row.min || max != row.max {
			t.Fatal("source count bounds", row.arg, min, max, row.min, row.max)
		}
	}
	// Each additional single-factor multiplication consumes one analysis depth.
	chain := even
	for i := 0; i < 5; i++ {
		chain, e = m.MkFamilyApp(fid, 6, nil, []ast.ASTHandle{chain})
		satOK(t, e)
		astTestPin(t, m, chain)
	}
	min, max, e := p.Count(chain)
	satOK(t, e)
	if min != 0 || max != 8 || p.tables[0] != nil {
		t.Fatal("depth-zero bounds/cache", min, max)
	}
	// Cached entries are erased reads of numerical data and allocate nothing.
	before := m.Stats()
	used := m.Budget().Used()
	m.Budget().FailAfter(0)
	min, max, e = p.Count(even)
	satOK(t, e)
	m.Budget().FailAfter(-1)
	if min != 2 || max != 2 || m.Stats() != before || m.Budget().Used() != used {
		t.Fatal("cache hit touched numerical/factory state")
	}
}

func TestBVTrailingNativeCacheGrowthOwnershipAndResetThreshold(t *testing.T) {
	m, p, _ := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	const count = 257
	keys := make([]ast.ASTHandle, count)
	for i := range keys {
		keys[i], e = m.MkVar(uint32(i), sort)
		satOK(t, e)
		astTestPin(t, m, keys[i])
	}
	// The first actual lazy table owns a cold header. All following growth
	// uses the native dictionary and must preserve exact INC ownership.
	_, _, e = p.Count(keys[0])
	satOK(t, e)
	used, storage := m.Budget().Used(), m.Stats()
	m.Budget().FailAfter(0)
	for _, key := range keys[1:] {
		before, e := m.GetRefCount(key)
		satOK(t, e)
		min, max, e := p.Count(key)
		satOK(t, e)
		after, e := m.GetRefCount(key)
		satOK(t, e)
		if min != 0 || max != 8 || after != before+1 {
			t.Fatal("native cache must own exactly one key reference", key, min, max, after)
		}
	}
	index := 0
	for _, entry := range p.tables[5].entries.All() {
		if entry.key != keys[index] || !entry.owned || entry.min != 0 || entry.max != 8 {
			t.Fatal("native growth changed ordered owning entry", index, entry)
		}
		index++
	}
	if index != count || m.Budget().Used() != used || m.Stats() != storage {
		t.Fatal("native cache growth changed actual object storage")
	}
	satOK(t, p.ResetCache(count+1))
	if p.tables[5] == nil || p.tables[5].entries.Len() != count {
		t.Fatal("strict reset threshold changed")
	}
	for _, key := range keys {
		min, max, e := p.Count(key)
		satOK(t, e)
		rc, e := m.GetRefCount(key)
		satOK(t, e)
		if min != 0 || max != 8 || rc != 2 {
			t.Fatal("cache hit reacquired key", key, rc)
		}
	}
	m.Budget().FailAfter(-1)
	satOK(t, p.ResetCache(count))
	if p.tables[5] != nil {
		t.Fatal("equal threshold did not retire table")
	}
	for _, key := range keys {
		rc, e := m.GetRefCount(key)
		satOK(t, e)
		if rc != 1 {
			t.Fatal("native cache cleanup changed caller reference", key, rc)
		}
	}
}

func TestBVTrailingCacheGrowthAndEqualThresholdReset(t *testing.T) {
	m, p, _ := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	for i := 0; i < 8; i++ {
		x, e := m.MkVar(uint32(i), sort)
		satOK(t, e)
		astTestPin(t, m, x)
		_, _, e = p.Count(x)
		satOK(t, e)
	}
	if p.tables[5].entries.Len() != 8 {
		t.Fatal("native cache lost an entry")
	}
	satOK(t, p.ResetCache(8))
	if p.tables[5] != nil {
		t.Fatal("equal threshold must reset")
	}
	copied := *p
	if _, _, e := copied.Count(ast.ASTHandle{}); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("copied owner accepted", e)
	}
	satOK(t, p.Close())
	satOK(t, p.Close())
	if _, _, e := p.Count(ast.ASTHandle{}); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("closed owner accepted", e)
	}
}

func TestBVTrailingIndependentBoundsAndDepthReferences(t *testing.T) {
	m, p, fid := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	x, e := m.MkVar(0, sort)
	satOK(t, e)
	astTestPin(t, m, x)
	two, e := m.BitVectorNumeralUnsigned(2, 8)
	satOK(t, e)
	astTestPin(t, m, two)
	values := map[ast.ASTHandle]uint64{two: 2}
	single, e := m.MkFamilyApp(fid, 6, nil, []ast.ASTHandle{two})
	satOK(t, e)
	astTestPin(t, m, single)
	expr, e := m.MkBitVectorAdd(two, single)
	satOK(t, e)
	astTestPin(t, m, expr)
	before, e := m.GetRefCount(two)
	satOK(t, e)
	_, _, e = p.Count(expr)
	satOK(t, e)
	after, e := m.GetRefCount(two)
	satOK(t, e)
	if after != before+2 || p.tables[3] == nil || p.tables[4] == nil || p.tables[0] != nil {
		t.Fatal("same key must have one reference per depth", before, after)
	}
	satOK(t, p.ResetCache(0))
	after, e = m.GetRefCount(two)
	satOK(t, e)
	if after != before {
		t.Fatal("depth resets retained references")
	}
	odd, e := m.BitVectorNumeralUnsigned(3, 8)
	satOK(t, e)
	astTestPin(t, m, odd)
	values[odd] = 3
	zero4, e := m.BitVectorNumeralUnsigned(0, 4)
	satOK(t, e)
	astTestPin(t, m, zero4)
	values[zero4] = 0
	mul, e := m.MkBitVectorMul(two, x)
	satOK(t, e)
	astTestPin(t, m, mul)
	sum, e := m.MkBitVectorAdd(mul, odd)
	satOK(t, e)
	astTestPin(t, m, sum)
	concat, e := m.MkBitVectorConcatPair(sum, zero4)
	satOK(t, e)
	astTestPin(t, m, concat)
	unknownSum, e := m.MkBitVectorAdd(x, two)
	satOK(t, e)
	astTestPin(t, m, unknownSum)
	var eval func(ast.ASTHandle) uint64
	eval = func(h ast.ASTHandle) uint64 {
		if value, ok := values[h]; ok {
			return value
		}
		kind, e := m.GetDeclKind(h)
		satOK(t, e)
		num, e := m.GetNumArgs(h)
		satOK(t, e)
		width, e := m.BitVectorSize(h)
		satOK(t, e)
		value := uint64(0)
		if kind == 6 {
			value = 1
		}
		for i := uint32(0); i < num; i++ {
			arg, e := m.GetArg(h, i)
			satOK(t, e)
			v := eval(arg)
			switch kind {
			case 4:
				value += v
			case 6:
				value *= v
			case 37:
				w, e := m.BitVectorSize(arg)
				satOK(t, e)
				value = (value << w) | v
			default:
				t.Fatal("unexpected expression", kind)
			}
		}
		return value & ((uint64(1) << width) - 1)
	}
	for _, h := range []ast.ASTHandle{mul, sum, concat, unknownSum} {
		min, max, e := p.Count(h)
		satOK(t, e)
		width, e := m.BitVectorSize(h)
		satOK(t, e)
		for assignment := uint64(0); assignment < 256; assignment++ {
			values[x] = assignment
			value := eval(h)
			zeros := width
			if value != 0 {
				zeros = uint32(bits.TrailingZeros64(value))
			}
			if zeros < min || zeros > max {
				t.Fatal("unsound bound", h, assignment, value, zeros, min, max)
			}
		}
	}
}

func TestBVTrailingEveryColdCountReservation(t *testing.T) {
	success, failures := false, 0
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		t.Run(strconv.FormatInt(ordinal, 10), func(t *testing.T) {
			m, p, _ := bvTrailingFixture(t)
			sort, e := m.BitVectorSort(8)
			satOK(t, e)
			x, e := m.MkVar(0, sort)
			satOK(t, e)
			astTestPin(t, m, x)
			coefficient, e := m.BitVectorNumeralUnsigned(12, 8)
			satOK(t, e)
			astTestPin(t, m, coefficient)
			mul, e := m.MkBitVectorMul(coefficient, x)
			satOK(t, e)
			astTestPin(t, m, mul)
			odd, e := m.BitVectorNumeralUnsigned(3, 4)
			satOK(t, e)
			astTestPin(t, m, odd)
			zero, e := m.BitVectorNumeralUnsigned(0, 4)
			satOK(t, e)
			astTestPin(t, m, zero)
			concat, e := m.MkBitVectorConcatPair(odd, zero)
			satOK(t, e)
			astTestPin(t, m, concat)
			input, e := m.MkBitVectorAdd(mul, concat)
			satOK(t, e)
			astTestPin(t, m, input)
			m.Budget().FailAfter(ordinal)
			min, max, body := p.Count(input)
			m.Budget().FailAfter(-1)
			if body != nil {
				failures++
				if !errors.Is(body, arena.ErrInjectedFailure) {
					var f umemory.Failure
					if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
						t.Fatal("unexpected refusal", body)
					}
				}
				if _, _, e := p.Count(input); !errors.Is(e, ast.ErrASTPending) {
					t.Fatal("failed count retried", e)
				}
			} else {
				success = true
				if min != 2 || max != 8 {
					t.Fatal("completed compound bounds", min, max)
				}
			}
		})
		if success {
			break
		}
	}
	if !success || failures == 0 {
		t.Fatal("cold count horizon", success, failures)
	}
}

func TestBVTrailingNativeWideDeletionReset(t *testing.T) {
	m, p, _ := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	var args []ast.ASTHandle
	for i := uint32(0); i < 32; i++ {
		x, e := m.MkVar(i, sort)
		satOK(t, e)
		args = append(args, x)
	}
	root, e := m.MkBitVectorOr(args)
	satOK(t, e)
	_, _, e = p.Count(root)
	satOK(t, e)
	rc, e := m.GetRefCount(root)
	satOK(t, e)
	if rc != 1 {
		t.Fatal("cache should be sole root owner", rc)
	}
	m.Budget().FailAfter(0)
	body := p.ResetCache(0)
	m.Budget().FailAfter(-1)
	satOK(t, body)
	for _, table := range p.tables {
		if table != nil {
			t.Fatal("native reset retained a cache table")
		}
	}
	if m.IsLiveHandle(root) {
		t.Fatal("native reset retained the sole-owned root")
	}
	satOK(t, p.Close())
}

func TestBVTrailingEmptyCompoundsAndUnreadOperands(t *testing.T) {
	m, p, fid := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	raw := func(kind int32, name string) ast.ASTHandle {
		symbol, e := m.Symbols.Intern(name)
		satOK(t, e)
		info, e := m.NewFuncDeclInfo(fid, kind, nil, ast.ASTFuncFlags{}, false)
		satOK(t, e)
		d, e := m.MkFuncDecl(symbol, nil, sort, info)
		satOK(t, e)
		satOK(t, m.DelFuncDeclInfo(&info))
		h, e := m.MkApp(d, nil)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	for _, row := range []struct {
		kind     int32
		name     string
		min, max uint32
	}{{6, "trailing-empty-mul", 0, 0}, {4, "trailing-empty-add", 8, 8}, {37, "trailing-empty-concat", 0, 0}} {
		min, max, e := p.Count(raw(row.kind, row.name))
		satOK(t, e)
		if min != row.min || max != row.max {
			t.Fatal("empty source compound", row.kind, min, max)
		}
	}
	// NUM identity is present but numerical parameters are absent. Each original
	// short-circuit must avoid reading/copying this unreachable operand.
	unread := raw(0, "trailing-unread-num")
	x, e := m.MkVar(0, sort)
	satOK(t, e)
	astTestPin(t, m, x)
	odd, e := m.BitVectorNumeralUnsigned(3, 8)
	satOK(t, e)
	astTestPin(t, m, odd)
	add, e := m.MkBitVectorAdd(x, unread)
	satOK(t, e)
	astTestPin(t, m, add)
	mul, e := m.MkBitVectorMul(x, unread)
	satOK(t, e)
	astTestPin(t, m, mul)
	concat, e := m.MkBitVectorConcatPair(unread, odd)
	satOK(t, e)
	astTestPin(t, m, concat)
	for _, row := range []struct {
		arg ast.ASTHandle
		max uint32
	}{{add, 8}, {mul, 8}, {concat, 0}} {
		min, max, e := p.Count(row.arg)
		satOK(t, e)
		if min != 0 || max != row.max {
			t.Fatal("source short-circuit bounds", min, max, row.max)
		}
	}
	if table := p.tables[4]; table != nil {
		_, found, e := table.find(m, unread)
		satOK(t, e)
		if found {
			t.Fatal("unread operand was cached")
		}
	}
}

func TestBVTrailingConstructorBorrowsExtractionOwner(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	satOK(t, e)
	_, e = m.RegisterBitVectorPlugin()
	satOK(t, e)
	extract, e := NewMkExtractProc(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, extract.Close()) })
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	refused, e := NewBVTrailing(extract)
	m.Budget().FailAfter(-1)
	if refused != nil || !errors.Is(e, arena.ErrInjectedFailure) || m.Budget().Used() != before {
		t.Fatal("constructor refusal retained header", refused, e)
	}
	p, e := NewBVTrailing(extract)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, p.Close()) })
	for _, table := range p.tables {
		if table != nil {
			t.Fatal("constructor eagerly created cache")
		}
	}
	satOK(t, p.Close())
	if m.Budget().Used() != before || extract.closed {
		t.Fatal("count owner closed borrowed extractor")
	}
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	x, e := m.MkVar(0, sort)
	satOK(t, e)
	astTestPin(t, m, x)
	_, e = extract.Apply(7, 7, x)
	satOK(t, e)
}
