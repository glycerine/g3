package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type lraRegistrationSnapshot struct {
	rows                                               uint32
	inequalities, constraints, pending, clauses, fixed int
}
type lraRegistrationWriter struct {
	owner            *SMTLRATheory
	output, observed bytes.Buffer
	snapshots        map[int]lraRegistrationSnapshot
	refuse           bool
	armAt            int
	armed            bool
}

func (w *lraRegistrationWriter) Write(p []byte) (int, error) {
	w.observed.Write(p)
	if w.armAt == -1 && !w.armed {
		w.armed = true
		w.owner.numeric.manager.Budget().FailAfter(0)
	}
	for _, line := range []int{440, 550, 640, 1220, 1612, 1627, 1973} {
		if !strings.Contains(w.observed.String(), fmt.Sprintf("src/smt/theory_lra.cpp:%d ", line)) {
			continue
		}
		if _, seen := w.snapshots[line]; seen {
			continue
		}
		a := w.owner
		w.snapshots[line] = lraRegistrationSnapshot{a.numeric.statistics.rows, len(a.numeric.inequalities), len(a.numeric.solver.Constraints), len(a.newBounds), len(a.state.Clauses()), len(a.fixedVariables)}
		if w.armAt == line && !w.armed {
			w.armed = true
			a.numeric.manager.Budget().FailAfter(0)
		}
	}
	if w.refuse {
		return 0, io.ErrClosedPipe
	}
	return w.output.Write(p)
}
func lraRegistrationTrace(f *lraFrontendFixture, refuse bool, tags ...string) (*lraRegistrationWriter, *diagnostic.TraceTags) {
	w := &lraRegistrationWriter{owner: f.a, refuse: refuse, snapshots: make(map[int]lraRegistrationSnapshot)}
	trace := diagnostic.NewTraceTagsWithWriter(w)
	for _, tag := range tags {
		trace.Enable(tag)
	}
	f.a.trace = trace
	return w, trace
}
func TestSMTLRARegistrationTraceNewTermCacheAndNumeralOrder(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x := f.variable("trace-term-x")
		term := f.app(smtArithOpAdd, f.app(smtArithOpMul, f.number(2), x), f.number(3))
		astTestMust(t, f.a.PushScope())
		w, tags := lraRegistrationTrace(f, refuse, "arith")
		v, err := f.a.internalizeDef(term)
		astTestMust(t, err)
		index, _ := f.a.numeric.index(v)
		want := lraObjectiveTraceRecord("arith", "internalize_def", 640, fmt.Sprintf("v%d := (+ (* 2.0 trace-term-x) 3.0) slack: %d scopes: 1\n3 + 2v0\n", v, index))
		if !tags.ShouldTrace("arith") || refuse {
			want = ""
		}
		if w.output.String() != want {
			t.Fatal(w.output.String(), want)
		}
		w, tags = lraRegistrationTrace(f, refuse, "arith")
		again, err := f.a.internalizeDef(term)
		astTestMust(t, err)
		if again != v || w.output.Len() != 0 || len(w.snapshots) != 0 {
			t.Fatal("mapped term was retraced")
		}
		seven := f.number(7)
		w, tags = lraRegistrationTrace(f, refuse, "arith")
		constant, err := f.a.internalizeDef(seven)
		astTestMust(t, err)
		if len(f.a.fixedVariables) != 1 || f.a.numeric.stopped {
			t.Fatal("numeral cache not published")
		}
		if tags.ShouldTrace("arith") && !refuse && w.snapshots[640].fixed != 0 {
			t.Fatal("numeral cache precedes term trace")
		}
		if _, ok := f.a.numeric.index(constant); !ok {
			t.Fatal("constant mapping lost")
		}
		astTestMust(t, f.a.PopScopes(1))
		if _, ok := f.a.numeric.index(v); ok {
			t.Fatal("popped term mapping survives")
		}
	}
}
func TestSMTLRARegistrationTraceInequalityPublicationAndBadbit(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x := f.variable("trace-register-x")
		f.assert(f.app(smtArithOpGE, x, f.number(2)), true)
		astTestMust(t, f.s.propagateTheoryRelevancy())
		astTestMust(t, f.s.PropagateTheoryAssignments())
		w, tags := lraRegistrationTrace(f, refuse, "arith")
		astTestMust(t, f.a.Propagate())
		if f.a.numeric.statistics.rows != 1 || len(f.a.numeric.inequalities) != 1 || f.a.assignmentHead != 1 || f.a.numeric.stopped {
			t.Fatal("inequality publication damaged")
		}
		if tags.ShouldTrace("arith") && !refuse {
			first := lraObjectiveTraceRecord("arith", "assert_bound", 1973, "v0\n")
			second := lraObjectiveTraceRecord("arith", "add_ineq_constraint", 550, "v0 >= 2\n\n")
			if !strings.Contains(w.output.String(), first+second) {
				t.Fatal(w.output.String())
			}
			a, b := w.snapshots[1973], w.snapshots[550]
			if a.constraints != 1 || a.rows != 0 || a.inequalities != 0 || b.rows != 1 || b.inequalities != 1 {
				t.Fatal("creation/publication trace order", a, b)
			}
		} else if w.output.Len() != 0 {
			t.Fatal(w.output.String())
		}
	}
}
func TestSMTLRARegistrationTraceBadbitStillEvaluatesNumericPrinter(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("trace-printer-failure-x")
	y := f.variable("trace-printer-failure-y")
	large, ok := new(big.Rat).SetString(strings.Repeat("1234567890", 30) + "/7")
	if !ok {
		t.Fatal("invalid test coefficient")
	}
	coefficient := f.pin(f.m.ArithmeticNumeralRat(large, false))
	term := f.app(smtArithOpAdd, f.app(smtArithOpMul, coefficient, x), y, f.number(-5))
	f.assert(f.app(smtArithOpGE, term, f.number(2)), true)
	astTestMust(t, f.s.propagateTheoryRelevancy())
	astTestMust(t, f.s.PropagateTheoryAssignments())
	astTestMust(t, f.a.flushBoundAxioms())
	w, tags := lraRegistrationTrace(f, true, "arith")
	w.armAt = -1
	err := f.a.Propagate()
	f.a.numeric.manager.Budget().FailAfter(-1)
	if tags.ShouldTrace("arith") {
		var mem umemory.Failure
		if !w.armed || err == nil || (!errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &mem) && mem.Kind == umemory.FailureMemout)) {
			t.Fatal("badbit suppressed later printer allocation", w.armed, err)
		}
		if f.a.numeric.statistics.rows != 1 || len(f.a.numeric.inequalities) != 1 {
			t.Fatal("row metadata missing before printer failure")
		}
	} else {
		astTestMust(t, err)
		if w.armed {
			t.Fatal("native trace armed failure")
		}
	}
	astTestMust(t, f.a.Close())
}
func TestSMTLRARegistrationTraceFlushBatchOrderAndBinaryClauses(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		f := newLRAFrontendFixture(t, true)
		x, y := f.variable("trace-flush-x"), f.variable("trace-flush-y")
		lower1 := f.app(smtArithOpGE, x, f.number(1))
		upper4 := f.app(smtArithOpLE, x, f.number(4))
		lower3 := f.app(smtArithOpGE, x, f.number(3))
		for _, h := range []ast.ASTHandle{lower1, f.app(smtArithOpGE, y, f.number(2)), upper4, lower3} {
			astTestMust(t, f.s.Internalize(h, true))
		}
		w, tags := lraRegistrationTrace(f, refuse, "arith", "arith_verbose")
		astTestMust(t, f.a.Propagate())
		if len(f.a.newBounds) != 0 || len(f.s.Clauses()) != 3 || f.a.numeric.stopped {
			t.Fatal("flush publication damaged", len(f.s.Clauses()))
		}
		if tags.ShouldTrace("arith") && !refuse {
			want := lraObjectiveTraceRecord("arith", "flush_bound_axioms", 1612, "flush bound axioms\n")
			want += lraObjectiveTraceRecord("arith_verbose", "flush_bound_axioms", 1627, "v0  <= 3\nv0  <= 1\nv0  >= 4\n")
			want += lraObjectiveTraceRecord("arith", "mk_clause", 440, "(>= trace-flush-x 1.0)\n(not (>= trace-flush-x 3.0))\n")
			want += lraObjectiveTraceRecord("arith", "mk_clause", 440, "(>= trace-flush-x 1.0)\n(<= trace-flush-x 4.0)\n")
			want += lraObjectiveTraceRecord("arith", "mk_clause", 440, "(>= trace-flush-x 3.0)\n(<= trace-flush-x 4.0)\n")
			want += lraObjectiveTraceRecord("arith_verbose", "flush_bound_axioms", 1627, "v1  <= 2\n")
			if w.output.String() != want {
				t.Fatal(w.output.String(), want)
			}
			if w.snapshots[1612].pending != 4 || w.snapshots[1627].pending != 1 || w.snapshots[440].clauses != 0 {
				t.Fatal("batch/clause trace order", w.snapshots)
			}
		} else if w.output.Len() != 0 {
			t.Fatal(w.output.String())
		}
		w, tags = lraRegistrationTrace(f, refuse, "arith", "arith_verbose")
		astTestMust(t, f.a.Propagate())
		if w.output.Len() != 0 || len(w.snapshots) != 0 {
			t.Fatal("empty flush traced")
		}
	}
}
func TestSMTLRARegistrationTraceUnderspecifiedSharingBranches(t *testing.T) {
	for _, many := range []bool{false, true} {
		for _, refuse := range []bool{false, true} {
			f := newLRAFrontendFixture(t, true)
			x, y := f.variable("trace-sharing-x"), f.variable("trace-sharing-y")
			xv, err := f.a.internalizeDef(x)
			astTestMust(t, err)
			_, err = f.a.internalizeDef(f.app(smtArithOpDiv, x, y))
			astTestMust(t, err)
			if many {
				for _, n := range []int64{2, 3, 4, 5} {
					_, err = f.a.internalizeDef(f.app(smtArithOpAdd, x, f.number(n)))
					astTestMust(t, err)
				}
			}
			astTestMust(t, f.s.relevancy.MarkRelevant(x))
			astTestMust(t, f.s.relevancy.Propagate())
			astTestMust(t, f.s.PushScope())
			root := f.s.context.enodes.get(x).Root()
			parents := len(root.parents)
			if many != (parents > 2*len(f.a.underspecified)) {
				t.Fatal("sharing fixture missed branch")
			}
			w, tags := lraRegistrationTrace(f, refuse, "shared")
			shared, err := f.a.relevantShared(f.s.context.enodes.get(x))
			astTestMust(t, err)
			if !shared {
				t.Fatal("underspecified child not shared")
			}
			want := lraObjectiveTraceRecord("shared", "is_shared", 1220, fmt.Sprintf("1 %d %d\n", xv, parents))
			if !tags.ShouldTrace("shared") || refuse {
				want = ""
			}
			if w.output.String() != want {
				t.Fatal(w.output.String(), want)
			}
		}
	}
}
