package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func lraFix(t *testing.T, f *lraFrontendFixture, h ast.ASTHandle, value int64) []SMTLiteral {
	t.Helper()
	var literals []SMTLiteral
	for _, op := range []int32{smtArithOpGE, smtArithOpLE} {
		bound := f.app(op, h, f.number(value))
		f.assert(bound, true)
		lit, err := f.s.Literal(bound)
		astTestMust(t, err)
		literals = append(literals, lit)
	}
	return literals
}

func TestSMTLRAFixedColumnsEqualityEvidenceAndPop(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			if proofs {
				astTestMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
			}
			x, y := f.variable("lra-fixed-x"), f.variable("lra-fixed-y")
			_, err := f.a.internalizeDef(x)
			astTestMust(t, err)
			_, err = f.a.internalizeDef(y)
			astTestMust(t, err)
			astTestMust(t, f.s.PushScope())
			want := lraFix(t, f, x, 7)
			want = append(want, lraFix(t, f, y, 7)...)
			f.propagate(true)
			if f.s.context.enodes.get(x).Root() != f.s.context.enodes.get(y).Root() || f.a.fixedEqualities != 1 {
				t.Fatal("fixed columns were not merged", f.a.fixedEqualities)
			}
			found := false
			for _, j := range f.s.ownedJustifications {
				if reason, ok := j.(*SMTExtTheoryEqualityPropagationJustification); ok {
					if len(reason.Parameters()) != 0 || len(reason.Equalities) != 0 || len(reason.Literals) != 4 || boundClauseSet(reason.Literals) != boundClauseSet(want) {
						t.Fatal("fixed equality lost original four inequalities", reason.Literals, want)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("missing fixed equality justification")
			}
			astTestMust(t, f.s.PopScopes(1))
			if f.s.context.enodes.get(x).Root() == f.s.context.enodes.get(y).Root() {
				t.Fatal("fixed equality survived its bound scope")
			}
			if len(f.a.fixedVariables) != 1 {
				t.Fatal("native fixed cache was incorrectly scoped")
			}
		})
	}
}

func TestSMTLRAFixedEqualityCongruenceConflictAndPop(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-fixed-cc-x"), f.variable("lra-fixed-cc-y")
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "lra-fixed-cc-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	fx := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
	fy := f.pin(f.m.MkApp(decl, []ast.ASTHandle{y}))
	astTestMust(t, f.s.Internalize(fx, false))
	astTestMust(t, f.s.Internalize(fy, false))
	lraFix(t, f, x, 7)
	lraFix(t, f, y, 7)
	f.propagate(true)
	if f.s.context.enodes.get(fx).Root() != f.s.context.enodes.get(fy).Root() || f.a.fixedEqualities != 1 {
		t.Fatal("fixed equality did not reach congruence closure")
	}
	astTestMust(t, f.s.PushScope())
	eq := f.pin(f.s.mkEqualityAtom(fx, fy))
	f.assert(eq, false)
	f.propagate(false)
	astTestMust(t, f.s.PopScopes(1))
	f.propagate(true)
	if f.s.context.enodes.get(fx).Root() != f.s.context.enodes.get(fy).Root() {
		t.Fatal("root congruence equality lost on child pop")
	}
}

func TestSMTLRAFixedNumeralImplicitWitnesses(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-fixed-number-x")
	constant := f.number(7)
	v, err := f.a.internalizeDef(constant)
	astTestMust(t, err)
	index, ok := f.a.numeric.index(v)
	if !ok {
		t.Fatal("missing numeral term")
	}
	for _, lower := range []bool{false, true} {
		witness, found, err := f.a.hasFixedBound(index, big.NewRat(7, 1), lower)
		astTestMust(t, err)
		if !found || witness != smtLRANoIndex {
			t.Fatal("numeral did not supply an implicit hard bound", witness, found)
		}
	}
	want := lraFix(t, f, x, 7)
	f.propagate(true)
	if f.s.context.enodes.get(x).Root() != f.s.context.enodes.get(constant).Root() {
		t.Fatal("fixed column did not merge with cached numeral")
	}
	found := false
	for _, j := range f.s.ownedJustifications {
		if reason, ok := j.(*SMTExtTheoryEqualityPropagationJustification); ok {
			if len(reason.Literals) != 2 || boundClauseSet(reason.Literals) != boundClauseSet(want) {
				t.Fatal("implicit numeral invented constraints", reason.Literals, want)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing numeral equality reason")
	}
}

func TestSMTLRAFixedTermBoundsTrailAndEquality(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y, z := f.variable("lra-fixed-term-x"), f.variable("lra-fixed-term-y"), f.variable("lra-fixed-term-z")
	u := f.app(smtArithOpAdd, x, y)
	v := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(2), x), z)
	uvar, err := f.a.internalizeDef(u)
	astTestMust(t, err)
	vvar, err := f.a.internalizeDef(v)
	astTestMust(t, err)
	ui, _ := f.a.numeric.index(uvar)
	vi, _ := f.a.numeric.index(vvar)
	f.assert(f.app(smtArithOpGE, u, f.number(7)), true)
	f.assert(f.app(smtArithOpGE, u, f.number(3)), true) // later weaker bound must not replace7
	f.propagate(true)
	witness, found, err := f.a.hasFixedBound(ui, big.NewRat(7, 1), true)
	astTestMust(t, err)
	if !found || witness == smtLRANoIndex {
		t.Fatal("missing parent term-bound metadata")
	}
	astTestMust(t, f.s.PushScope())
	f.assert(f.app(smtArithOpLE, u, f.number(7)), true)
	f.assert(f.app(smtArithOpLE, u, f.number(9)), true) // likewise for the upper directory
	lraFix(t, f, v, 7)
	f.propagate(true)
	if f.s.context.enodes.get(u).Root() != f.s.context.enodes.get(v).Root() {
		t.Fatal("fixed compound terms were not merged")
	}
	astTestMust(t, f.s.PopScopes(1))
	if f.s.context.enodes.get(u).Root() == f.s.context.enodes.get(v).Root() {
		t.Fatal("compound equality survived child scope")
	}
	actual, found, err := f.a.hasFixedBound(ui, big.NewRat(7, 1), true)
	astTestMust(t, err)
	if !found || actual != witness {
		t.Fatal("parent term witness changed after pop", actual, witness)
	}
	for _, index := range []uint32{ui, vi} {
		_, found, err = f.a.hasFixedBound(index, big.NewRat(7, 1), false)
		astTestMust(t, err)
		if found {
			t.Fatal("child term upper bound survived pop")
		}
	}
}

func TestSMTLRAFixedCacheRevalidatesPoppedColumn(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y, z := f.variable("lra-stale-x"), f.variable("lra-stale-y"), f.variable("lra-stale-z")
	for _, h := range []ast.ASTHandle{x, y, z} {
		_, err := f.a.internalizeDef(h)
		astTestMust(t, err)
	}
	astTestMust(t, f.s.PushScope())
	lraFix(t, f, x, 5)
	f.propagate(true)
	astTestMust(t, f.s.PopScopes(1))
	lraFix(t, f, y, 5)
	lraFix(t, f, z, 5)
	f.propagate(true)
	if f.s.context.enodes.get(x).Root() == f.s.context.enodes.get(y).Root() || f.s.context.enodes.get(y).Root() == f.s.context.enodes.get(z).Root() || f.a.fixedEqualities != 0 {
		t.Fatal("stale fixed cache was used or silently replaced")
	}
	// Source does not replace a still-live cached variable just because its
	// old bounds disappeared; the later model-combination phase handles this.
	key, err := f.a.fixedKey(f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID()), big.NewRat(5, 1))
	astTestMust(t, err)
	if f.a.fixedVariables[key] != f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID()) {
		t.Fatal("source stale-cache retention changed")
	}
}

func TestSMTLRAFixedCacheReplacesPoppedVariableID(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	y := f.variable("lra-replace-fixed-y")
	_, err := f.a.internalizeDef(y)
	astTestMust(t, err)
	astTestMust(t, f.s.PushScope())
	x := f.variable("lra-replace-fixed-x")
	lraFix(t, f, x, 5)
	f.propagate(true)
	astTestMust(t, f.s.PopScopes(1))
	lraFix(t, f, y, 5)
	f.propagate(true)
	z := f.variable("lra-replace-fixed-z")
	lraFix(t, f, z, 5)
	f.propagate(true)
	if f.s.context.enodes.get(y).Root() != f.s.context.enodes.get(z).Root() || f.a.fixedEqualities != 1 {
		t.Fatal("popped cached variable ID was not replaced safely", f.a.fixedEqualities)
	}
}

func TestSMTLRAFixedSortKeysAndIndependentBoundMode(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-fixed-key-real")
	xv, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	old := f.sort
	f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
	astTestMust(t, err)
	y := f.variable("lra-fixed-key-int")
	yv, err := f.a.internalizeDef(y)
	astTestMust(t, err)
	f.sort = old
	key1, err := f.a.fixedKey(xv, big.NewRat(5, 1))
	astTestMust(t, err)
	key2, err := f.a.fixedKey(yv, big.NewRat(5, 1))
	astTestMust(t, err)
	if key1 == key2 || key1.integer || !key2.integer {
		t.Fatal("Int and Real fixed-value keys were conflated")
	}
	config := f.a.config
	config.BoundProp = 0
	astTestMust(t, f.a.ConfigureArithmetic(config))
	z := f.variable("lra-fixed-key-real-z")
	lraFix(t, f, x, 5)
	lraFix(t, f, z, 5)
	f.propagate(true)
	if f.s.context.enodes.get(x).Root() != f.s.context.enodes.get(z).Root() || f.a.fixedEqualities != 1 {
		t.Fatal("bound propagation mode incorrectly disabled fixed equalities")
	}
}

func TestSMTLRAFixedEqualityConfigurationAndStrictness(t *testing.T) {
	for _, threshold := range []bool{false, true} {
		t.Run(fmt.Sprint(threshold), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			config := f.a.config
			if threshold {
				config.PropagationThreshold = 0
			} else {
				config.PropagateEqs = false
			}
			astTestMust(t, f.a.ConfigureArithmetic(config))
			x, y := f.variable("lra-disabled-fixed-x"), f.variable("lra-disabled-fixed-y")
			lraFix(t, f, x, 2)
			lraFix(t, f, y, 2)
			f.propagate(true)
			if f.s.context.enodes.get(x).Root() == f.s.context.enodes.get(y).Root() || f.a.fixedEqualities != 0 {
				t.Fatal("disabled fixed equality propagated")
			}
		})
	}
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-strict-fixed-x")
	f.assert(f.app(smtArithOpLE, x, f.number(1)), false)
	f.propagate(true)
	v := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	index, _ := f.a.numeric.index(v)
	_, found, err := f.a.hasFixedBound(index, big.NewRat(1, 1), true)
	astTestMust(t, err)
	if found {
		t.Fatal("strict infinitesimal lower bound was treated as exact")
	}
	if f.a.fixedEqualities != 0 {
		t.Fatal("strict bound generated fixed equality")
	}
	status, err := f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("feasible strict real bound did not finish", status)
	}
}
