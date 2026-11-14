package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2078-2079,2109-2137.

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

func bvAddFixture(t *testing.T) (*ast.ASTManager, ast.FamilyID) {
	t.Helper()
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	return m, fid
}

func bvAddNum(t *testing.T, m *ast.ASTManager, v, width uint32) ast.ASTHandle {
	t.Helper()
	h, e := m.BitVectorNumeralUnsigned(v, width)
	astTestMust(t, e)
	astTestPin(t, m, h)
	return h
}

func TestBVRewriterAdditionAfterCoreStatusAndUnreadOperands(t *testing.T) {
	for _, st := range []RewriteStatus{Rewrite1, Rewrite2, Rewrite3, RewriteFull} {
		got, e := RewriteBitVectorAdditionAfterCore(nil, st, ast.SliceArguments([]ast.ASTHandle{{}}), ast.ASTRef{})
		if got != st || e != nil {
			t.Fatal("early status read manager/operands/output", got, e)
		}
	}
	for _, args := range [][]ast.ASTHandle{nil, {{}}} {
		st, e := RewriteBitVectorAdditionAfterCore(nil, RewriteFailed, ast.SliceArguments(args), ast.ASTRef{})
		if st != RewriteFailed || e != nil {
			t.Fatal("short arity read its operand", st, e)
		}
	}
	m, fid := bvAddFixture(t)
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	x, e := m.MkVar(0, s)
	astTestMust(t, e)
	astTestPin(t, m, x)
	y, e := m.MkVar(1, s)
	astTestMust(t, e)
	astTestPin(t, m, y)
	// Raw NUM lacks numerical parameters. A bit-major loop must stop at x/y
	// on bit zero, before reaching this malformed third operand.
	name := astTestSymbol(t, m, "addition-unread-num")
	info, e := m.NewFuncDeclInfo(fid, 0, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, e)
	d, e := m.MkFuncDecl(name, nil, s, info)
	astTestMust(t, e)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	raw, e := m.MkApp(d, nil)
	astTestMust(t, e)
	astTestPin(t, m, raw)
	out := astTestPin(t, m, x)
	args := []ast.ASTHandle{x, y, raw}
	before := m.Stats()
	m.Budget().FailAfter(0)
	st, body := RewriteBitVectorAdditionAfterCore(m, RewriteFailed, ast.SliceArguments(args), out)
	m.Budget().FailAfter(-1)
	astTestMust(t, body)
	h, e := m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteFailed || h != x || m.Stats() != before {
		t.Fatal("conflict changed output", st, h)
	}
	// DONE with a non-addition result leaves the original argument storage unread.
	st, e = RewriteBitVectorAdditionAfterCore(m, RewriteDone, ast.SliceArguments([]ast.ASTHandle{{}, {}}), out)
	astTestMust(t, e)
	if st != RewriteDone {
		t.Fatal("non-addition DONE", st)
	}
	sum, e := m.MkFamilyApp(fid, 4, nil, args)
	astTestMust(t, e)
	astTestPin(t, m, sum)
	astTestMust(t, m.SetRef(out, sum))
	st, e = RewriteBitVectorAdditionAfterCore(m, RewriteDone, ast.SliceArguments([]ast.ASTHandle{{}, {}}), out)
	astTestMust(t, e)
	h, e = m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteDone || h != sum {
		t.Fatal("DONE conflict changed core result", st)
	}
}

func TestBVRewriterAdditionAfterCoreNumeralsAndBorrowedResult(t *testing.T) {
	for _, incoming := range []RewriteStatus{RewriteFailed, RewriteDone} {
		t.Run(strconv.Itoa(int(incoming)), func(t *testing.T) {
			m, fid := bvAddFixture(t)
			args := []ast.ASTHandle{bvAddNum(t, m, 1, 8), bvAddNum(t, m, 2, 8), bvAddNum(t, m, 4, 8)}
			sentinel := bvAddNum(t, m, 77, 8)
			out := astTestPin(t, m, sentinel)
			selected := ast.SliceArguments(args)
			var old ast.AppArguments
			if incoming == RewriteDone {
				sum, e := m.MkFamilyApp(fid, 4, nil, args)
				astTestMust(t, e)
				astTestMust(t, m.SetRef(out, sum)) // out is the sole owner of this sum.
				old, e = m.AppArgumentView(sum)
				astTestMust(t, e)
				selected = ast.SliceArguments([]ast.ASTHandle{{}, {}})
			}
			st, e := RewriteBitVectorAdditionAfterCore(m, incoming, selected, out)
			astTestMust(t, e)
			h, e := m.RefGet(out)
			astTestMust(t, e)
			k, e := m.GetDeclKind(h)
			astTestMust(t, e)
			n, e := m.GetNumArgs(h)
			astTestMust(t, e)
			if st != Rewrite1 || k != 31 || n != 3 {
				t.Fatal("disjoint-bit OR", st, k, n)
			}
			for i, a := range args {
				got, e := m.GetArg(h, uint32(i))
				astTestMust(t, e)
				if got != a {
					t.Fatal("OR operand order", i)
				}
			}
			if incoming == RewriteDone {
				if _, e = old.At(0); e == nil {
					t.Fatal("post-core reduction retained old sum")
				}
			}
		})
	}
}

func TestBVRewriterAdditionAfterCoreSymbolicDisjointSemantics(t *testing.T) {
	m, _ := bvAddFixture(t)
	s, e := m.BitVectorSort(4)
	astTestMust(t, e)
	x, e := m.MkVar(0, s)
	astTestMust(t, e)
	astTestPin(t, m, x)
	y, e := m.MkVar(1, s)
	astTestMust(t, e)
	astTestPin(t, m, y)
	zero := bvAddNum(t, m, 0, 4)
	left, e := m.MkBitVectorConcatPair(x, zero)
	astTestMust(t, e)
	astTestPin(t, m, left)
	right, e := m.MkBitVectorConcatPair(zero, y)
	astTestMust(t, e)
	astTestPin(t, m, right)
	out := astTestPin(t, m, left)
	st, e := RewriteBitVectorAdditionAfterCore(m, RewriteFailed, ast.SliceArguments([]ast.ASTHandle{left, right}), out)
	astTestMust(t, e)
	result, e := m.RefGet(out)
	astTestMust(t, e)
	if st != Rewrite1 {
		t.Fatal("disjoint symbolic bits", st)
	}
	// Independent uint32 evaluation over all 256 assignments checks addition
	// against the emitted concat/OR expression, rather than repeating bit tests.
	var eval func(ast.ASTHandle, uint32, uint32) uint32
	eval = func(h ast.ASTHandle, xv, yv uint32) uint32 {
		switch h {
		case x:
			return xv
		case y:
			return yv
		case zero:
			return 0
		}
		k, e := m.GetDeclKind(h)
		astTestMust(t, e)
		n, e := m.GetNumArgs(h)
		astTestMust(t, e)
		var v uint32
		for i := uint32(0); i < n; i++ {
			a, e := m.GetArg(h, i)
			astTestMust(t, e)
			switch k {
			case 37:
				w, e := m.BitVectorSize(a)
				astTestMust(t, e)
				v = v<<w | eval(a, xv, yv)
			case 31:
				v |= eval(a, xv, yv)
			default:
				t.Fatal("unexpected emitted operator", k)
			}
		}
		return v
	}
	for xv := uint32(0); xv < 16; xv++ {
		for yv := uint32(0); yv < 16; yv++ {
			want := (eval(left, xv, yv) + eval(right, xv, yv)) & 255
			if got := eval(result, xv, yv); got != want {
				t.Fatal("sum/OR disagree", xv, yv, got, want)
			}
		}
	}
}

func TestBVRewriterAdditionAfterCoreEveryColdReservation(t *testing.T) {
	for _, incoming := range []RewriteStatus{RewriteFailed, RewriteDone} {
		success, failures := false, 0
		for ordinal := int64(0); ordinal < 512; ordinal++ {
			t.Run(strconv.Itoa(int(incoming))+"/"+strconv.FormatInt(ordinal, 10), func(t *testing.T) {
				m, fid := bvAddFixture(t)
				args := []ast.ASTHandle{bvAddNum(t, m, 1, 4), bvAddNum(t, m, 2, 4), bvAddNum(t, m, 4, 4)}
				sentinel := bvAddNum(t, m, 9, 4)
				out := astTestPin(t, m, sentinel)
				selected := ast.SliceArguments(args)
				if incoming == RewriteDone {
					original, e := m.MkFamilyApp(fid, 4, nil, args)
					astTestMust(t, e)
					astTestPin(t, m, original)
					astTestMust(t, m.SetRef(out, original))
					// Deliberately impossible original operands: only the core sum is read.
					selected = ast.SliceArguments([]ast.ASTHandle{{}, {}})
					sentinel = original
				}
				// Original inputs remain pinned, so final assignment cannot create a
				// destruction callback error after publishing the new result.
				before := make([]uint32, len(args))
				for i, h := range args {
					var e error
					before[i], e = m.GetRefCount(h)
					astTestMust(t, e)
				}
				m.Budget().FailAfter(ordinal)
				st, body := RewriteBitVectorAdditionAfterCore(m, incoming, selected, out)
				m.Budget().FailAfter(-1)
				if body != nil {
					failures++
					if !errors.Is(body, arena.ErrInjectedFailure) {
						var f umemory.Failure
						if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
							t.Fatal("unexpected allocation refusal", body)
						}
					}
					h, e := m.RefGet(out)
					astTestMust(t, e)
					if h != sentinel {
						t.Fatal("failed body published", h, sentinel)
					}
					for i, h := range args {
						count, e := m.GetRefCount(h)
						astTestMust(t, e)
						if count != before[i] {
							t.Fatal("failed body retained operand reference", i, count, before[i])
						}
					}
				} else {
					success = true
					if st != Rewrite1 {
						t.Fatal("completed status", st)
					}
					h, e := m.RefGet(out)
					astTestMust(t, e)
					k, e := m.GetDeclKind(h)
					astTestMust(t, e)
					if k != 31 {
						t.Fatal("completed result", k)
					}
				}
			})
			if success {
				break
			}
		}
		// A warm AST page can satisfy the small OR result without a new
		// Runtime reservation. The separate numeral-child campaign below
		// requires and checks actual refusal points.
		if !success {
			t.Fatal("cold reservation horizon", incoming, success, failures)
		}
	}
}

func TestBVRewriterAdditionAfterCoreNativeWideAssignment(t *testing.T) {
	success, failures := false, 0
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		t.Run(strconv.FormatInt(ordinal, 10), func(t *testing.T) {
			m, _ := bvAddFixture(t)
			args := []ast.ASTHandle{bvAddNum(t, m, 1, 4), bvAddNum(t, m, 2, 4), bvAddNum(t, m, 4, 4)}
			s, e := m.BitVectorSort(8)
			astTestMust(t, e)
			var older []ast.ASTHandle
			for i := uint32(0); i < 32; i++ {
				h, e := m.MkVar(i, s)
				astTestMust(t, e)
				older = append(older, h)
			}
			old, e := m.MkBitVectorOr(older)
			astTestMust(t, e)
			out := astTestPin(t, m, old) // sole reference; deletion grows the native worklist.
			m.Budget().FailAfter(ordinal)
			st, body := RewriteBitVectorAdditionAfterCore(m, RewriteFailed, ast.SliceArguments(args), out)
			m.Budget().FailAfter(-1)
			got, e := m.RefGet(out)
			astTestMust(t, e)
			if body != nil {
				if !errors.Is(body, arena.ErrInjectedFailure) {
					var f umemory.Failure
					if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
						t.Fatal("unexpected refusal", body)
					}
				}
				failures++
				if got != old {
					t.Fatal("failed replacement changed the old reference", got, old)
				}
			} else {
				success = true
				if st != Rewrite1 || got == old {
					t.Fatal("completed assignment", st)
				}
				if m.IsLiveHandle(old) {
					t.Fatal("completed assignment retained old root")
				}
				n, e := m.GetNumArgs(got)
				astTestMust(t, e)
				if n != 3 {
					t.Fatal("published replacement arity", n)
				}
			}
			// Fixture closure destroys only the reference currently in out; the
			// manager owns the remaining AST and numerical contexts.
		})
		if success {
			break
		}
	}
	if !success {
		t.Fatal("native assignment horizon", success, failures)
	}
}

func TestBVRewriterAdditionAfterCoreActualRuntimeNumeralChild(t *testing.T) {
	success, failures, reachedDivision := false, 0, false
	for ordinal := int64(0); ordinal < 512; ordinal++ {
		t.Run(strconv.FormatInt(ordinal, 10), func(t *testing.T) {
			b := arena.NewBudget(1 << 28)
			rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, Transport: func(f umemory.Failure) error { return f }})
			astTestMust(t, e)
			q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
			astTestMust(t, e)
			sy, e := usymbol.NewManager(usymbol.Config{Budget: b})
			astTestMust(t, e)
			m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
			astTestMust(t, e)
			t.Cleanup(func() {
				astTestMust(t, q.CleanupLexical())
				astTestMust(t, m.Close())
				astTestMust(t, q.Close())
				astTestMust(t, sy.Close())
				astTestMust(t, rt.Close())
				if b.Used() != 0 {
					t.Fatal("actual Runtime graph retained", b.Used())
				}
			})
			_, e = m.RegisterArithmeticPlugin()
			astTestMust(t, e)
			fid, e := m.RegisterBitVectorPlugin()
			astTestMust(t, e)
			s, e := m.BitVectorSort(2)
			astTestMust(t, e)
			x, e := m.MkVar(0, s)
			astTestMust(t, e)
			astTestPin(t, m, x)
			value, e := q.NewValue()
			astTestMust(t, e)
			var words [33]uint32
			words[32] = 3
			astTestMust(t, q.Manager.Z.SetLimbs(&value.Raw.Num, words[:]))
			astTestMust(t, q.Manager.Z.Neg(&value.Raw.Num))
			param, e := m.NewParameterRational(&value)
			astTestMust(t, e)
			bits, e := m.NewParameterUint(2)
			astTestMust(t, e)
			info, e := m.NewFuncDeclInfo(fid, 0, []ast.ASTParameter{param, bits}, ast.ASTFuncFlags{}, false)
			astTestMust(t, e)
			decl, e := m.MkFuncDecl(astTestSymbol(t, m, "bv-add-cold"), nil, s, info)
			astTestMust(t, e)
			astTestMust(t, m.DelFuncDeclInfo(&info))
			astTestMust(t, m.DelParameter(&bits))
			astTestMust(t, m.DelParameter(&param))
			astTestMust(t, q.Del(&value))
			raw, e := m.MkApp(decl, nil)
			astTestMust(t, e)
			astTestPin(t, m, raw)
			out := astTestPin(t, m, x)
			// The stored negative 1024-bit number has zero in both declared bits.
			// Its copied value still exercises the original large negative division.
			b.FailAfter(ordinal)
			st, body := RewriteBitVectorAdditionAfterCore(m, RewriteFailed, ast.SliceArguments([]ast.ASTHandle{raw, x}), out)
			b.FailAfter(-1)
			reachedDivision = reachedDivision || q.Manager.Z.DivisionRaw.Invocation != 0
			got, e := m.RefGet(out)
			astTestMust(t, e)
			if body != nil {
				failures++
				if !errors.Is(body, arena.ErrInjectedFailure) {
					var f umemory.Failure
					if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
						t.Fatal("unexpected Runtime refusal", body)
					}
				}
				if got != x {
					t.Fatal("failed numeric child published result")
				}
			} else {
				success = true
				if st != Rewrite1 {
					t.Fatal("completed Runtime reduction", st)
				}
				k, e := m.GetDeclKind(got)
				astTestMust(t, e)
				first, e := m.GetArg(got, 0)
				astTestMust(t, e)
				if k != 31 || first != raw {
					t.Fatal("stored numeral replaced or normalized", k)
				}
			}
		})
		if success {
			break
		}
	}
	if !success || failures == 0 || !reachedDivision {
		t.Fatal("actual Runtime horizon", success, failures, reachedDivision)
	}
}
