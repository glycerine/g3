package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func lraDirectValue(t *testing.T, f *lraFrontendFixture, h ast.ASTHandle, want *big.Rat) {
	t.Helper()
	got, present, err := f.s.GetTheoryValue(f.s.context.enodes.get(h))
	astTestMust(t, err)
	if !present {
		t.Fatal("missing direct theory value")
	}
	got = f.pin(got, nil)
	value, matched, err := f.a.numeral(got)
	astTestMust(t, err)
	if !matched || value.Cmp(want) != 0 {
		t.Fatal("direct theory value", value, want)
	}
}

func TestSMTLRADirectValueEligibilityAndSortDispatch(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-direct-x")
	lraFix(t, f, x, 2)
	term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(3), x), f.number(1))
	_, err := f.a.internalizeDef(term)
	astTestMust(t, err)
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "lra-direct-foreign"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	foreign := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
	_, err = f.a.internalizeDef(foreign)
	astTestMust(t, err)
	lraFix(t, f, foreign, 5)
	f.propagate(true)
	node := f.s.context.enodes.get(x)
	got, present, err := f.s.GetTheoryValue(node)
	astTestMust(t, err)
	if present || !got.IsNull() {
		t.Fatal("uncached proper column invented a numeral")
	}
	// Native terms are eligible even before init_model. Missing proper-column
	// cache entries are inserted as zero; the resulting term stays cached.
	lraDirectValue(t, f, term, big.NewRat(1, 1))
	astTestMust(t, f.a.initializeVariableValues())
	lraDirectValue(t, f, x, big.NewRat(2, 1))
	lraDirectValue(t, f, term, big.NewRat(1, 1))
	// The context dispatches the uninterpreted function by its Real sort,
	// despite the application's null declaration family.
	lraDirectValue(t, f, foreign, big.NewRat(5, 1))
	f.a.modelValues = nil
	astTestMust(t, f.a.initializeVariableValues())
	lraDirectValue(t, f, term, big.NewRat(7, 1))
	unattached := f.variable("lra-direct-unattached")
	astTestMust(t, f.s.Internalize(unattached, false))
	got, present, err = f.s.GetTheoryValue(f.s.context.enodes.get(unattached))
	astTestMust(t, err)
	if present || !got.IsNull() {
		t.Fatal("unattached variable has a value")
	}
	boolean := f.pin(f.m.MkTrue())
	astTestMust(t, f.s.Internalize(boolean, false))
	got, present, err = f.s.GetTheoryValue(f.s.context.enodes.get(boolean))
	astTestMust(t, err)
	if present || !got.IsNull() {
		t.Fatal("missing theory callback manufactured a value")
	}
	if _, _, err = f.s.GetTheoryValue(nil); err != ast.ErrASTDomain {
		t.Fatal("nil direct-value node", err)
	}
}

func TestSMTLRADirectValueIntegerAndStoppedOwnerGuards(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	var err error
	f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
	astTestMust(t, err)
	x := f.variable("lra-direct-fractional-int")
	v, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	index, err := f.a.numeric.getVarIndex(v)
	astTestMust(t, err)
	f.a.modelValues = map[uint32]*big.Rat{index: big.NewRat(3, 2)}
	if _, present, err := f.s.GetTheoryValue(f.s.context.enodes.get(x)); err == nil || present {
		t.Fatal("fractional Int value was rounded", present, err)
	}
	f.a.modelValues[index] = big.NewRat(3, 1)
	lraDirectValue(t, f, x, big.NewRat(3, 1))
	f.a.numeric.stopped = true
	if _, present, err := f.s.GetTheoryValue(f.s.context.enodes.get(x)); err != lp.ErrExactVectorPending || present {
		t.Fatal("stopped direct-value owner reused", present, err)
	}
}

func TestSMTLRADirectValueManagedQuickChecker(t *testing.T) {
	m, s, registry, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(a))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	f := &lraFrontendFixture{t, m, s, a, sort}
	x := f.variable("lra-quick-value-x")
	lraFix(t, f, x, 2)
	f.propagate(true)
	astTestMust(t, a.initializeVariableValues())
	variable := f.pin(m.MkVar(0, sort))
	sum := f.app(smtArithOpAdd, variable, f.number(1))
	for _, n := range []int64{3, 4} {
		eq := f.pin(m.MkEq(sum, f.number(n)))
		quantifier := f.pin(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "lra-quick-r")}, Body: eq}))
		astTestMust(t, s.Internalize(quantifier, true))
		generated, err := manager.quick.InstantiateNotSatCandidates(quantifier, []ast.ASTHandle{x})
		astTestMust(t, err)
		if generated != (n == 4) {
			t.Fatal("managed quick checker failed arithmetic substitution", n, generated)
		}
		if len(manager.quick.newExprs) != 0 || manager.quick.canonCache != nil {
			t.Fatal("quick query retained temporary references")
		}
	}
	if !manager.CanPropagate() {
		t.Fatal("counterexample instance was not queued")
	}
	astTestMust(t, manager.Propagate())
	if registry.schedule.Stats().Instances != 1 || manager.CanPropagate() {
		t.Fatal("unexpected quick instances", registry.schedule.Stats())
	}
}

func TestSMTLRADirectValueQuickCheckerRootsEveryArgument(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	rw := smtQuickTheoryRewriter(t, f.m)
	registry, _ := smtQuickRegistry(t, f.m)
	q, err := NewSMTQuickChecker(f.s.context, rw, registry)
	astTestMust(t, err)
	q.state = f.s
	t.Cleanup(func() { astTestMust(t, q.clearQuery()) })
	x, y := f.variable("lra-quick-root-x"), f.variable("lra-quick-root-y")
	lraFix(t, f, x, 2)
	lraFix(t, f, y, 5)
	f.propagate(true)
	astTestMust(t, f.a.initializeVariableValues())
	sum := f.app(smtArithOpAdd, x, y)
	q.canonCache = make(map[ast.ASTHandle]ast.ASTHandle)
	result, err := q.canonize(sum)
	astTestMust(t, err)
	value, matched, err := f.a.numeral(result)
	astTestMust(t, err)
	if !matched || value.Cmp(big.NewRat(7, 1)) != 0 || len(q.newExprs) != 3 {
		t.Fatal("argument value refs were not retained", value, len(q.newExprs))
	}
	refs := append([]ast.ASTRef(nil), q.newExprs...)
	astTestMust(t, q.clearQuery())
	for _, ref := range refs {
		if _, err = f.m.RefGet(ref); err == nil {
			t.Fatal("query value root survived cleanup")
		}
	}
}
