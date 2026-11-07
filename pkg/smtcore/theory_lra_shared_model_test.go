package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSMTLRASharedCandidatesOrderPhaseAndScope(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	config := f.a.config
	config.PropagateEqs = false
	astTestMust(t, f.a.ConfigureArithmetic(config))
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "lra-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	vars := []ast.ASTHandle{f.variable("lra-shared-x"), f.variable("lra-shared-y"), f.variable("lra-shared-z")}
	for _, v := range vars {
		app := f.pin(f.m.MkApp(decl, []ast.ASTHandle{v}))
		astTestMust(t, f.s.Internalize(app, false))
		lraFix(t, f, v, 1)
	}
	f.propagate(true)
	for _, v := range vars {
		astTestMust(t, f.s.relevancy.MarkRelevant(v))
	}
	astTestMust(t, f.s.relevancy.Propagate())
	astTestMust(t, f.s.PushScope())
	random := f.s.random
	start := int(random.Next()) % len(vars)
	progress, err := f.a.assumeSharedEqualities()
	astTestMust(t, err)
	if !progress || len(f.a.assumeCandidates) != 2 || f.a.assumeHead != 1 {
		t.Fatal("missing scoped shared candidates", progress, f.a.assumeCandidates, f.a.assumeHead)
	}
	for i, pair := range f.a.assumeCandidates {
		wantLeft := f.s.context.enodes.get(vars[(start+i+1)%len(vars)]).TheoryVar(f.a.FamilyID())
		wantRight := f.s.context.enodes.get(vars[start]).TheoryVar(f.a.FamilyID())
		if pair.left != wantLeft || pair.right != wantRight {
			t.Fatal("native cyclic candidate order", pair, wantLeft, wantRight)
		}
	}
	pair := f.a.assumeCandidates[0]
	left, _ := f.a.Enode(pair.left)
	right, _ := f.a.Enode(pair.right)
	eq := f.pin(f.m.MkEq(left.owner, right.owner))
	lit, err := f.s.Literal(eq)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	data, err := f.s.BooleanData(lit.Var())
	astTestMust(t, err)
	if value != lbool.Undef || !data.TryTrueFirst() || left.Root() == right.Root() {
		t.Fatal("shared model equality was forced instead of proposed", value)
	}
	progress, err = f.a.delayedAssumeEqualities()
	astTestMust(t, err)
	if !progress || f.a.assumeHead != 2 {
		t.Fatal("second shared candidate was lost")
	}
	astTestMust(t, f.s.PopScopes(1))
	if len(f.a.assumeCandidates) != 0 || f.a.assumeHead != 0 || f.a.NumVariables() != 3 {
		t.Fatal("shared candidate state survived pop", len(f.a.assumeCandidates), f.a.assumeHead, f.a.NumVariables())
	}
}

func TestSMTLRASharedUnderspecifiedParentScan(t *testing.T) {
	for _, many := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x, y := f.variable("lra-shared-div-x"), f.variable("lra-shared-div-y")
		_, err := f.a.internalizeDef(x)
		astTestMust(t, err)
		div := f.app(smtArithOpDiv, x, y)
		_, err = f.a.internalizeDef(div)
		astTestMust(t, err)
		if many {
			for _, n := range []int64{2, 3, 4, 5} {
				_, err = f.a.internalizeDef(f.app(smtArithOpAdd, x, f.number(n)))
				astTestMust(t, err)
			}
		}
		astTestMust(t, f.s.relevancy.MarkRelevant(x))
		astTestMust(t, f.s.relevancy.Propagate())
		shared, err := f.a.relevantShared(f.s.context.enodes.get(x))
		astTestMust(t, err)
		if !shared {
			t.Fatal("underspecified child was not shared", many, len(f.s.context.enodes.get(x).Root().parents))
		}
		parents := len(f.s.context.enodes.get(x).Root().parents)
		if many != (parents > 2*len(f.a.underspecified)) {
			t.Fatal("test did not reach both native sharing branches", many, parents, len(f.a.underspecified))
		}
	}
}

func TestSMTLRADelayedCandidateRechecksCurrentValues(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	config := f.a.config
	config.PropagateEqs = false
	astTestMust(t, f.a.ConfigureArithmetic(config))
	x, y := f.variable("lra-delayed-x"), f.variable("lra-delayed-y")
	lraFix(t, f, x, 1)
	f.assert(f.app(smtArithOpGE, y, f.number(1)), true)
	f.propagate(true)
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	yv := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
	f.a.assumeCandidates = append(f.a.assumeCandidates, smtArithmeticEqualityCandidate{xv, yv})
	f.assert(f.app(smtArithOpGE, y, f.number(2)), true)
	f.propagate(true)
	progress, err := f.a.delayedAssumeEqualities()
	astTestMust(t, err)
	if progress || f.a.assumeHead != 1 || f.a.equalityAdapter.NumAxioms() != 0 {
		t.Fatal("stale model equality was proposed")
	}
}

func TestSMTLRASharedRandomizationSeparatesFlexibleValues(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-random-shared-x"), f.variable("lra-random-shared-y")
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "lra-random-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	for _, v := range []ast.ASTHandle{x, y} {
		app := f.pin(f.m.MkApp(decl, []ast.ASTHandle{v}))
		astTestMust(t, f.s.Internalize(app, false))
		f.assert(f.app(smtArithOpGE, v, f.number(0)), true)
		f.assert(f.app(smtArithOpLE, v, f.number(10)), true)
	}
	f.propagate(true)
	for _, v := range []ast.ASTHandle{x, y} {
		astTestMust(t, f.s.relevancy.MarkRelevant(v))
	}
	astTestMust(t, f.s.relevancy.Propagate())
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	yv := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
	x0, err := f.a.numeric.value(xv)
	astTestMust(t, err)
	y0, err := f.a.numeric.value(yv)
	astTestMust(t, err)
	if x0.Cmp(&y0) != 0 {
		t.Fatal("fixture did not begin with colliding values")
	}
	progress, err := f.a.assumeSharedEqualities()
	astTestMust(t, err)
	x1, err := f.a.numeric.value(xv)
	astTestMust(t, err)
	y1, err := f.a.numeric.value(yv)
	astTestMust(t, err)
	if progress || len(f.a.assumeCandidates) != 0 || x1.Cmp(&y1) == 0 {
		t.Fatal("actual LAR randomization was skipped", progress, x1, y1)
	}
}

func TestSMTLRADelayedCandidateKeepsEpsilonDistinct(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	config := f.a.config
	config.PropagateEqs = false
	astTestMust(t, f.a.ConfigureArithmetic(config))
	x, y := f.variable("lra-epsilon-shared-x"), f.variable("lra-epsilon-shared-y")
	lraFix(t, f, x, 1)
	f.assert(f.app(smtArithOpGE, y, f.number(0)), true)
	f.assert(f.app(smtArithOpGE, y, f.number(1)), false)
	f.propagate(true)
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	yv := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
	_, err := f.a.numeric.maximize(yv)
	astTestMust(t, err)
	a, err := f.a.numeric.value(xv)
	astTestMust(t, err)
	b, err := f.a.numeric.value(yv)
	astTestMust(t, err)
	if a.Main.Cmp(&b.Main) != 0 || a.Cmp(&b) == 0 {
		t.Fatal("fixture did not reach equal rationals with distinct epsilon")
	}
	f.a.assumeCandidates = append(f.a.assumeCandidates, smtArithmeticEqualityCandidate{xv, yv})
	progress, err := f.a.delayedAssumeEqualities()
	astTestMust(t, err)
	if progress || f.a.assumeHead != 1 {
		t.Fatal("epsilon was discarded from interface-equality recheck")
	}
}

func lraModelFixture(t *testing.T) (*lraFrontendFixture, *SMTModelGenerator) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(a))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	g, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	astTestMust(t, g.SetContext(s.context))
	astTestMust(t, g.InitializeModel(cfg))
	return &lraFrontendFixture{t, m, s, a, sort}, g
}

func TestSMTLRAModelStrictValuesTermFactoryAndOwnership(t *testing.T) {
	f, g := lraModelFixture(t)
	x := f.variable("lra-model-x")
	f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
	f.assert(f.app(smtArithOpGE, x, f.number(1)), false)
	term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(3), x), f.number(2))
	_, err := f.a.internalizeDef(term)
	astTestMust(t, err)
	f.propagate(true)
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	_, err = f.a.numeric.maximize(xv)
	astTestMust(t, err)
	astTestMust(t, f.a.InitializeModel(g))
	var values [2]*big.Rat
	var handles [2]ast.ASTHandle
	procedures := make(map[*SMTEnode]SMTModelValueProcedure)
	var roots []*SMTEnode
	for i, h := range []ast.ASTHandle{x, term} {
		procedure, err := f.a.ModelValue(f.s.context.enodes.get(h), g)
		astTestMust(t, err)
		root := f.s.context.enodes.get(h).Root()
		roots = append(roots, root)
		procedures[root] = procedure
		handles[i], err = procedure.MakeValue(g, nil)
		astTestMust(t, err)
		var matched bool
		values[i], matched, err = f.a.numeral(handles[i])
		astTestMust(t, err)
		if !matched {
			t.Fatal("model factory did not produce a rational numeral")
		}
	}
	if values[0].Sign() < 0 || values[0].Cmp(big.NewRat(1, 1)) >= 0 {
		t.Fatal("strict LAR delta was discarded", values[0])
	}
	want := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(3, 1), values[0]), big.NewRat(2, 1))
	if values[1].Cmp(want) != 0 {
		t.Fatal("model term lost coefficients/constant", values[1], want)
	}
	astTestMust(t, g.BuildValues(roots, procedures))
	if len(g.rootValues) != 2 {
		t.Fatal("model generator did not publish arithmetic root values")
	}
	factory := g.model.Factory(f.a.FamilyID())
	if factory != f.a.modelOwner.factory {
		t.Fatal("generator did not acquire arithmetic factory")
	}
	astTestMust(t, f.a.Close())
	for _, h := range handles {
		_, err = f.m.GetSort(h)
		astTestMust(t, err)
	}
	if g.model.Factory(factory.FamilyID()) != factory {
		t.Fatal("theory closed the model-owned factory")
	}
	_, err = factory.FreshValue(f.sort)
	astTestMust(t, err)
}

func TestSMTLRAModelMissingColumnCacheAndFeasibilityReset(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-model-cache-x")
	term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(3), x), f.number(2))
	v, err := f.a.internalizeDef(term)
	astTestMust(t, err)
	lraFix(t, f, x, 2)
	f.propagate(true)
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	if f.a.canGetModelValue(xv) {
		t.Fatal("proper column was available before model initialization")
	}
	value, err := f.a.cachedModelValue(v)
	astTestMust(t, err)
	if value.Cmp(big.NewRat(2, 1)) != 0 || !f.a.canGetModelValue(xv) {
		t.Fatal("source missing-column zero insertion changed", value)
	}
	astTestMust(t, f.a.initializeVariableValues())
	value, err = f.a.cachedModelValue(v)
	astTestMust(t, err)
	if value.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("GetModel unexpectedly erased the cached term")
	}
	f.assert(f.app(smtArithOpLE, x, f.number(3)), true)
	f.propagate(true)
	if f.a.canGetModelValue(xv) {
		t.Fatal("make_feasible failed to reset rational cache")
	}
	astTestMust(t, f.a.initializeVariableValues())
	value, err = f.a.cachedModelValue(v)
	astTestMust(t, err)
	if value.Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatal("fresh model term did not use new column values", value)
	}
}

func TestSMTLRAModelRejectsFractionalIntAndUninitializedGenerator(t *testing.T) {
	f, g := lraModelFixture(t)
	var err error
	f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
	astTestMust(t, err)
	x := f.variable("lra-model-fractional-int")
	v, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	index, err := f.a.numeric.getVarIndex(v)
	astTestMust(t, err)
	if _, err = f.a.ModelValue(f.s.context.enodes.get(x), g); err == nil {
		t.Fatal("model value accepted without init_model")
	}
	astTestMust(t, f.a.InitializeModel(g))
	f.a.modelValues[index] = big.NewRat(3, 2)
	if _, err = f.a.ModelValue(f.s.context.enodes.get(x), g); err == nil {
		t.Fatal("native fractional Int rejection was replaced with rounding")
	}
}

func TestSMTLRAModelAllocationRefusalOwnership(t *testing.T) {
	refusals, successes := 0, 0
	for _, allowance := range []int64{-1, 0, 1, 2, 5, 12, 32} {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f, g := lraModelFixture(t)
			astTestMust(t, f.a.numeric.Close())
			astTestMust(t, f.a.numerals.CloseChecked())
			budget := arena.NewBudget(8 << 20)
			m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
				Cells: arena.Config{PageSize: 4, Budget: budget}, Words: arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 4, Budget: budget}},
			}, Synchronized: true})
			astTestMust(t, err)
			f.a.numerals = m
			f.a.numeric, err = newSMTLRANumeric(m, smtLRANumericConfig{strategy: lp.StrategyTableauRows})
			astTestMust(t, err)
			var padding []numeral.Rational
			t.Cleanup(func() {
				budget.FailAfter(-1)
				astTestMust(t, f.a.numeric.Close())
				for i := range padding {
					astTestMust(t, m.Del(&padding[i]))
				}
				lraNumericManagerCensus(t, m)
				astTestMust(t, f.a.Close())
				if budget.Used() != 0 {
					t.Fatal("model retained private budget", budget.Used())
				}
			})
			x := f.variable("lra-model-refusal-x")
			v, err := f.a.internalizeDef(x)
			astTestMust(t, err)
			ten := new(big.Int).Exp(big.NewInt(10), big.NewInt(300), nil)
			value := new(big.Rat).SetFrac(new(big.Int).Add(ten, big.NewInt(13)), new(big.Int).Add(new(big.Int).Quo(ten, big.NewInt(10)), big.NewInt(7)))
			_, err = f.a.numeric.assertBound(v, 101, value, false, true)
			astTestMust(t, err)
			status, err := f.a.numeric.makeFeasible()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			for i := 0; i < 64; i++ {
				stats := m.Z.Cells.Stats()
				if stats.Slots-stats.Live-stats.Retired <= 1 {
					break
				}
				r := numeral.NewRational()
				astTestMust(t, m.SetInt64(&r, 1<<60))
				padding = append(padding, r)
			}
			stats := m.Z.Cells.Stats()
			if stats.Slots-stats.Live-stats.Retired > 1 {
				t.Fatal("no cold model allocation frontier", stats)
			}
			budget.FailAfter(allowance)
			err = f.a.InitializeModel(g)
			budget.FailAfter(-1)
			if err != nil {
				var failure umemory.Failure
				memout := errors.As(err, &failure) && failure.Kind == umemory.FailureMemout && failure.ErrorCode() == 101 && allowance >= 0 && budget.Used() < budget.Limit()
				if !errors.Is(err, arena.ErrInjectedFailure) && !memout {
					t.Fatal(err)
				}
				refusals++
				if f.a.numeric.model == nil || f.a.modelOwner != nil {
					t.Fatal("failed model lost its output owner or published a factory")
				}
				if _, next := f.a.numeric.makeFeasible(); next != lp.ErrExactVectorPending {
					t.Fatal("failed model owner was reused", next)
				}
				if next := f.a.InitializeModel(g); next != lp.ErrExactVectorPending {
					t.Fatal("failed model callback was reused", next)
				}
			} else {
				successes++
				index, _ := f.a.numeric.index(v)
				if f.a.modelValues[index].Cmp(value) != 0 {
					t.Fatal("model lost large exact rational")
				}
			}
		})
	}
	if refusals == 0 || successes == 0 {
		t.Fatal("model samples require refusal and success", refusals, successes)
	}
}
