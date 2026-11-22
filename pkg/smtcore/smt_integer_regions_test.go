package smtcore

import (
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type regionTestAST struct {
	t       testing.TB
	m       *ast.ASTManager
	family  ast.FamilyID
	sort, x ast.ASTHandle
}

func newRegionTestAST(t testing.TB, m *ast.ASTManager) *regionTestAST {
	t.Helper()
	f, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	s, err := m.MkFamilySort(f, smtArithIntSort, nil)
	astTestMust(t, err)
	x, err := m.MkVar(0, s)
	astTestMust(t, err)
	return &regionTestAST{t, m, f, s, x}
}

func (f *regionTestAST) must(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	return h
}

func (f *regionTestAST) num(n int64) ast.ASTHandle {
	return f.must(f.m.ArithmeticNumeralInt64(n, true))
}
func (f *regionTestAST) app(op int32, args ...ast.ASTHandle) ast.ASTHandle {
	return f.must(f.m.MkFamilyApp(f.family, op, nil, args))
}
func (f *regionTestAST) affine(a, b int64) ast.ASTHandle {
	return f.app(smtArithOpAdd, f.app(smtArithOpMul, f.num(a), f.x), f.num(b))
}

func TestIntegerRegionsAgainstExhaustiveEvaluation(t *testing.T) {
	m := astTestManager(t)
	f := newRegionTestAST(t, m)
	rng := rand.New(rand.NewSource(427))
	for trial := 0; trial < 200; trial++ {
		// All branch predicates compare lines whose offsets are in [-20,20].
		// Thus [-41,41] includes representatives of every possible truth value.
		var a, b [5]int64
		var lines [5]ast.ASTHandle
		for j := range a {
			a[j] = int64(rng.Intn(9) - 4)
			b[j] = int64(rng.Intn(41) - 20)
			lines[j] = f.affine(a[j], b[j])
		}
		guard := f.app(smtArithOpGT, lines[0], lines[1])
		piece := f.must(m.MkIte(guard, lines[2], lines[3]))
		eq := f.must(m.MkEq(piece, lines[4]))
		bound := f.app(smtArithOpLE, lines[3], lines[0])
		var body ast.ASTHandle
		switch trial % 6 {
		case 0:
			body = f.must(m.MkIff(eq, bound))
		case 1:
			body = f.must(m.MkImplies(eq, bound))
		case 2:
			body = f.must(m.MkXor(eq, bound))
		case 3:
			body = f.must(m.MkOrArgs([]ast.ASTHandle{eq, bound}))
		case 4:
			body = f.must(m.MkAndArgs([]ast.ASTHandle{eq, bound}))
		case 5:
			body = f.must(m.MkIte(eq, bound, f.must(m.MkNot(guard))))
		}
		truth := func(x int64) bool {
			var u [5]int64
			for j := range u {
				u[j] = a[j]*x + b[j]
			}
			g := u[0] > u[1]
			y := u[3]
			if g {
				y = u[2]
			}
			eq, le := y == u[4], u[3] <= u[0]
			switch trial % 6 {
			case 0:
				return eq == le
			case 1:
				return !eq || le
			case 2:
				return eq != le
			case 3:
				return eq || le
			case 4:
				return eq && le
			default:
				if eq {
					return le
				}
				return !g
			}
		}
		p := compileIntegerRegions(m, f.family, f.sort, body)
		if p == nil {
			t.Fatalf("trial %d did not compile", trial)
		}
		violated := false
		for x := int64(-41); x <= 41; x++ {
			got, next, ok := p.evaluate(x)
			if !ok || got != truth(x) || next <= x {
				t.Fatalf("trial %d at %d: truth %t next %d ok %t", trial, x, got, next, ok)
			}
			for y := x + 1; y < next && y <= 41; y++ {
				if truth(y) != got {
					t.Fatalf("trial %d skipped change at %d from %d", trial, y, x)
				}
			}
			violated = violated || !got
		}
		status, w := p.counterexample(m)
		if status == lbool.Undef || (status == lbool.True) != violated || (violated && truth(w)) {
			t.Fatalf("trial %d: status %v witness %d expected violation %t", trial, status, w, violated)
		}
	}
}

func TestIntegerRegionsLimitsAndSingletons(t *testing.T) {
	m := astTestManager(t)
	f := newRegionTestAST(t, m)
	for _, x := range []int64{-integerCoefficientLimit, -17, 0, 23, integerCoefficientLimit} {
		body := f.must(m.MkNot(f.must(m.MkEq(f.x, f.num(x)))))
		p := compileIntegerRegions(m, f.family, f.sort, body)
		got, w := p.counterexample(m)
		if got != lbool.True || w != x {
			t.Fatal("missed singleton", x, got, w)
		}
	}
	maxOffset := f.num(integerCoefficientLimit)
	nearTail := f.app(smtArithOpSub, f.x, maxOffset)
	body := f.app(smtArithOpLE, nearTail, maxOffset)
	p := compileIntegerRegions(m, f.family, f.sort, body)
	status, w := p.counterexample(m)
	if status != lbool.True || w != 2*integerCoefficientLimit+1 {
		t.Fatal("missed positive tail", w, status)
	}
	half := f.must(m.MkNot(f.must(m.MkEq(f.affine(-2, 0), f.num(3)))))
	p = compileIntegerRegions(m, f.family, f.sort, half)
	if status, _ = p.counterexample(m); status != lbool.False {
		t.Fatal("nonintegral root produced witness", status)
	}
	m.ResourceLimit().Cancel()
	status, _ = p.counterexample(m)
	m.ResourceLimit().DecCancel()
	if status != lbool.Undef {
		t.Fatal("canceled sweep", status)
	}
	for _, term := range []ast.ASTHandle{f.app(smtArithOpMul, f.x, f.x), f.app(smtArithOpMul, f.num(integerCoefficientLimit), f.num(2))} {
		body := f.must(m.MkEq(term, f.x))
		p := compileIntegerRegions(m, f.family, f.sort, body)
		if status, _ := p.counterexample(m); status != lbool.Undef {
			t.Fatal("unsupported product", status)
		}
	}
	foreign := f.must(m.MkVar(1, f.sort))
	real := f.must(m.MkFamilySort(f.family, smtArithRealSort, nil))
	for _, term := range []ast.ASTHandle{foreign, f.must(m.MkVar(0, real)), f.num(integerCoefficientLimit + 1), f.app(smtArithOpPower, f.x, f.num(2))} {
		body := f.must(m.MkEq(term, term))
		if p := compileIntegerRegions(m, f.family, f.sort, body); p != nil {
			t.Fatal("accepted unsupported term", term)
		}
	}
}

func TestSMTModelCheckerIntegerRegionsRespectAuxiliaryPremises(t *testing.T) {
	m, c, proto, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := newRegionTestAST(t, m)
	// The single exception is x=7. With a false assertion already in the
	// auxiliary base, its actual check of the negated body is unsatisfiable.
	left := f.app(smtArithOpSub, f.app(smtArithOpMul, f.num(2), f.x), f.num(3))
	body := f.must(m.MkNot(f.must(m.MkEq(left, f.num(11)))))
	q := f.must(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	astTestPin(t, m, q)
	astTestMust(t, c.state.Internalize(q, true))
	n := uint32(1)
	found, err := c.auxiliaryCounterexamples(proto, q, &n)
	astTestMust(t, err)
	if !found || n != 0 || len(c.newInstances) != 1 {
		t.Fatal("missing witness", n, found)
	}
	v, ok, err := m.ArithmeticSmallInteger(c.newInstances[0].bindings[0])
	if err != nil || !ok || v != 7 {
		t.Fatal("wrong witness", v, ok, err)
	}
	astTestMust(t, c.ResetNewInstances())
	astTestMust(t, c.auxiliaryContext.Assert(f.must(m.MkFalse())))
	n = 1
	found, err = c.auxiliaryCounterexamples(proto, q, &n)
	astTestMust(t, err)
	if found || n != 1 || c.HasNewInstances() {
		t.Fatal("ignored asserted auxiliary premise")
	}
}

func TestSMTModelCheckerIntegerRegionsResourceLimits(t *testing.T) {
	m, c, proto, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := newRegionTestAST(t, m)
	left := f.affine(-3, 2)
	body := f.must(m.MkNot(f.must(m.MkEq(left, f.num(-19)))))
	q := f.must(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	astTestPin(t, m, q)
	astTestMust(t, c.state.Internalize(q, true))
	astTestMust(t, c.initAuxiliaryContext())
	a := c.auxiliaryContext
	astTestMust(t, a.PushCore())
	defer func() { astTestMust(t, a.PopCore(1)) }()
	n := uint32(1)
	try := func(want lbool.LBool) {
		t.Helper()
		got, err := c.integerCounterexample(proto, q, q, body, &n)
		if err != nil || got != want {
			t.Fatalf("integer model check %v, %v; want %v", got, err, want)
		}
	}
	limit := m.ResourceLimit()
	limit.Limit = 100
	try(lbool.Undef)
	limit.Limit = 0
	a.kernel.smtState.MaxConflicts = 0
	try(lbool.Undef)
	a.kernel.smtState.MaxConflicts = ^uint32(0)
	limit.IncCancel()
	try(lbool.Undef)
	limit.DecCancel()
	if n != 1 || c.HasNewInstances() {
		t.Fatal("limited check emitted an instance")
	}
	try(lbool.True)
	if n != 0 || len(c.newInstances) != 1 {
		t.Fatal("missing enabled witness")
	}
	x, ok, err := m.ArithmeticSmallInteger(c.newInstances[0].bindings[0])
	if !ok || err != nil || x != 7 {
		t.Fatal("incorrect negative-slope witness", x, ok, err)
	}
}

// Re-evaluate the same depth of piecewise integer term found in an
// auxiliary model check. Construction and solving can be timed separately.
func BenchmarkSMTIntegerRegions(b *testing.B) {
	m := astTestManager(b)
	f := newRegionTestAST(b, m)
	piece := f.num(0)
	for x := int64(1); x <= 110; x++ {
		piece = f.must(m.MkIte(f.app(smtArithOpGE, f.x, f.num(x)), f.num(x), piece))
	}
	body := f.app(smtArithOpGE, piece, f.num(0))
	b.Run("CompileAndDecide", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			p := compileIntegerRegions(m, f.family, f.sort, body)
			if status, _ := p.counterexample(m); status != lbool.False {
				b.Fatal(status)
			}
		}
	})
	p := compileIntegerRegions(m, f.family, f.sort, body)
	b.Run("Decide", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if status, _ := p.counterexample(m); status != lbool.False {
				b.Fatal(status)
			}
		}
	})
}
