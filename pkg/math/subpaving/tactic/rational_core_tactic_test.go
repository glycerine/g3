package tactic

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestRationalCoreTacticBorrowedGoalStatisticsAndFreshCleanup(t *testing.T) {
	b := arena.NewBudget(1 << 27)
	integers := numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: integers})
	if err != nil {
		t.Fatal(err)
	}
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: integers})
	if err != nil {
		t.Fatal(err)
	}
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	p, err := pc.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	depth := "max_depth"
	if err := pc.SetUIntName(&p, &depth, 0); err != nil {
		t.Fatal(err)
	}
	core, err := NewRationalCoreTactic(m, qm, pc, &p)
	if err != nil {
		t.Fatal(err)
	}
	goal, err := goaltactic.NewGoal(m, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	value, err := q.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, e := range []error{goal.Close(), core.Close(), pc.Del(&p), pc.Close(), q.Del(&value), m.Close(), q.Close(), sy.Close(), qm.CloseChecked()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("rational core owner roots", b.Used())
		}
	})
	name, err := sy.Intern("x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetString(&value, "3/2"); err != nil {
		t.Fatal(err)
	}
	threshold, err := m.ArithmeticNumeral(&value, false)
	if err != nil {
		t.Fatal(err)
	}
	le, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{x, threshold})
	if err != nil {
		t.Fatal(err)
	}
	if err := goal.Assert(le); err != nil {
		t.Fatal(err)
	}
	result, err := core.Apply(goal)
	if err != nil || result != goal {
		t.Fatal("source borrowed result", result, err)
	}
	st := statistics.New()
	if err := core.CollectStatistics(st); err != nil || st.Size() != 2 || st.Key(0) != "new bounds" || st.UintValue(0) != 1 || st.Key(1) != "nodes" || st.UintValue(1) != 1 {
		t.Fatal("first search statistics", err)
	}
	if err := core.Cleanup(); err != nil {
		t.Fatal("source fresh implementation", err)
	}
	result, err = core.Apply(goal)
	if err != nil || result != goal {
		t.Fatal("reused goal with fresh implementation", result, err)
	}
	st.Reset()
	if err := core.CollectStatistics(st); err != nil || st.Size() != 4 || st.Key(2) != "new bounds" || st.UintValue(2) != 1 || st.Key(3) != "nodes" || st.UintValue(3) != 1 {
		t.Fatal("accumulated statistics", err)
	}
	if err := core.ResetStatistics(); err != nil {
		t.Fatal(err)
	}
	st.Reset()
	if err := core.CollectStatistics(st); err != nil || st.Size() != 0 {
		t.Fatal("statistics reset", err)
	}
	unsupportedGoal, err := goaltactic.NewGoal(m, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unsupportedGoal.Close(); err != nil {
			t.Error(err)
		}
	})
	boolSort, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	unsupportedName, err := sy.Intern("unsupported")
	if err != nil {
		t.Fatal(err)
	}
	unsupportedAtom, err := m.MkConst(unsupportedName, boolSort)
	if err != nil {
		t.Fatal(err)
	}
	if err := unsupportedGoal.Assert(unsupportedAtom); err != nil {
		t.Fatal(err)
	}
	if result, err := core.Apply(unsupportedGoal); result != nil {
		t.Fatal("unsupported source atom must fail before returning the goal", result, err)
	} else {
		var tacticError goaltactic.TacticException
		if !errors.As(err, &tacticError) || tacticError.Message() != "unsupported atom" {
			t.Fatal("source tactic exception transport", err)
		}
	}
	printKey := "print_nodes"
	if err := pc.SetBoolName(&p, &printKey, true); err != nil {
		t.Fatal(err)
	}
	if err := core.UpdateParams(&p); err != nil {
		t.Fatal(err)
	}
	if result, err := core.ApplyWithOutput(goal, nil); result != nil || !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("print route requires writer", result, err)
	}
	if err := core.Cleanup(); err != nil {
		t.Fatal(err)
	}
	var shown bytes.Buffer
	if result, err := core.ApplyWithOutput(goal, &shown); err != nil || result != goal {
		t.Fatal("source print_nodes result", result, err)
	}
	complete := "x <= 3/2\nbounds at leaves: \nx <= 3/2\n"
	if shown.String() != complete {
		t.Fatal("source print_nodes text", shown.String())
	}
	if err := core.Cleanup(); err != nil {
		t.Fatal(err)
	}
	short := &shortFastTacticOutput{limit: 6, withError: true}
	if result, err := core.ApplyWithOutput(goal, short); err != nil || result != goal || short.String() != complete[:6] || short.calls == 0 {
		t.Fatal("nonthrowing output prefix", result, err, short.String())
	}
	m.ResourceLimit().Cancel()
	result, err = core.ApplyWithOutput(goal, &shown)
	var tacticErr goaltactic.TacticException
	var sourceErr z3exception.DefaultException
	if result != nil || !errors.As(err, &tacticErr) || tacticErr.Message() != messages.Canceled || errors.As(err, &sourceErr) {
		t.Fatal("converter cancellation must cross the source tactic boundary", result, err)
	}
	m.ResourceLimit().ResetCancel()
	if err := core.Cleanup(); err != nil {
		t.Fatal(err)
	}
	badGoal, err := goaltactic.NewGoal(m, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := badGoal.Close(); err != nil {
			t.Error(err)
		}
	})
	sub, err := m.MkFamilyApp(family, 7, nil, []ast.ASTHandle{x, x})
	if err != nil {
		t.Fatal(err)
	}
	unsimplified, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{sub, threshold})
	if err != nil {
		t.Fatal(err)
	}
	if err := badGoal.Assert(unsimplified); err != nil {
		t.Fatal(err)
	}
	result, err = core.ApplyWithOutput(badGoal, &shown)
	if result != nil || !errors.As(err, &tacticErr) || tacticErr.Message() != ErrExprConverterNonSimplified.Error() {
		t.Fatal("unsimplified source exception must cross tactic boundary", result, err)
	}
	if err := core.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Apply(goal); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed tactic", err)
	}
}
