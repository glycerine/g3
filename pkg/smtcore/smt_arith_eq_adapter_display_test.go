package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func arithmeticAdapterDisplayFixture(t *testing.T) (*arithmeticObjectiveFixture, ast.ASTHandle, ast.ASTHandle, string) {
	t.Helper()
	f := newArithmeticObjectiveFixture(t, false)
	x, y := f.variable("adapter-display-x"), f.variable("adapter-display-y")
	f.internalizeVar(x)
	f.internalizeVar(y)
	left, right := f.s.context.enodes.get(x), f.s.context.enodes.get(y)
	astTestMust(t, f.a.NewEquality(right.TheoryVar(f.a.FamilyID()), left.TheoryVar(f.a.FamilyID())))
	first, err := left.OwnerID()
	astTestMust(t, err)
	second, err := right.OwnerID()
	astTestMust(t, err)
	if first > second {
		first, second = second, first
	}
	return f, x, y, fmt.Sprintf("eq_adapter: #%d #%d\n", first, second)
}

func TestSMTArithmeticAdapterDisplayProcessedPairsAndScopes(t *testing.T) {
	f, x, y, want := arithmeticAdapterDisplayFixture(t)
	a := f.a.arithEqAdapter
	var out bytes.Buffer
	before := f.m.Stats()
	astTestMust(t, a.DisplayAlreadyProcessed(&out))
	if out.String() != want || f.m.Stats() != before {
		t.Fatal("literal record or borrowed owner changed", out.String(), want)
	}
	pair, err := a.pair(f.s.context.enodes.get(y), f.s.context.enodes.get(x))
	astTestMust(t, err)
	first, err := f.m.GetASTID(pair.first)
	astTestMust(t, err)
	second, err := f.m.GetASTID(pair.second)
	astTestMust(t, err)
	if first >= second {
		t.Fatal("pair canonicalization does not use owner IDs", first, second)
	}
	astTestMust(t, f.a.NewDisequality(f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID()), f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())))
	if len(a.processed) != 1 {
		t.Fatal("reverse duplicate cache entry")
	}
	z := f.variable("adapter-display-z")
	f.internalizeVar(z)
	astTestMust(t, f.s.PushScope())
	astTestMust(t, f.a.NewEquality(f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID()), f.s.context.enodes.get(z).TheoryVar(f.a.FamilyID())))
	out.Reset()
	astTestMust(t, a.DisplayAlreadyProcessed(&out))
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(out.String(), want) {
		t.Fatal("scoped pair missing", out.String())
	}
	astTestMust(t, f.s.PopScopes(1))
	out.Reset()
	astTestMust(t, a.DisplayAlreadyProcessed(&out))
	if out.String() != want {
		t.Fatal("popped pair still displayed", out.String())
	}
}

func TestSMTArithmeticAdapterDisplayEveryWriterPrefixAndGuards(t *testing.T) {
	f, _, _, want := arithmeticAdapterDisplayFixture(t)
	a := f.a.arithEqAdapter
	for cut := 0; cut < len(want); cut++ {
		out := &smtLRAOutputPrefix{remaining: cut}
		err := a.DisplayAlreadyProcessed(out)
		if !errors.Is(err, io.ErrClosedPipe) || out.text.String() != want[:cut] {
			t.Fatal(cut, err, out.text.String(), want[:cut])
		}
	}
	if err := a.DisplayAlreadyProcessed(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if err := a.DisplayAlreadyProcessed(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	var absent *smtArithmeticEqualityAdapter
	if err := absent.DisplayAlreadyProcessed(io.Discard); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	unbound := &smtArithmeticEqualityAdapter{owner: &SMTTheoryBase{}}
	if err := unbound.DisplayAlreadyProcessed(io.Discard); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("unbound context", err)
	}
}

type arithmeticAdapterTraceWriter struct {
	output                bytes.Buffer
	owner                 *SMTTheoryArith
	seen, pending, refuse bool
}

func (w *arithmeticAdapterTraceWriter) Write(p []byte) (int, error) {
	if !w.seen && w.owner != nil {
		w.seen = true
		w.pending = w.owner.CanPropagate()
	}
	if w.refuse {
		return 0, io.ErrClosedPipe
	}
	return w.output.Write(p)
}

func TestSMTArithmeticAdapterFinalCheckTracePrecedesPropagation(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f, x, _, body := arithmeticAdapterDisplayFixture(t)
		f.assert(f.app(smtArithOpGE, x, f.number("2")))
		if !f.a.CanPropagate() {
			t.Fatal("fixture has no pending arithmetic assignment")
		}
		writer := &arithmeticAdapterTraceWriter{owner: f.a, refuse: refuse}
		tags := diagnostic.NewTraceTagsWithWriter(writer)
		tags.Enable("arith_eq_adapter_info")
		f.a.arithEqAdapter.trace = tags
		status, err := f.a.FinalCheck()
		astTestMust(t, err)
		// final_check_eh propagates, then runs final_check_core: x = 2 with
		// y unconstrained is a model.
		if status != SMTFinalCheckDone || f.a.CanPropagate() {
			t.Fatal("trace changed actual propagation", status)
		}
		want := ""
		if tags.ShouldTrace("arith_eq_adapter_info") {
			if !writer.seen || !writer.pending {
				t.Fatal("trace moved after propagation")
			}
			if !refuse {
				want = "-------- [arith_eq_adapter_info] final_check_eh src/smt/theory_arith_core.h:1475 ---------\n" + body + "------------------------------------------------\n"
			}
		} else if writer.seen {
			t.Fatal("native trace was not erased")
		}
		if writer.output.String() != want {
			t.Fatal(writer.output.String(), want)
		}
	}
}

func TestSMTArithmeticAdapterTraceBadbitContinuesOperandsAndNativeErases(t *testing.T) {
	f, x, y, _ := arithmeticAdapterDisplayFixture(t)
	f.assert(f.app(smtArithOpGE, x, f.number("2")))
	dead, err := f.m.MkConst(astTestSymbol(t, f.m, "adapter-stale-operand"), f.sort)
	astTestMust(t, err)
	ref, err := f.m.NewRef(dead)
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&ref))
	if _, err = f.m.GetASTID(dead); err == nil {
		t.Fatal("stale test operand is live")
	}
	invalid := smtArithmeticEqualityPair{first: dead, second: y}
	f.a.arithEqAdapter.processed[invalid] = struct{}{}
	defer delete(f.a.arithEqAdapter.processed, invalid)
	writer := &arithmeticAdapterTraceWriter{owner: f.a, refuse: true}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("arith_eq_adapter_info")
	f.a.arithEqAdapter.trace = tags
	status, err := f.a.FinalCheck()
	if tags.ShouldTrace("arith_eq_adapter_info") {
		if err == nil || tags.IsStreamFailure(err) || status != SMTFinalCheckGiveUp || !f.a.CanPropagate() {
			t.Fatal("badbit suppressed AST operand failure", status, err)
		}
	} else {
		astTestMust(t, err)
		if status != SMTFinalCheckDone || writer.seen || f.a.CanPropagate() {
			t.Fatal("native trace evaluated stale operand", status)
		}
	}
}
