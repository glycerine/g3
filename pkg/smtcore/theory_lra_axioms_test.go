package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTLRABoundAxiomsKindsAndIntegerAdjacency(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, deferred := range []bool{false, true} {
			for _, test := range []struct {
				first, second int32
				k1, k2        int64
				clauses       [][2]bool // true denotes a negated literal
			}{
				{smtArithOpGE, smtArithOpGE, 1, 2, [][2]bool{{false, true}}},
				{smtArithOpGE, smtArithOpGE, 2, 1, [][2]bool{{true, false}}},
				{smtArithOpLE, smtArithOpLE, 1, 2, [][2]bool{{true, false}}},
				{smtArithOpLE, smtArithOpLE, 2, 1, [][2]bool{{false, true}}},
				{smtArithOpGE, smtArithOpLE, 1, 2, [][2]bool{{false, false}}},
				{smtArithOpLE, smtArithOpGE, 2, 1, [][2]bool{{false, false}}},
				{smtArithOpGE, smtArithOpLE, 2, 1, [][2]bool{{true, true}}},
				{smtArithOpLE, smtArithOpGE, 1, 2, [][2]bool{{true, true}}},
				{smtArithOpGE, smtArithOpLE, 3, 1, [][2]bool{{true, true}}},
				{smtArithOpGE, smtArithOpLE, 1, 1, [][2]bool{{false, false}}},
			} {
				t.Run(fmt.Sprint(integer, deferred, test), func(t *testing.T) {
					f := newLRAFrontendFixture(t, true)
					if integer {
						var err error
						f.sort, err = f.m.MkFamilySort(f.a.FamilyID(), 1, nil)
						astTestMust(t, err)
					}
					number := func(n int64) ast.ASTHandle { return f.pin(f.m.ArithmeticNumeralRat(big.NewRat(n, 1), integer)) }
					x := f.variable("lra-bound-kinds-x")
					h1, h2 := f.app(test.first, x, number(test.k1)), f.app(test.second, x, number(test.k2))
					f.s.searching = !deferred
					astTestMust(t, f.s.Internalize(h1, true))
					astTestMust(t, f.s.Internalize(h2, true))
					if deferred {
						if len(f.s.Clauses()) != 0 {
							t.Fatal("deferred axiom published early")
						}
						astTestMust(t, f.a.Propagate())
					}
					v1, ok1 := f.s.BoolVar(h1)
					v2, ok2 := f.s.BoolVar(h2)
					if !ok1 || !ok2 {
						t.Fatal("missing Boolean bounds")
					}
					var want, got []string
					for _, signs := range test.clauses {
						want = append(want, boundClauseSet([]SMTLiteral{NewSMTLiteral(v1, signs[0]), NewSMTLiteral(v2, signs[1])}))
					}
					if integer && test.first != test.second && test.clauses[0] == [2]bool{true, true} && (test.k1-test.k2 == 1 || test.k2-test.k1 == 1) {
						want = append(want, boundClauseSet([]SMTLiteral{NewSMTLiteral(v1, false), NewSMTLiteral(v2, false)}))
					}
					for _, clause := range f.s.Clauses() {
						got = append(got, boundClauseSet(clause.Literals))
					}
					sort.Strings(want)
					sort.Strings(got)
					if !reflect.DeepEqual(got, want) {
						t.Fatal("bound clauses", got, "want", want)
					}
				})
			}
		}
	}
}

func TestSMTLRAEqualityTrianglesCacheScopeAndNumericalConflict(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-equality-x"), f.variable("lra-equality-y")
	xv, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	yv, err := f.a.internalizeDef(y)
	astTestMust(t, err)
	f.assert(f.app(smtArithOpGE, x, f.number(2)), true)
	f.assert(f.app(smtArithOpLE, y, f.number(1)), true)
	f.propagate(true)
	astTestMust(t, f.s.PushScope())
	eq := f.pin(f.s.mkEqualityAtom(x, y))
	f.assert(eq, true)
	if f.a.equalityAdapter.NumAxioms() != 1 || len(f.a.equalityAdapter.processed) != 1 {
		t.Fatal("recursive equality was not cached")
	}
	astTestMust(t, f.a.NewDisequality(yv, xv))
	if f.a.equalityAdapter.NumAxioms() != 1 {
		t.Fatal("reverse pair bypassed cache")
	}
	f.propagate(false)
	if !f.a.notHandled.IsNull() {
		t.Fatal("normalized equality refused", f.a.notHandled)
	}
	astTestMust(t, f.s.PopScopes(1))
	if len(f.a.equalityAdapter.processed) != 0 {
		t.Fatal("scoped cache survived pop")
	}
	f.propagate(true)
	astTestMust(t, f.s.PushScope())
	astTestMust(t, f.a.NewEquality(xv, yv))
	if f.a.equalityAdapter.NumAxioms() != 2 {
		t.Fatal("equality was not recreated")
	}
	// The three triangle clauses have the source polarities, independently of
	// bound-axiom clauses added on other axes.
	eqLit, err := f.s.Literal(eq)
	astTestMust(t, err)
	var le, ge SMTLiteral
	for boolean, bound := range f.a.boundByBoolean {
		if bound.variable == xv || bound.variable == yv {
			continue
		}
		if bound.upper {
			le = NewSMTLiteral(boolean, false)
		} else {
			ge = NewSMTLiteral(boolean, false)
		}
	}
	want := map[string]bool{
		boundClauseSet([]SMTLiteral{eqLit.Negated(), le}):               false,
		boundClauseSet([]SMTLiteral{eqLit.Negated(), ge}):               false,
		boundClauseSet([]SMTLiteral{eqLit, le.Negated(), ge.Negated()}): false,
	}
	for _, clause := range f.s.Clauses() {
		key := boundClauseSet(clause.Literals)
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for clause, found := range want {
		if !found {
			t.Fatal("missing triangle", clause)
		}
	}
}

func TestSMTLRAEqualityRestartAndRelevancy(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-restart-x"), f.variable("lra-restart-y")
	xv, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	yv, err := f.a.internalizeDef(y)
	astTestMust(t, err)
	astTestMust(t, f.s.InitializeTheorySearch())
	astTestMust(t, f.s.beginSearchPersistence(0, SMTParamsState{AgilityFactor: 0.9999, RestartAdaptive: true}))
	defer func() { astTestMust(t, f.s.endSearchPersistence()) }()
	astTestMust(t, f.s.PushScope())
	astTestMust(t, f.a.NewEquality(xv, yv))
	if len(f.a.equalityAdapter.restart) != 1 {
		t.Fatal("missing eager restart pair")
	}
	astTestMust(t, f.s.PopScopes(1))
	astTestMust(t, f.s.RestartTheories())
	if len(f.a.equalityAdapter.restart) != 0 || f.a.equalityAdapter.NumAxioms() != 2 {
		t.Fatal("restart did not recreate base axioms")
	}
	targets := append([]ast.ASTHandle(nil), f.s.boolExpr[SMTFirstBoolVar:]...)
	if len(targets) != 3 {
		t.Fatal("expected equality and two bounds", len(targets))
	}
	astTestMust(t, f.s.relevancy.MarkRelevant(x))
	astTestMust(t, f.s.relevancy.Propagate())
	for _, target := range targets {
		if f.s.relevancy.IsRelevant(target) {
			t.Fatal("single term activated triangle")
		}
	}
	astTestMust(t, f.s.relevancy.MarkRelevant(y))
	astTestMust(t, f.s.relevancy.Propagate())
	for _, target := range targets {
		if !f.s.relevancy.IsRelevant(target) {
			t.Fatal("both terms failed to activate triangle")
		}
		lit, err := f.s.Literal(target)
		astTestMust(t, err)
		value, err := f.s.Assignment(lit)
		astTestMust(t, err)
		if value != lbool.Undef {
			t.Fatal("relevancy assigned a triangle literal")
		}
	}
}

func TestSMTLRABoundNearestNeighborsAndUnitAxis(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		t.Run(fmt.Sprint(deferred), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			x, y := f.variable("lra-neighbor-x"), f.variable("lra-neighbor-y")
			var formulas []ast.ASTHandle
			for _, n := range []int64{0, 3, 7, 10} {
				formulas = append(formulas, f.app(smtArithOpGE, x, f.number(n)))
			}
			for _, n := range []int64{-1, 4, 8, 11} {
				formulas = append(formulas, f.app(smtArithOpLE, x, f.number(n)))
			}
			for _, h := range formulas {
				astTestMust(t, f.s.Internalize(h, true))
			}
			astTestMust(t, f.a.Propagate())
			before := len(f.s.Clauses())
			f.s.searching = !deferred
			// Unit terms share the actual LRA variable, even with different ASTs.
			h := f.app(smtArithOpGE, f.app(smtArithOpAdd, x, f.number(0)), f.number(5))
			astTestMust(t, f.s.Internalize(h, true))
			astTestMust(t, f.s.Internalize(f.app(smtArithOpGE, y, f.number(5)), true))
			astTestMust(t, f.a.Propagate())
			v, _ := f.s.BoolVar(h)
			var want, got []string
			for _, i := range []int{1, 2, 5, 6} {
				other, _ := f.s.BoolVar(formulas[i])
				l1, l2 := NewSMTLiteral(v, i == 1 || i == 5), NewSMTLiteral(other, i == 2 || i == 5)
				want = append(want, boundClauseSet([]SMTLiteral{l1, l2}))
			}
			for _, c := range f.s.Clauses()[before:] {
				got = append(got, boundClauseSet(c.Literals))
			}
			sort.Strings(want)
			sort.Strings(got)
			if !reflect.DeepEqual(got, want) {
				t.Fatal("nearest neighbors or unit axis", got, want)
			}
		})
	}
}

func TestSMTLRAAxiomProofOwnershipAndPhase(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	astTestMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	x, y := f.variable("lra-proof-x"), f.variable("lra-proof-y")
	xv, err := f.a.internalizeDef(x)
	astTestMust(t, err)
	yv, err := f.a.internalizeDef(y)
	astTestMust(t, err)
	first, second := f.app(smtArithOpGE, x, f.number(1)), f.app(smtArithOpGE, x, f.number(2))
	eq := f.pin(f.s.mkEqualityAtom(x, y))
	// The source copies a pre-existing equality's phase preference to its bounds.
	// Suppress the callback just while allocating that Boolean variable.
	f.a.equalityAdapter.processed[smtArithmeticEqualityPair{x, y}] = struct{}{}
	astTestMust(t, f.s.Internalize(eq, true))
	delete(f.a.equalityAdapter.processed, smtArithmeticEqualityPair{x, y})
	eqLit, err := f.s.Literal(eq)
	astTestMust(t, err)
	_, err = f.s.setScopedTrueFirst(eqLit.Var())
	astTestMust(t, err)
	// Warm the plugin-owned small-numeral cache before measuring proof owners.
	f.number(0)
	f.number(-1)
	baseline := f.m.Stats().LiveParameters
	for repeat := 0; repeat < 2; repeat++ {
		astTestMust(t, f.s.PushScope())
		astTestMust(t, f.a.NewEquality(xv, yv))
		if len(f.a.equalityAdapter.hint) != 1 {
			t.Fatal("triangle proof hint missing")
		}
		triangles := 0
		for _, entry := range f.s.clauses {
			j, ok := entry.clause.Justification().(*SMTTheoryAxiomJustification)
			if !ok {
				continue
			}
			parameters := j.Parameters()
			if len(parameters) != 1 {
				continue
			}
			symbol, err := f.m.ParameterSymbol(parameters[0])
			astTestMust(t, err)
			name, _, err := f.m.Symbols.BareString(symbol)
			astTestMust(t, err)
			if name != "triangle-eq" {
				t.Fatal("wrong triangle proof", name)
			}
			triangles++
		}
		if triangles != 3 {
			t.Fatal("triangle proof clause count", triangles)
		}
		for bv := range f.a.boundByBoolean {
			data, err := f.s.BooleanData(bv)
			astTestMust(t, err)
			if !data.TryTrueFirst() {
				t.Fatal("equality phase not copied to generated bound")
			}
		}
		astTestMust(t, f.s.Internalize(first, true))
		astTestMust(t, f.s.Internalize(second, true))
		astTestMust(t, f.a.Propagate())
		farkas := 0
		for _, entry := range f.s.clauses {
			j, ok := entry.clause.Justification().(*SMTTheoryAxiomJustification)
			if !ok || len(j.Parameters()) != 3 {
				continue
			}
			parameters := j.Parameters()
			symbol, err := f.m.ParameterSymbol(parameters[0])
			astTestMust(t, err)
			name, _, err := f.m.Symbols.BareString(symbol)
			astTestMust(t, err)
			if name != "farkas" {
				t.Fatal(name)
			}
			for _, p := range parameters[1:] {
				one, err := f.m.NewParameterRat(big.NewRat(1, 1))
				astTestMust(t, err)
				equal, err := f.m.ParameterEqual(p, one)
				astTestMust(t, err)
				astTestMust(t, f.m.DelParameter(&one))
				if !equal {
					t.Fatal("nonunit bound coefficient")
				}
			}
			farkas++
		}
		if farkas == 0 {
			t.Fatal("missing Farkas bound proof")
		}
		astTestMust(t, f.s.PopScopes(1))
		if got := f.m.Stats().LiveParameters; got != baseline+1 {
			t.Fatal("only cached triangle hint should survive pop", got, baseline)
		}
	}
	astTestMust(t, f.a.Close())
	astTestMust(t, f.a.Close())
	if f.m.Stats().LiveParameters != baseline {
		t.Fatal("triangle hint survived close")
	}
}

func TestSMTLRABoundAxiomAllocationRefusalAndRetry(t *testing.T) {
	refused, succeeded := false, false
	for _, stop := range []int64{0, 1, 2, 4, 8, 16} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			astTestMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
			// Checked source validators create a real default context, lazily
			// initializing declaration-plugin parameters owned by the AST manager.
			// Warm those persistent owners before the exact proof-parameter census.
			astTestMust(t, f.a.checkValidation(smtLRAValidateAssignment, []SMTLiteral{SMTTrueLiteral}, SMTTrueLiteral, nil, nil))
			x := f.variable("lra-bound-refusal-x")
			first, second := f.app(smtArithOpGE, x, f.number(1)), f.app(smtArithOpGE, x, f.number(2))
			var padding []ast.ASTParameter
			t.Cleanup(func() { astTestMust(t, smtArithmeticReleaseParameters(f.m, padding)) })
			f.m.Budget().FailAfter(0)
			cold := false
			for i := 0; i < 8192; i++ {
				p, err := f.m.NewParameterInt(int32(i))
				if err != nil {
					cold = errors.Is(err, arena.ErrInjectedFailure)
					break
				}
				padding = append(padding, p)
			}
			f.m.Budget().FailAfter(-1)
			if !cold {
				t.Fatal("no physical parameter allocation frontier")
			}
			baseline := f.m.Stats().LiveParameters
			astTestMust(t, f.s.PushScope())
			astTestMust(t, f.s.Internalize(first, true))
			astTestMust(t, f.s.Internalize(second, true))
			f.m.Budget().FailAfter(stop)
			err := f.a.Propagate()
			f.m.Budget().FailAfter(-1)
			if err != nil {
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
				refused = true
				if len(f.a.newBounds) == 0 {
					t.Fatal("refused batch lost retry ownership")
				}
				astTestMust(t, f.a.Propagate())
			} else {
				succeeded = true
			}
			v1, _ := f.s.BoolVar(first)
			v2, _ := f.s.BoolVar(second)
			astTestMust(t, f.s.AssignLiteral(NewSMTLiteral(v2, false), NewSMTBooleanAxiom()))
			f.propagate(true)
			value, err := f.s.Assignment(NewSMTLiteral(v1, false))
			astTestMust(t, err)
			if value != lbool.True {
				t.Fatal("retry lost bound implication", value)
			}
			astTestMust(t, f.s.PopScopes(1))
			if got := f.m.Stats().LiveParameters; got != baseline {
				t.Fatal("refused proof parameters leaked", got, baseline)
			}
		})
	}
	if !refused || !succeeded {
		t.Fatal("allocation samples did not cover both refusal and success", refused, succeeded)
	}
}

// AUDIT.md smtcore_11-F1: internalize_eq_eh tests get_th_var, which also
// returns a variable inherited as a class root, and passes the equality's
// own enodes to arith_eq_adapter::mk_axioms.
func TestSMTLRAInternalizeEqualityInheritedRootVariable(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	family := f.a.FamilyID()
	y, x, c := f.variable("lra-inherit-y"), f.variable("lra-inherit-x"), f.variable("lra-inherit-c")
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "lra-inherit-g"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	gy := f.pin(f.m.MkApp(decl, []ast.ASTHandle{y}))
	f.assert(f.app(smtArithOpLE, gy, f.number(3)), true)
	f.assert(f.app(smtArithOpGE, c, f.number(1)), true)
	astTestMust(t, f.s.Internalize(x, false))
	if f.s.context.enodes.get(x).TheoryVar(family) != SMTNullTheoryVar {
		t.Fatal("x must not own an LRA variable")
	}
	f.assert(f.pin(f.m.MkEq(gy, x)), true)
	f.propagate(true)
	xNode := f.s.context.enodes.get(x)
	if xNode.TheoryVar(family) == SMTNullTheoryVar || f.a.IsAttachedToVariable(xNode) {
		t.Fatal("x did not inherit the root variable of (g y)")
	}
	before := f.a.equalityAdapter.NumAxioms()
	astTestMust(t, f.s.Internalize(f.pin(f.m.MkEq(x, c)), false))
	if f.a.equalityAdapter.NumAxioms() != before+1 {
		t.Fatal("inherited variable skipped the equality adapter", before, f.a.equalityAdapter.NumAxioms())
	}
	first, second := x, c
	xid, err := f.m.GetASTID(x)
	astTestMust(t, err)
	cid, err := f.m.GetASTID(c)
	astTestMust(t, err)
	if xid > cid {
		first, second = c, x
	}
	if _, found := f.a.equalityAdapter.processed[smtArithmeticEqualityPair{first, second}]; !found {
		t.Fatal("mk_axioms did not receive the equality's own enodes", f.a.equalityAdapter.processed)
	}
}
