package smtcore

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
)

type lraLifecycleTraceWriter struct {
	output      bytes.Buffer
	owner       *SMTLRATheory
	refuse      bool
	seen        bool
	firstCount  uint32
	factorySeen bool
}

func (w *lraLifecycleTraceWriter) Write(p []byte) (int, error) {
	if !w.seen {
		w.seen = true
		w.firstCount = w.owner.numeric.statistics.makeFeasible
		w.factorySeen = w.owner.modelOwner != nil
	}
	if w.refuse {
		return 0, io.ErrClosedPipe
	}
	return w.output.Write(p)
}

func TestSMTLRALifecycleTraceFeasibilityCacheAndFactoryOrdering(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f, g := lraModelFixture(t)
		x := f.variable("lifecycle-x")
		term := f.app(smtArithOpAdd, x, f.number(1))
		v, err := f.a.internalizeDef(term)
		astTestMust(t, err)
		lraFix(t, f, x, 3)
		f.propagate(true)
		astTestMust(t, f.a.initializeVariableValues())
		previous, err := f.a.cachedModelValue(v)
		astTestMust(t, err)
		if previous.Cmp(big.NewRat(4, 1)) != 0 {
			t.Fatal("initial model", previous)
		}
		index, _ := f.a.numeric.index(v)
		writer := &lraLifecycleTraceWriter{owner: f.a, refuse: refuse}
		tags := diagnostic.NewTraceTagsWithWriter(writer)
		tags.Enable("arith_verbose")
		f.a.trace = tags
		expectedCount := f.a.numeric.statistics.makeFeasible + 1
		result, err := f.a.makeFeasible()
		astTestMust(t, err)
		if result != lbool.True {
			t.Fatal(result)
		}
		if tags.ShouldTrace("arith_verbose") {
			cached := f.a.modelValues[index]
			if !writer.seen || writer.firstCount != expectedCount || writer.factorySeen || cached == nil || f.a.testBigOf(cached).Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("counter/reset/term cache order", writer.firstCount, expectedCount, writer.factorySeen, cached)
			}
			if !refuse && !strings.Contains(writer.output.String(), "make_feasible src/smt/theory_lra.cpp:2150") {
				t.Fatal(writer.output.String())
			}
		} else if writer.seen || len(f.a.modelValues) != 0 {
			t.Fatal("native trace/cache evaluation")
		}
		modelWriter := &lraLifecycleTraceWriter{owner: f.a, refuse: refuse}
		modelTags := diagnostic.NewTraceTagsWithWriter(modelWriter)
		modelTags.Enable("arith")
		f.a.trace = modelTags
		astTestMust(t, f.a.InitializeModel(g))
		value, err := f.a.cachedModelValue(v)
		astTestMust(t, err)
		want := int64(4)
		if tags.ShouldTrace("arith_verbose") {
			want = 1
		}
		if value.Cmp(big.NewRat(want, 1)) != 0 {
			t.Fatal("source preserves cached term across model initialization", value, want)
		}
		if modelTags.ShouldTrace("arith") {
			if !modelWriter.seen || !modelWriter.factorySeen {
				t.Fatal("model trace preceded registered factory")
			}
			if !refuse && !strings.Contains(modelWriter.output.String(), "init_model src/smt/theory_lra.cpp:2258") {
				t.Fatal(modelWriter.output.String())
			}
		} else if modelWriter.seen {
			t.Fatal("native model trace executed")
		}
		if f.a.numeric.stopped {
			t.Fatal("ostream badbit stopped numeric owner")
		}
	}
}

func TestSMTLRALifecycleTraceSharedCandidateOrderAndCacheEffects(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		config := f.a.config
		config.PropagateEqs = false
		astTestMust(t, f.a.ConfigureArithmetic(config))
		decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "lifecycle-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
		xs := []ast.ASTHandle{f.variable("lifecycle-a"), f.variable("lifecycle-b")}
		for _, x := range xs {
			h := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
			astTestMust(t, f.s.Internalize(h, false))
			lraFix(t, f, x, 1)
		}
		term := f.app(smtArithOpAdd, xs[0], xs[1])
		v, err := f.a.internalizeDef(term)
		astTestMust(t, err)
		f.propagate(true)
		for _, x := range xs {
			astTestMust(t, f.s.relevancy.MarkRelevant(x))
		}
		astTestMust(t, f.s.relevancy.Propagate())
		astTestMust(t, f.s.PushScope())
		writer := &lraLifecycleTraceWriter{owner: f.a, refuse: refuse}
		tags := diagnostic.NewTraceTagsWithWriter(writer)
		tags.Enable("arith")
		f.a.trace = tags
		progress, err := f.a.assumeSharedEqualities()
		astTestMust(t, err)
		if !progress || len(f.a.assumeCandidates) != 1 || f.a.assumeHead != 1 {
			t.Fatal(progress, f.a.assumeCandidates, f.a.assumeHead)
		}
		index, _ := f.a.numeric.index(v)
		if tags.ShouldTrace("arith") {
			if value := f.a.modelValues[index]; value == nil || value.Sign() != 0 {
				t.Fatal("shared display did not run after random update", value)
			}
			if !refuse {
				text := writer.output.String()
				previous := -1
				for _, site := range []string{"assume_eqs src/smt/theory_lra.cpp:1081", "assume_eqs src/smt/theory_lra.cpp:1091", "assume_eqs src/smt/theory_lra.cpp:1111", "delayed_assume_eqs src/smt/theory_lra.cpp:1138"} {
					pos := strings.Index(text, site)
					if pos <= previous {
						t.Fatal(site, text)
					}
					previous = pos
				}
				if !strings.Contains(text, "v0 1 v1 0 \n") {
					t.Fatal("shared var directory", text)
				}
				pair := f.a.assumeCandidates[0]
				left, right := "lifecycle-a", "lifecycle-b"
				if pair.left == 1 {
					left, right = right, left
				}
				if strings.Count(text, left+" = "+right+"\n") != 2 || !strings.Contains(text, fmt.Sprintf("assuming eq: v%d = v%d\n", pair.left, pair.right)) {
					t.Fatal("native duplicated candidate/recheck", text)
				}
			}
		} else if writer.seen || len(f.a.modelValues) != 0 {
			t.Fatal("native shared trace evaluated operands")
		}
		astTestMust(t, f.s.PopScopes(1))
		if len(f.a.assumeCandidates) != 0 || f.a.assumeHead != 0 {
			t.Fatal("trace damaged scoped candidate restore")
		}
		if f.a.numeric.stopped {
			t.Fatal("shared trace badbit stopped LP")
		}
	}
}

func TestSMTLRALifecycleTraceInconclusiveStatus(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	f.a.numeric.solver.Settings.SimplexStrategy = lp.StrategyLU
	x, y := f.variable("lifecycle-cancel-x"), f.variable("lifecycle-cancel-y")
	f.assert(f.app(smtArithOpGE, f.app(smtArithOpAdd, x, y), f.number(3)), true)
	lraFinalCheckAssignments(t, f)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("arith")
	f.a.trace = tags
	f.m.Limit.Cancel()
	defer f.m.Limit.ResetCancel()
	result, err := f.a.makeFeasible()
	astTestMust(t, err)
	if result != lbool.Undef {
		t.Fatal("LP did not reach cancellation", result)
	}
	want := "status treated as inconclusive: TIME_EXHAUSTED\n"
	if tags.ShouldTrace("arith") {
		if !strings.Contains(out.String(), want) || !strings.Contains(out.String(), "make_feasible src/smt/theory_lra.cpp:2166") {
			t.Fatal(out.String())
		}
	} else if out.Len() != 0 {
		t.Fatal(out.String())
	}
}
