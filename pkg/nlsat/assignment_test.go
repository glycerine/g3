package nlsat

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func assignmentMust(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func assignmentFixture(t testing.TB) (*polynomial.AlgebraicManager, *arena.Budget) {
	t.Helper()
	am, budget, _ := assignmentFixtureWithLimit(t)
	return am, budget
}
func assignmentFixtureWithLimit(t testing.TB) (*polynomial.AlgebraicManager, *arena.Budget, *control.Reslimit) {
	t.Helper()
	// Keep this numerical fixture independent of the command/SMT composition
	// root: the production SMT backend now consumes NLSAT.
	budget := arena.NewBudget(1 << 40)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	assignmentMust(t, err)
	q, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	assignmentMust(t, err)
	t.Cleanup(func() {
		assignmentMust(t, q.Close())
		assignmentMust(t, symbols.Close())
		if budget.Used() != 0 {
			t.Fatal("assignment fixture retained bytes", budget.Used())
		}
	})
	context, err := uparams.NewParamsContext(symbols, q, uparams.ParamsConfig{Budget: budget})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, context.Close()) })
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, catalog.Close()) })
	g, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, g.Close()) })
	assignmentMust(t, g.InitializeSource())
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: q, Budget: budget})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, m.Close()) })
	previous, err := uparams.BindProcessGlobalParamsService(g)
	assignmentMust(t, err)
	t.Cleanup(func() { _, err := uparams.BindProcessGlobalParamsService(previous); assignmentMust(t, err) })
	p, err := g.Context.NewValue()
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, g.Context.Del(&p)) })
	coop, err := cooperate.NewCooperation(m.Budget())
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, coop.Close()) })
	primes, err := primegen.NewGenerator(m.Budget())
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, primes.Close()) })
	am, err := polynomial.NewAlgebraicManager(polynomial.AlgebraicManagerConfig{Rationals: m.NumeralService(), Limit: &m.Limit, Cooperation: coop, Params: &p, Globals: g, Factorization: polynomial.UPolynomialFactorizationConfig{Primes: primes}})
	if am != nil {
		t.Cleanup(func() { assignmentMust(t, am.Close()) })
	}
	assignmentMust(t, err)
	return am, m.Budget(), &m.Limit
}
func assignmentNew(t *testing.T, m *polynomial.AlgebraicManager) *Assignment {
	t.Helper()
	a, err := NewAssignment(m)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, a.Close()) })
	return a
}
func assignmentInteger(t *testing.T, m *polynomial.AlgebraicManager, n int32) *polynomial.Algebraic {
	t.Helper()
	value := &polynomial.Algebraic{}
	num := unumeral.NewInt(n)
	assignmentMust(t, m.SetInteger(value, &num))
	t.Cleanup(func() { assignmentMust(t, m.Del(value)) })
	return value
}
func assignmentWantInteger(t *testing.T, m *polynomial.AlgebraicManager, value *polynomial.Algebraic, n int32) {
	t.Helper()
	q, err := m.RationalManager()
	assignmentMust(t, err)
	number := unumeral.NewRational()
	assignmentMust(t, q.SetInt32(&number, n))
	defer func() { assignmentMust(t, q.Del(&number)) }()
	cmp, err := m.CompareRational(value, &number)
	assignmentMust(t, err)
	if cmp != 0 {
		t.Fatal("assigned value differs", n, cmp)
	}
}

func TestAssignmentResetCopyAndExchangePreserveHiddenCells(t *testing.T) {
	m, _ := assignmentFixture(t)
	a, b := assignmentNew(t, m), assignmentNew(t, m)
	value := assignmentInteger(t, m, 7)
	assignmentMust(t, a.Set(2, value))
	assignmentMust(t, a.ResetVar(2))
	contains, err := a.Contains(2)
	assignmentMust(t, err)
	if contains {
		t.Fatal("reset variable still assigned")
	}
	cell, err := a.Value(2)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, 7)
	assignmentMust(t, a.Reset())
	cell, err = a.Value(2)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, 7)
	incoming := assignmentInteger(t, m, -3)
	assignmentMust(t, a.SetCore(2, incoming))
	assignmentWantInteger(t, m, incoming, 7)
	cell, err = a.Value(2)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, -3)
	assignmentMust(t, b.Set(0, assignmentInteger(t, m, 9)))
	assignmentMust(t, b.Set(3, assignmentInteger(t, m, 11)))
	assignmentMust(t, b.Copy(a))
	cell, err = b.Value(0)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, 9)
	contains, err = b.Contains(0)
	assignmentMust(t, err)
	if contains {
		t.Fatal("unassigned source flag not copied")
	}
	cell, err = b.Value(3)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, 11)
	assignmentMust(t, a.Copy(a))
	if len(a.assigned) != 0 || len(a.values) != 3 {
		t.Fatal("source self-copy retained wrong mask/slots")
	}
	cell, err = a.Value(2)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, -3)
}

func TestAssignmentIndependentStateMachine(t *testing.T) {
	m, _ := assignmentFixture(t)
	assignments := []*Assignment{assignmentNew(t, m), assignmentNew(t, m)}
	type state struct {
		values []int32
		flags  []bool
	}
	states := []state{{}, {}}
	rng := rand.New(rand.NewPCG(93173, 17121))
	for step := 0; step < 240; step++ {
		index := rng.IntN(2)
		a, s := assignments[index], &states[index]
		x := rng.IntN(7)
		value := int32(rng.IntN(31) - 15)
		switch rng.IntN(7) {
		case 0, 1:
			for len(s.values) <= x {
				s.values = append(s.values, 0)
			}
			for len(s.flags) <= x {
				s.flags = append(s.flags, false)
			}
			incoming := assignmentInteger(t, m, value)
			if step%2 == 0 {
				assignmentMust(t, a.Set(uint32(x), incoming))
			} else {
				assignmentMust(t, a.SetCore(uint32(x), incoming))
				assignmentWantInteger(t, m, incoming, s.values[x])
			}
			s.values[x], s.flags[x] = value, true
		case 2:
			assignmentMust(t, a.ResetVar(uint32(x)))
			if x < len(s.flags) {
				s.flags[x] = false
			}
		case 3:
			assignmentMust(t, a.Reset())
			s.flags = nil
		case 4:
			other := states[1-index]
			assignmentMust(t, a.Copy(assignments[1-index]))
			s.flags = append([]bool(nil), other.flags...)
			for len(s.values) < len(s.flags) {
				s.values = append(s.values, 0)
			}
			for i, assigned := range s.flags {
				if assigned {
					s.values[i] = other.values[i]
				}
			}
		case 5:
			assignmentMust(t, a.Swap(assignments[1-index]))
			states[index], states[1-index] = states[1-index], states[index]
		case 6:
			if len(s.flags) > 0 {
				x, y := rng.IntN(len(s.flags)), rng.IntN(len(s.flags))
				assignmentMust(t, a.SwapVars(uint32(x), uint32(y)))
				s.values[x], s.values[y] = s.values[y], s.values[x]
				s.flags[x], s.flags[y] = s.flags[y], s.flags[x]
			}
		}
		for index, a := range assignments {
			s := states[index]
			if len(a.values) != len(s.values) || len(a.assigned) != len(s.flags) {
				t.Fatal("state shape", step, index)
			}
			for i := 0; i < 9; i++ {
				actual, err := a.Contains(uint32(i))
				assignmentMust(t, err)
				want := i < len(s.flags) && s.flags[i]
				if actual != want {
					t.Fatal("membership", step, index, i, actual, want)
				}
				if i < len(s.values) {
					cell, err := a.Value(uint32(i))
					assignmentMust(t, err)
					assignmentWantInteger(t, m, cell, s.values[i])
				}
			}
		}
	}
}

func TestAssignmentMaskBorrowAndSourceAssertions(t *testing.T) {
	debugBudget := arena.NewBudget(1 << 20)
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: debugBudget}})
	assignmentMust(t, err)
	debug, err := diagnostic.NewDebugContext(region, debugBudget)
	assignmentMust(t, err)
	previous, err := diagnostic.BindProcessDebugContext(debug)
	assignmentMust(t, err)
	t.Cleanup(func() {
		_, err := diagnostic.BindProcessDebugContext(previous)
		assignmentMust(t, err)
		assignmentMust(t, debug.Close())
		assignmentMust(t, region.Close())
		if debugBudget.Used() != 0 {
			t.Fatal("debug fixture retained storage")
		}
	})
	m, _ := assignmentFixture(t)
	a := assignmentNew(t, m)
	assignmentMust(t, a.Set(1, assignmentInteger(t, m, 5)))
	u := UndefVarAssignment{Assignment: a, Variable: 1}
	contains, err := u.Contains(1)
	assignmentMust(t, err)
	if contains {
		t.Fatal("mask membership")
	}
	cell, err := u.Algebraic(1)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, 5)
	assignmentMust(t, a.ResetVar(1))
	cell, err = u.Algebraic(1)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, 5)
	cell, err = a.Algebraic(1)
	if diagnostic.DebugBuild {
		if !errors.Is(err, diagnostic.ErrDebugAssertion) {
			t.Fatal("missing source assignment assertion", err)
		}
	} else {
		assignmentMust(t, err)
		assignmentWantInteger(t, m, cell, 5)
	}
	copy := *a
	if _, err := copy.Contains(1); err == nil {
		t.Fatal("copied assignment accepted")
	}
	assignmentMust(t, a.Close())
	assignmentMust(t, a.Close())
	contains, err = u.Contains(1)
	assignmentMust(t, err)
	if contains {
		t.Fatal("masked membership reached retired assignment")
	}
	if _, err = u.Algebraic(1); err == nil {
		t.Fatal("raw lookup bypassed retired owner")
	}
}

func TestAssignmentGrowthRefusalRetainsReachedPrefixAndDisplay(t *testing.T) {
	m, budget := assignmentFixture(t)
	a := assignmentNew(t, m)
	assignmentMust(t, a.Set(0, assignmentInteger(t, m, -3)))
	value := assignmentInteger(t, m, 8)
	budget.FailAfter(1)
	err := a.Set(4, value)
	budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("missing flag-growth refusal", err)
	}
	if len(a.values) != 5 || len(a.assigned) != 1 {
		t.Fatal("allocation prefix rolled back or published flags")
	}
	cell, err := a.Value(4)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, cell, 0)
	assignmentMust(t, a.Set(4, value))
	var out bytes.Buffer
	assignmentMust(t, a.Display(&out))
	if out.String() != "x0 := -3\nx4 := 8\n" {
		t.Fatal("assignment display", out.String())
	}
}

func TestAssignmentIrrationalOwnersAndPrebuiltCPPOracle(t *testing.T) {
	m, budget := assignmentFixture(t)
	a, b := assignmentNew(t, m), assignmentNew(t, m)
	root := assignmentInteger(t, m, 2)
	assignmentMust(t, m.Root(root, 2, root))
	assignmentMust(t, a.Set(0, root))
	assignmentMust(t, b.Copy(a))
	assignmentMust(t, m.Del(root))
	assignmentMust(t, a.Reset())
	incoming := assignmentInteger(t, m, 5)
	assignmentMust(t, b.SetCore(0, incoming))
	bcell, err := b.Algebraic(0)
	assignmentMust(t, err)
	assignmentWantInteger(t, m, bcell, 5)
	acell, err := a.Value(0)
	assignmentMust(t, err)
	var script strings.Builder
	for _, value := range []*polynomial.Algebraic{acell, incoming} {
		var square polynomial.Algebraic
		assignmentMust(t, m.Power(value, 2, &square))
		assignmentWantInteger(t, m, &square, 2)
		assignmentMust(t, m.Del(&square))
		stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: budget})
		assignmentMust(t, err)
		assignmentMust(t, m.DisplayRootSMT2Into(stream, value))
		text, err := stream.String()
		assignmentMust(t, err)
		assignmentMust(t, stream.Close())
		fmt.Fprintf(&script, "(push)(assert (not (= (* %s %s) 2.0)))(check-sat)(pop)\n", text, text)
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		return
	} // owning algebraic checks above always run
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = strings.NewReader(script.String())
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(raw))
	}
	if strings.Join(strings.Fields(string(raw)), " ") != "unsat unsat" {
		t.Fatal("retained/exchanged irrational values", string(raw))
	}
}
