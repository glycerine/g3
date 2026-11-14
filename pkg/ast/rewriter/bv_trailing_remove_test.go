package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:98-244,399-401.

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func bvTrailingValueFixture(t *testing.T) (*ast.ASTManager, *BVTrailing, ast.FamilyID, *unumeral.RationalService) {
	t.Helper()
	sy, q, b := astTestOwners(t)
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	satOK(t, e)
	t.Cleanup(func() { satOK(t, m.Close()) })
	_, e = m.RegisterArithmeticPlugin()
	satOK(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	satOK(t, e)
	extract, e := NewMkExtractProc(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, extract.Close()) })
	p, e := NewBVTrailing(extract)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, p.Close()) })
	return m, p, fid, q
}

func TestBVTrailingRemovalEveryOwnerAndFactoryReservation(t *testing.T) {
	for _, mode := range []string{"compound", "wide-mul", "wide-add", "single-coefficient"} {
		success, failures := false, 0
		for ordinal := int64(0); ordinal < 512; ordinal++ {
			t.Run(mode+"/"+strconv.FormatInt(ordinal, 10), func(t *testing.T) {
				m, p, fid := bvTrailingFixture(t)
				sort, e := m.BitVectorSort(8)
				satOK(t, e)
				x, e := m.MkVar(0, sort)
				satOK(t, e)
				astTestPin(t, m, x)
				coef, e := m.BitVectorNumeralUnsigned(12, 8)
				satOK(t, e)
				astTestPin(t, m, coef)
				var input ast.ASTHandle
				switch mode {
				case "compound":
					mul, e := m.MkBitVectorMul(coef, x)
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
					input, e = m.MkBitVectorAdd(mul, concat)
					satOK(t, e)
				case "wide-mul", "wide-add":
					args := []ast.ASTHandle{coef}
					for i := uint32(1); i < 20; i++ {
						h := coef
						if mode == "wide-mul" {
							h, e = m.MkVar(i, sort)
							satOK(t, e)
							astTestPin(t, m, h)
						}
						args = append(args, h)
					}
					kind := int32(4)
					if mode == "wide-mul" {
						kind = 6
					}
					input, e = m.MkFamilyApp(fid, kind, nil, args)
					satOK(t, e)
				case "single-coefficient":
					coef, e = m.BitVectorNumeralUnsigned(4, 8)
					satOK(t, e)
					astTestPin(t, m, coef)
					input, e = m.MkFamilyApp(fid, 6, nil, []ast.ASTHandle{coef})
					satOK(t, e)
				}
				astTestPin(t, m, input)
				before, e := m.GetRefCount(input)
				satOK(t, e)
				out := astTestPin(t, m, x)
				m.Budget().FailAfter(ordinal)
				removed, body := p.Remove(input, 2, out)
				m.Budget().FailAfter(-1)
				if body != nil {
					failures++
					if !errors.Is(body, arena.ErrInjectedFailure) {
						var f umemory.Failure
						if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
							t.Fatal("unexpected refusal", body)
						}
					}
					got, e := m.RefGet(out)
					satOK(t, e)
					if got != x {
						t.Fatal("failed body published output")
					}
					if _, e := p.Remove(input, 2, out); !errors.Is(e, ast.ErrASTPending) {
						t.Fatal("failed source operation retried", e)
					}
					for f := p.frames; f != nil; f = f.next {
						for _, h := range f.args {
							refs, e := m.GetRefCount(h)
							satOK(t, e)
							if refs == 0 {
								t.Fatal("retained native frame root lost its reference")
							}
						}
					}
				} else {
					success = true
					if removed != 2 {
						t.Fatal("completed removal", removed)
					}
				}
				satOK(t, p.Close())
				after, e := m.GetRefCount(input)
				satOK(t, e)
				if before != after {
					t.Fatal("caller root ref leaked", before, after)
				}
			})
			if success {
				break
			}
		}
		if !success || failures == 0 {
			t.Fatal("owner/factory reservation horizon", mode, success, failures)
		}
	}
}

func TestBVTrailingRemovalNativeWideRefAndOlderSliceDestruction(t *testing.T) {
	m, p, _ := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	var args []ast.ASTHandle
	for i := uint32(0); i < 32; i++ {
		h, e := m.MkVar(i, sort)
		satOK(t, e)
		args = append(args, h)
	}
	root, e := m.MkBitVectorOr(args)
	satOK(t, e)
	held, e := m.MkVar(100, sort)
	satOK(t, e)
	older, e := m.MkVar(101, sort)
	satOK(t, e)
	var call bvTrailingCall
	parent, e := p.beginFrame(bvTrailingAddFrame, &call)
	satOK(t, e)
	satOK(t, parent.pushArgValue(older))
	satOK(t, parent.newRef(0))
	f, e := p.beginFrame(bvTrailingAddFrame, &parent.call)
	satOK(t, e)
	satOK(t, f.newRef(0))
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	for i := 0; i < 257; i++ {
		satOK(t, f.pushArgValue(held))
	}
	m.Budget().FailAfter(-1)
	if m.Budget().Used() != before {
		t.Fatal("native root growth changed arena charge")
	}
	refs, e := m.GetRefCount(held)
	satOK(t, e)
	if refs != 257 {
		t.Fatal("native argument root acquisition", refs)
	}
	satOK(t, m.SetRef(f.refs[0], root))
	m.Budget().FailAfter(0)
	var body error
	f.finish(&body)
	parent.finish(&body)
	m.Budget().FailAfter(-1)
	satOK(t, body)
	if f.refs[0] != (ast.ASTRef{}) || f.args != nil || !f.sourceDone || parent.refs[0] != (ast.ASTRef{}) || parent.args != nil || !parent.sourceDone {
		t.Fatal("native frame destruction did not finish children and parent")
	}
	satOK(t, p.Close())
	if m.IsLiveHandle(held) {
		t.Fatal("older vector ref retained")
	}
	if m.IsLiveHandle(older) {
		t.Fatal("parent vector ref retained")
	}
	if e := call.sourceReady(m); e != nil {
		t.Fatal("completed native source DT not ready", e)
	}
}

func TestBVTrailingRemovalConcatUnchangedOutputSourceEdge(t *testing.T) {
	m, p, _ := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	x, e := m.MkVar(0, sort)
	satOK(t, e)
	astTestPin(t, m, x)
	zero, e := m.BitVectorNumeralUnsigned(0, 4)
	satOK(t, e)
	astTestPin(t, m, zero)
	input, e := m.MkBitVectorConcatPair(x, zero)
	satOK(t, e)
	astTestPin(t, m, input)
	out := astTestPin(t, m, input)
	removed, body := p.Remove(input, 6, out)
	got, e := m.RefGet(out)
	satOK(t, e)
	if ast.ASTSourceDebug {
		if !errors.Is(body, ast.ErrASTDomain) || got != input {
			t.Fatal("source full-width assertion", body, got)
		}
	} else {
		satOK(t, body)
		if removed != 4 || !got.IsNull() {
			t.Fatal("source leaves new_last null after unknown operand", removed, got)
		}
	}
}

func TestBVTrailingOriginalStructuralRemoval(t *testing.T) {
	m, p, fid, q := bvTrailingValueFixture(t)
	num := func(v, w uint32) ast.ASTHandle {
		h, e := m.BitVectorNumeralUnsigned(v, w)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	app := func(k int32, args ...ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkFamilyApp(fid, k, nil, args)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	x, e := m.MkVar(0, sort)
	satOK(t, e)
	astTestPin(t, m, x)
	two, twelve, twenty, zero := num(2, 8), num(12, 8), num(20, 8), num(0, 8)
	z4, z1 := num(0, 4), num(0, 1)
	rows := []struct {
		arg               ast.ASTHandle
		n, removed, width uint32
		kind              int32
		arity             uint32
		identity          ast.ASTHandle
	}{
		{twelve, 2, 2, 6, 0, 0, ast.ASTHandle{}},
		{app(4, twelve, twenty), 2, 2, 6, 4, 2, ast.ASTHandle{}},
		{app(6, two, x), 1, 1, 7, 40, 1, ast.ASTHandle{}},
		{app(6, twelve, x), 2, 2, 6, 6, 2, ast.ASTHandle{}},
		{app(6, two), 1, 1, 7, 0, 0, ast.ASTHandle{}},
		{app(37, num(3, 4), z4), 2, 2, 6, 37, 2, ast.ASTHandle{}},
		{app(37, x, z4), 4, 4, 8, 0, 0, x},
		{app(37, z4, z1), 2, 3, 2, 0, 0, ast.ASTHandle{}},
		{app(37, z4, z4), 8, 8, 0, 0, 0, ast.ASTHandle{}},
		{app(4, zero, zero), 8, 8, 0, 0, 0, ast.ASTHandle{}},
	}
	constants := make(map[ast.ASTHandle]uint64)
	var assignment uint64
	var eval func(ast.ASTHandle) uint64
	eval = func(h ast.ASTHandle) uint64 {
		if h.IsNull() {
			return 0
		}
		if h == x {
			return assignment
		}
		if v, ok := constants[h]; ok {
			return v
		}
		width, e := m.BitVectorSize(h)
		satOK(t, e)
		kind, e := m.GetDeclKind(h)
		satOK(t, e)
		if kind == 0 {
			v, e := q.NewValue()
			satOK(t, e)
			var bits uint32
			yes, e := m.BitVectorNumeralValue(h, &v, &bits)
			satOK(t, e)
			text, e := q.String(&v)
			satOK(t, e)
			satOK(t, q.Del(&v))
			value, e := strconv.ParseUint(text, 10, 64)
			satOK(t, e)
			if !yes || bits != width {
				t.Fatal("numeral metadata")
			}
			constants[h] = value
			return value
		}
		if kind == 40 {
			var lo, hi uint32
			var base ast.ASTHandle
			yes, e := m.BitVectorExtractValue(h, &lo, &hi, &base)
			satOK(t, e)
			if !yes || hi-lo+1 != width {
				t.Fatal("extract metadata")
			}
			return (eval(base) >> lo) & ((uint64(1) << width) - 1)
		}
		count, e := m.GetNumArgs(h)
		satOK(t, e)
		v := uint64(0)
		if kind == 6 {
			v = 1
		}
		for i := uint32(0); i < count; i++ {
			a, e := m.GetArg(h, i)
			satOK(t, e)
			value := eval(a)
			switch kind {
			case 4:
				v += value
			case 6:
				v *= value
			case 37:
				w, e := m.BitVectorSize(a)
				satOK(t, e)
				v = (v << w) | value
			default:
				t.Fatal("unexpected operation", kind)
			}
		}
		return v & ((uint64(1) << width) - 1)
	}
	for i, row := range rows {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			out := astTestPin(t, m, x)
			removed, e := p.Remove(row.arg, row.n, out)
			satOK(t, e)
			got, e := m.RefGet(out)
			satOK(t, e)
			if removed != row.removed {
				t.Fatal("removed count", removed, row.removed)
			}
			if row.width == 0 {
				if !got.IsNull() {
					t.Fatal("full removal did not publish null")
				}
				return
			}
			width, e := m.BitVectorSize(got)
			satOK(t, e)
			if width != row.width {
				t.Fatal("reduced width", width, row.width)
			}
			if !row.identity.IsNull() {
				if got != row.identity {
					t.Fatal("single remaining operand identity")
				}
			} else {
				kind, e := m.GetDeclKind(got)
				satOK(t, e)
				count, e := m.GetNumArgs(got)
				satOK(t, e)
				if kind != row.kind || count != row.arity {
					t.Fatal("source shape", kind, count, row.kind, row.arity)
				}
			}
			for assignment = 0; assignment < 256; assignment++ {
				if eval(got) != eval(row.arg)>>removed {
					t.Fatal("shift semantics", assignment, removed, eval(got), eval(row.arg))
				}
			}
		})
	}
}

func TestBVTrailingRemovalUnknownAndShallowGuards(t *testing.T) {
	m, p, _ := bvTrailingFixture(t)
	sort, e := m.BitVectorSort(8)
	satOK(t, e)
	x, e := m.MkVar(0, sort)
	satOK(t, e)
	astTestPin(t, m, x)
	one, e := m.BitVectorNumeralUnsigned(1, 8)
	satOK(t, e)
	out := astTestPin(t, m, one)
	if n, e := p.Remove(x, 2, out); e != nil || n != 0 {
		t.Fatal("unknown", n, e)
	}
	got, e := m.RefGet(out)
	satOK(t, e)
	if got != one {
		t.Fatal("unknown must leave output unchanged")
	}
	mul, e := m.MkBitVectorMul(one, x)
	satOK(t, e)
	astTestPin(t, m, mul)
	var call bvTrailingCall
	if n, e := p.remove(mul, 1, out, 1, &call); e != nil || n != 0 {
		t.Fatal("shallow", n, e)
	}
	got, e = m.RefGet(out)
	satOK(t, e)
	if got != mul {
		t.Fatal("shallow compound must assign its raw input")
	}
	m.Budget().FailAfter(0)
	if n, e := p.remove(mul, 1, ast.ASTRef{}, 0, &call); e != nil || n != 0 {
		t.Fatal("depth zero touched output", n, e)
	}
	if n, e := p.Remove(mul, 0, ast.ASTRef{}); e != nil || n != 0 {
		t.Fatal("zero request touched output", n, e)
	}
	m.Budget().FailAfter(-1)
}

func TestBVTrailingRemovalZeroCoefficientReachedInvalidRead(t *testing.T) {
	m, p, _ := bvTrailingFixture(t)
	zero, e := m.BitVectorNumeralUnsigned(0, 8)
	satOK(t, e)
	astTestPin(t, m, zero)
	input, e := m.MkBitVectorMul(zero, zero)
	satOK(t, e)
	astTestPin(t, m, input)
	out := astTestPin(t, m, input)
	_, e = p.Remove(input, 8, out)
	if !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("source reaches is_numeral(null) before new_sz guard", e)
	}
	got, e := m.RefGet(out)
	satOK(t, e)
	if got != input {
		t.Fatal("invalid null coefficient published")
	}
}

func TestBVTrailingRemovalUnreadOperandsAndZeroInputGuard(t *testing.T) {
	t.Run("unread", func(t *testing.T) {
		m, p, fid := bvTrailingFixture(t)
		sort, e := m.BitVectorSort(8)
		satOK(t, e)
		x, e := m.MkVar(0, sort)
		satOK(t, e)
		astTestPin(t, m, x)
		info, e := m.NewFuncDeclInfo(fid, 0, nil, ast.ASTFuncFlags{}, false)
		satOK(t, e)
		decl, e := m.MkFuncDecl(astTestSymbol(t, m, "unread-trailing-num"), nil, sort, info)
		satOK(t, e)
		satOK(t, m.DelFuncDeclInfo(&info))
		unread, e := m.MkApp(decl, nil)
		satOK(t, e)
		astTestPin(t, m, unread)
		for _, kind := range []int32{4, 6} {
			input, e := m.MkFamilyApp(fid, kind, nil, []ast.ASTHandle{x, unread})
			satOK(t, e)
			astTestPin(t, m, input)
			out := astTestPin(t, m, x)
			removed, e := p.Remove(input, 1, out)
			satOK(t, e)
			h, e := m.RefGet(out)
			satOK(t, e)
			if removed != 0 || h != input {
				t.Fatal("source zero-removal branch must leave later numerical metadata unread", kind, removed, h)
			}
		}
	})
	t.Run("zero-input", func(t *testing.T) {
		m, p, _ := bvTrailingFixture(t)
		before := m.Stats()
		m.Budget().FailAfter(0)
		removed, body := p.Remove(ast.ASTHandle{}, 0, ast.ASTRef{})
		m.Budget().FailAfter(-1)
		if ast.ASTSourceDebug {
			if !errors.Is(body, ast.ErrASTHandle) {
				t.Fatal("source BV assertion precedes zero-limit guard", body)
			}
		} else {
			satOK(t, body)
		}
		if removed != 0 || m.Stats() != before {
			t.Fatal("zero-limit guard mutated AST")
		}
	})
}

func TestBVTrailingRemovalActualRuntimeNumeralChild(t *testing.T) {
	success, failures, completedChildDT := false, 0, false
	for ordinal := int64(0); ordinal < 256; ordinal++ {
		t.Run(strconv.FormatInt(ordinal, 10), func(t *testing.T) {
			b := arena.NewBudget(1 << 28)
			rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, Transport: func(f umemory.Failure) error { return f }})
			satOK(t, e)
			q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
			satOK(t, e)
			sy, e := usymbol.NewManager(usymbol.Config{Budget: b})
			satOK(t, e)
			m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
			satOK(t, e)
			t.Cleanup(func() {
				satOK(t, q.CleanupLexical())
				satOK(t, m.Close())
				satOK(t, q.Close())
				satOK(t, sy.Close())
				satOK(t, rt.Close())
				if b.Used() != 0 {
					t.Fatal("actual Runtime graph retained", b.Used())
				}
			})
			_, e = m.RegisterArithmeticPlugin()
			satOK(t, e)
			fid, e := m.RegisterBitVectorPlugin()
			satOK(t, e)
			extract, e := NewMkExtractProc(m)
			satOK(t, e)
			t.Cleanup(func() { satOK(t, extract.Close()) })
			p, e := NewBVTrailing(extract)
			satOK(t, e)
			t.Cleanup(func() { satOK(t, p.Close()) })
			sort, e := m.BitVectorSort(8)
			satOK(t, e)
			x, e := m.MkVar(0, sort)
			satOK(t, e)
			astTestPin(t, m, x)
			value, e := q.NewValue()
			satOK(t, e)
			var words [33]uint32
			words[32] = 3
			satOK(t, q.Manager.Z.SetLimbs(&value.Raw.Num, words[:]))
			satOK(t, q.Manager.Z.Neg(&value.Raw.Num))
			param, e := m.NewParameterRational(&value)
			satOK(t, e)
			bits, e := m.NewParameterUint(8)
			satOK(t, e)
			info, e := m.NewFuncDeclInfo(fid, 0, []ast.ASTParameter{param, bits}, ast.ASTFuncFlags{}, false)
			satOK(t, e)
			decl, e := m.MkFuncDecl(astTestSymbol(t, m, "bv-trailing-cold"), nil, sort, info)
			satOK(t, e)
			satOK(t, m.DelFuncDeclInfo(&info))
			satOK(t, m.DelParameter(&bits))
			satOK(t, m.DelParameter(&param))
			satOK(t, q.Del(&value))
			coef, e := m.MkApp(decl, nil)
			satOK(t, e)
			astTestPin(t, m, coef)
			input, e := m.MkBitVectorMul(coef, x)
			satOK(t, e)
			astTestPin(t, m, input)
			out := astTestPin(t, m, x)
			b.FailAfter(ordinal)
			removed, body := p.Remove(input, 2, out)
			b.FailAfter(-1)
			if body != nil {
				failures++
				if !errors.Is(body, arena.ErrInjectedFailure) {
					var f umemory.Failure
					if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
						t.Fatal("unexpected actual refusal", body)
					}
				}
				h, e := m.RefGet(out)
				satOK(t, e)
				if h != x {
					t.Fatal("failed numerical child published")
				}
				for f := p.frames; f != nil; f = f.next {
					if e := f.call.sourceReady(m); e != nil {
						if f.sourceDone || f.refs[0] == (ast.ASTRef{}) {
							t.Fatal("older tmp DT crossed stopped exact numeral child", e)
						}
					} else if f.sourceDone && f.refs[0] == (ast.ASTRef{}) {
						completedChildDT = true
					}
				}
			} else {
				success = true
				if removed != 2 {
					t.Fatal("completed removal", removed)
				}
			}
			satOK(t, p.Close())
		})
		if success {
			break
		}
	}
	if !success || failures == 0 || !completedChildDT {
		t.Fatal("actual Runtime removal horizon", success, failures, completedChildDT)
	}
}
