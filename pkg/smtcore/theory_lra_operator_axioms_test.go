package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestSMTLRAOperatorConstantDivisionThroughRelevance(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-axiom-div-x")
	lraFix(t, f, x, 6)
	f.propagate(true)
	div := f.app(smtArithOpDiv, x, f.number(2))
	_, err := f.a.internalizeDef(div)
	astTestMust(t, err)
	astTestMust(t, f.s.markRelevant(div))
	astTestMust(t, f.s.relevancy.Propagate())
	f.propagate(true)
	// mk_div_axiom's eqz is th.mk_eq(2, 0): theory_lra keeps theory::
	// mk_eq_atom, so (= 0 2) is an ordinary atom that search refutes; assert
	// that refutation here so the axiom's other disjunct fixes the quotient.
	eqz, err := f.s.MkEqAtom(f.number(2), f.number(0))
	astTestMust(t, err)
	f.assert(eqz, false)
	f.propagate(true)
	v := f.s.context.enodes.get(div).TheoryVar(f.a.FamilyID())
	value, err := f.a.numeric.value(v)
	astTestMust(t, err)
	checkInfiniteArithmetic(t, value, 0, 3, 0)
	if !f.a.notHandled.IsNull() {
		t.Fatal("constant division was spuriously refused", f.a.notHandled)
	}
	astTestMust(t, f.s.PushScope())
	f.assert(f.app(smtArithOpLE, div, f.number(2)), true)
	f.propagate(false)
	astTestMust(t, f.s.PopScopes(1))
	f.propagate(true)
}

func TestSMTLRAOperatorDivisionZeroAndSymbolicGuard(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-axiom-symbolic-x"), f.variable("lra-axiom-symbolic-y")
	_, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	_, err = f.a.internalizeDef(y)
	astTestMust(t, err)
	zero := f.number(0)
	divZero := f.app(smtArithOpDiv, x, zero)
	_, err = f.a.internalizeDef(divZero)
	astTestMust(t, err)
	before := len(f.s.Clauses())
	astTestMust(t, f.s.markRelevant(divZero))
	astTestMust(t, f.s.relevancy.Propagate())
	if len(f.s.Clauses()) != before {
		t.Fatal("division by zero received a nonzero axiom")
	}
	astTestMust(t, f.s.PushScope())
	div := f.app(smtArithOpDiv, x, y)
	_, err = f.a.internalizeDef(div)
	astTestMust(t, err)
	astTestMust(t, f.s.markRelevant(div))
	product := f.app(smtArithOpMul, y, div)
	equality := f.pin(f.s.mkEqualityAtom(product, x))
	guard := f.pin(f.s.mkEqualityAtom(y, zero))
	eqLit, err := f.s.Literal(equality)
	astTestMust(t, err)
	guardLit, err := f.s.Literal(guard)
	astTestMust(t, err)
	want := boundClauseSet([]SMTLiteral{guardLit, eqLit})
	found := false
	for _, clause := range f.s.Clauses() {
		if boundClauseSet(clause.Literals) == want {
			found = true
		}
	}
	if !found {
		t.Fatal("missing q=0 or q*(p/q)=p", want)
	}
	f.assert(guard, false)
	f.propagate(true)
	value, err := f.s.Assignment(eqLit)
	astTestMust(t, err)
	if value != lbool.True || !f.s.relevancy.IsRelevant(equality) {
		t.Fatal("nonzero guard did not activate division equality", value)
	}
	if f.a.notHandled.IsNull() {
		t.Fatal("nonlinear symbolic division refusal was lost")
	}
	astTestMust(t, f.s.PopScopes(1))
	if len(f.a.axiomRefs) != 0 {
		t.Fatal("division watch owner survived pop")
	}
}

func TestSMTLRAOperatorNegatedWatchTargetOwnership(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	a := f.pin(f.m.MkConst(astTestSymbol(t, f.m, "lra-watch-a"), boolean))
	b := f.pin(f.m.MkConst(astTestSymbol(t, f.m, "lra-watch-b"), boolean))
	astTestMust(t, f.s.Internalize(a, true))
	astTestMust(t, f.s.Internalize(b, true))
	la, err := f.s.Literal(a)
	astTestMust(t, err)
	lb, err := f.s.Literal(b)
	astTestMust(t, err)
	astTestMust(t, f.s.PushScope())
	c := smtLRAAxiomCall{owner: f.a}
	astTestMust(t, c.binary(la, lb.Negated()))
	astTestMust(t, c.close())
	if len(f.a.axiomRefs) != 1 {
		t.Fatal("negative watch target was not owned")
	}
	target, err := f.m.RefGet(f.a.axiomRefs[0])
	astTestMust(t, err)
	_, err = f.m.GetKind(target)
	astTestMust(t, err)
	astTestMust(t, f.s.AssignLiteral(la.Negated(), NewSMTBooleanAxiom()))
	f.propagate(true)
	if !f.s.relevancy.IsRelevant(b) {
		t.Fatal("negative watch did not make consequent relevant")
	}
	astTestMust(t, f.s.PopScopes(1))
	if len(f.a.axiomRefs) != 0 {
		t.Fatal("watch ref was not retired")
	}
	if f.m.IsLiveHandle(target) {
		t.Fatal("temporary NOT target remained pinned after pop")
	}
}

func TestSMTLRAOperatorProfileRejectsDeadEnumeration(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	config := f.a.config
	config.EnumConstMod = true
	if err := f.a.ConfigureArithmetic(config); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unused source enumeration profile accepted", err)
	}
}

func TestSMTLRAOperatorIntegerDivisionRemainderClauseShapes(t *testing.T) {
	for _, op := range []int32{smtArithOpMod, smtArithOpRem} {
		f := newLRAFrontendFixture(t, true)
		var err error
		f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
		astTestMust(t, err)
		x, y := f.variable("lra-integer-axiom-x"), f.variable("lra-integer-axiom-y")
		h := f.app(op, x, y)
		astTestMust(t, f.s.Internalize(h, false))
		astTestMust(t, f.s.markRelevant(h))
		zero := f.pin(f.m.ArithmeticNumeralRat(new(big.Rat), true))
		mod := f.app(smtArithOpMod, x, y)
		literal := func(h ast.ASTHandle) SMTLiteral { l, err := f.s.Literal(h); astTestMust(t, err); return l }
		equality := func(a, b ast.ASTHandle) SMTLiteral { return literal(f.pin(f.s.mkEqualityAtom(a, b))) }
		ge := literal(f.app(smtArithOpGE, y, zero))
		var want [][2]SMTLiteral
		if op == smtArithOpRem {
			want = [][2]SMTLiteral{{ge.Negated(), equality(h, mod)}, {ge, equality(h, f.app(smtArithOpUMinus, mod))}}
		} else {
			le := literal(f.app(smtArithOpLE, y, zero))
			div := f.app(smtArithOpIDiv, x, y)
			eq := equality(f.app(smtArithOpAdd, f.app(smtArithOpMul, y, div), mod), x)
			modGE := literal(f.app(smtArithOpGE, mod, zero))
			upperPositive := literal(f.app(smtArithOpGE, f.app(smtArithOpSub, mod, y), zero))
			upperNegative := literal(f.app(smtArithOpGE, f.app(smtArithOpAdd, mod, y), zero))
			want = [][2]SMTLiteral{{ge, eq}, {le, eq}, {ge, modGE}, {le, modGE}, {le, upperPositive.Negated()}, {ge, upperNegative.Negated()}}
		}
		clauses := make(map[string]bool)
		for _, c := range f.s.Clauses() {
			clauses[boundClauseSet(c.Literals)] = true
		}
		for _, pair := range want {
			if !clauses[boundClauseSet(pair[:])] {
				t.Fatal("missing native integer operator axiom", op, pair)
			}
		}
	}
}

func TestSMTLRAOperatorFloorAndIsIntAxioms(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-floor-axiom-x")
	cast := f.app(smtArithOpToInt, x)
	astTestMust(t, f.s.Internalize(cast, false))
	astTestMust(t, f.s.markRelevant(cast))
	back := f.app(smtArithOpToReal, cast)
	lo := f.app(smtArithOpLE, f.app(smtArithOpSub, back, x), f.number(0))
	hi := f.app(smtArithOpGE, f.app(smtArithOpSub, x, back), f.number(1))
	for i, h := range []ast.ASTHandle{lo, hi} {
		l, err := f.s.Literal(h)
		astTestMust(t, err)
		v, err := f.s.Assignment(l)
		astTestMust(t, err)
		want := lbool.True
		if i == 1 {
			want = lbool.False
		}
		if v != want {
			t.Fatal("floor unit polarity", i, v, want)
		}
	}
	isInt := f.app(smtArithOpIsInt, x)
	astTestMust(t, f.s.Internalize(isInt, true))
	astTestMust(t, f.s.markRelevant(isInt))
	eq := f.pin(f.s.mkEqualityAtom(back, x))
	a, err := f.s.Literal(isInt)
	astTestMust(t, err)
	b, err := f.s.Literal(eq)
	astTestMust(t, err)
	clauses := make(map[string]bool)
	for _, c := range f.s.Clauses() {
		clauses[boundClauseSet(c.Literals)] = true
	}
	for _, pair := range [][2]SMTLiteral{{a.Negated(), b}, {a, b.Negated()}} {
		if !clauses[boundClauseSet(pair[:])] {
			t.Fatal("missing is_int equivalence", pair)
		}
	}
}

func TestSMTLRAOperatorToIntToRealRoundTrip(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	var err error
	f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
	astTestMust(t, err)
	x := f.variable("lra-cast-roundtrip-x")
	_, err = f.a.internalizeDef(x)
	astTestMust(t, err)
	cast := f.app(smtArithOpToInt, f.app(smtArithOpToReal, x))
	astTestMust(t, f.s.Internalize(cast, false))
	astTestMust(t, f.s.markRelevant(cast))
	eq := f.pin(f.s.mkEqualityAtom(x, cast))
	lit, err := f.s.Literal(eq)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("to_int(to_real x)=x unit axiom missing", value)
	}
}

func TestSMTLRAOperatorZeroModAndNoDirectIDivCallback(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	var err error
	f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
	astTestMust(t, err)
	x, y := f.variable("lra-zero-mod-x"), f.variable("lra-idiv-y")
	zero := f.pin(f.m.ArithmeticNumeralRat(new(big.Rat), true))
	for _, h := range []ast.ASTHandle{f.app(smtArithOpMod, x, zero), f.app(smtArithOpIDiv, x, y)} {
		astTestMust(t, f.s.Internalize(h, false))
		before := len(f.s.Clauses())
		astTestMust(t, f.s.markRelevant(h))
		if len(f.s.Clauses()) != before {
			t.Fatal("source absent operator axiom was invented")
		}
	}
}

func TestSMTLRAOperatorPhysicalRefusalAndOwnerCleanup(t *testing.T) {
	refusals, successes := 0, 0
	for _, stop := range []int64{-1, 0, 1, 2, 4, 8, 16, 32} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m := astTestManager(t)
			// Registered before context cleanup, so this census runs after
			// context-owned and caller-owned references have both retired.
			t.Cleanup(func() {
				if m.Stats().LiveReferences != 0 {
					t.Fatal("operator AST references leaked", m.Stats().LiveReferences)
				}
			})
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, s := smtInternalizerFixture(t, m, nil)
			a, err := NewSMTLRATheory(family)
			astTestMust(t, err)
			astTestMust(t, s.RegisterTheory(a))
			sort, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			f := &lraFrontendFixture{t, m, s, a, sort}
			x := f.variable("lra-operator-refusal-x")
			two := f.number(2)
			div := f.app(smtArithOpDiv, x, two)
			_, err = f.a.internalizeDef(x)
			astTestMust(t, err)
			_, err = f.a.internalizeDef(div)
			astTestMust(t, err)
			var padding []ast.ASTRef
			t.Cleanup(func() {
				f.m.Budget().FailAfter(-1)
				for i := range padding {
					astTestMust(t, f.m.DelRef(&padding[i]))
				}
			})
			f.m.Budget().FailAfter(0)
			cold := false
			for i := 0; i < 8192; i++ {
				ref, err := f.m.NewRef(div)
				if err != nil {
					cold = errors.Is(err, arena.ErrInjectedFailure)
					break
				}
				padding = append(padding, ref)
			}
			f.m.Budget().FailAfter(-1)
			if !cold {
				t.Fatal("did not reach a cold AST reference frontier")
			}
			f.m.Budget().FailAfter(stop)
			err = f.a.Relevant(div)
			f.m.Budget().FailAfter(-1)
			if err != nil {
				var failure umemory.Failure
				memout := errors.As(err, &failure) && failure.Kind == umemory.FailureMemout && failure.ErrorCode() == 101 && stop >= 0
				if !errors.Is(err, arena.ErrInjectedFailure) && !memout {
					t.Fatal(err)
				}
				refusals++
			} else {
				successes++
			}
		})
	}
	if refusals == 0 || successes == 0 {
		t.Fatal("operator samples require physical refusal and success", refusals, successes)
	}
}
