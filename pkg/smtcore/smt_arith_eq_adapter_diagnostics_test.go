package smtcore

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func enableArithmeticAdapterTrace(tags *diagnostic.TraceTags) {
	for _, tag := range []string{
		"arith_eq_adapter_mk_axioms", "arith_eq_adapter_relevancy",
		"arith_eq_adapter_profile", "arith_eq_adapter_profile_detail",
		"arith_eq_adapter_bug", "arith_eq_adapter_perf", "interface_eq",
		"arith_eq_adapter", "arith_eq_adapter_detail",
	} {
		tags.Enable(tag)
	}
}

func checkArithmeticAdapterTrace(t *testing.T, text string) {
	t.Helper()
	for _, want := range []string{
		"[arith_eq_adapter] new_eq_eh src/smt/arith_eq_adapter.cpp:230",
		"[arith_eq_adapter_mk_axioms] mk_axioms src/smt/arith_eq_adapter.cpp:86",
		"[arith_eq_adapter_relevancy] mk_axioms src/smt/arith_eq_adapter.cpp:102",
		"[arith_eq_adapter_profile] mk_axioms src/smt/arith_eq_adapter.cpp:116",
		"[arith_eq_adapter_profile_detail] mk_axioms src/smt/arith_eq_adapter.cpp:121",
		"[arith_eq_adapter_bug] mk_axioms src/smt/arith_eq_adapter.cpp:130",
		"[arith_eq_adapter_perf] mk_axioms src/smt/arith_eq_adapter.cpp:174",
		"[interface_eq] mk_axioms src/smt/arith_eq_adapter.cpp:182",
		"[arith_eq_adapter_bug] mk_axioms src/smt/arith_eq_adapter.cpp:186",
		"[arith_eq_adapter] mk_axioms src/smt/arith_eq_adapter.cpp:208",
		"[arith_eq_adapter_detail] mk_axioms src/smt/arith_eq_adapter.cpp:226",
		"Logical context:\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in trace:\n%s", want, text)
		}
	}
}

func TestSMTArithmeticEqualityAdapterRetainedDiagnostics(t *testing.T) {
	t.Run("generic", func(t *testing.T) {
		f := newArithmeticObjectiveFixture(t, false)
		x, y := f.variable("adapter-diagnostic-x"), f.variable("adapter-diagnostic-y")
		f.internalizeVar(x)
		f.internalizeVar(y)
		var out bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&out)
		enableArithmeticAdapterTrace(tags)
		f.a.arithEqAdapter.trace = tags
		xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
		yv := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
		astTestMust(t, f.a.NewEquality(xv, yv))
		if tags.ShouldTrace("arith_eq_adapter") {
			checkArithmeticAdapterTrace(t, out.String())
		} else if out.Len() != 0 {
			t.Fatal("disabled source trace wrote output", out.String())
		}
	})
	t.Run("lra", func(t *testing.T) {
		f := newLRAFrontendFixture(t, true)
		x, y := f.variable("lra-adapter-diagnostic-x"), f.variable("lra-adapter-diagnostic-y")
		xv, err := f.a.internalizeDef(x)
		astTestMust(t, err)
		yv, err := f.a.internalizeDef(y)
		astTestMust(t, err)
		var out bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&out)
		enableArithmeticAdapterTrace(tags)
		f.a.trace = tags
		astTestMust(t, f.a.NewEquality(xv, yv))
		if tags.ShouldTrace("arith_eq_adapter") {
			checkArithmeticAdapterTrace(t, out.String())
		} else if out.Len() != 0 {
			t.Fatal("disabled source trace wrote output", out.String())
		}
	})
}

func TestSMTArithmeticEqualityAdapterTraceBadbitAndAssertionGate(t *testing.T) {
	f := newArithmeticObjectiveFixture(t, false)
	x, y := f.variable("adapter-badbit-x"), f.variable("adapter-badbit-y")
	f.internalizeVar(x)
	f.internalizeVar(y)
	tags := diagnostic.NewTraceTagsWithWriter(&arithmeticAdapterTraceWriter{refuse: true})
	enableArithmeticAdapterTrace(tags)
	f.a.arithEqAdapter.trace = tags
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	yv := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
	astTestMust(t, f.a.NewDisequality(xv, yv))
	if f.a.arithEqAdapter.NumAxioms() != 1 {
		t.Fatal("trace badbit stopped adapter operands")
	}

	_, _ = newLRASourceAssertionContext(t)
	err := f.a.arithEqAdapter.MkAxioms(xv, xv)
	checkLRASourceAssertionResult(t, err, true, ast.ErrASTDomain)
}

func TestSMTContextDisplayFullCompositionAndRefusal(t *testing.T) {
	m := astTestManager(t)
	k, err := NewSMTKernel(m, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, k.Close()) })
	k.assertedInconsistent = true
	k.internalizer.inconsistent = true
	var out bytes.Buffer
	astTestMust(t, k.internalizer.DisplayContext(&out))
	text := out.String()
	prefix := "Logical context:\nscope-lvl: 0\nbase-lvl:  0\nsearch-lvl:  0\ninconsistent(): 1\nm_asserted_formulas.inconsistent(): 1\n"
	if !strings.HasPrefix(text, prefix) {
		t.Fatalf("context header:\n%s", text)
	}
	if strings.Index(text, "decl2enodes:\n") > strings.Index(text, "hot bool vars:\n") {
		t.Fatal("context section order", text)
	}
	writer := &smtLRAOutputPrefix{remaining: len(prefix) - 1}
	if err = k.internalizer.DisplayContext(writer); !errors.Is(err, io.ErrClosedPipe) || writer.text.String() != prefix[:len(prefix)-1] {
		t.Fatal("context writer refusal", err, writer.text.String())
	}
	if err = k.internalizer.DisplayContext(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil context output", err)
	}
}
