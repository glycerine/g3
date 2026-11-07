package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func smtLemmaFixture(t *testing.T) (*lraFrontendFixture, []SMTLiteral, []SMTEnodePair) {
	t.Helper()
	f := newLRAFrontendFixture(t, true)
	var ls []SMTLiteral
	for _, name := range []string{"lemma-p", "lemma-q"} {
		h := smtRelevancyBool(t, f.m, name)
		astTestMust(t, f.s.Internalize(h, true))
		l, err := f.s.Literal(h)
		astTestMust(t, err)
		ls = append(ls, l)
	}
	x, y := f.variable("lemma-x"), f.variable("lemma-y")
	astTestMust(t, f.s.Internalize(x, false))
	astTestMust(t, f.s.Internalize(y, false))
	return f, ls, []SMTEnodePair{{First: f.s.context.enodes.get(x), Second: f.s.context.enodes.get(y)}}
}

func TestSMTLemmaProblemPremiseOrderAndConclusionPolarity(t *testing.T) {
	f, ls, eqs := smtLemmaFixture(t)
	logic := astTestSymbol(t, f.m, "QF_UFLRA")
	var out bytes.Buffer
	before := f.m.Stats().LiveReferences
	astTestMust(t, f.s.DisplayLemmaAsSMTProblem(&out, []SMTLiteral{ls[0], ls[1].Negated(), ls[0]}, eqs, ls[1].Negated(), logic))
	want := "(set-logic QF_UFLRA)\n(declare-fun lemma-p () Bool)\n(declare-fun lemma-q () Bool)\n(declare-fun lemma-y () Real)\n(declare-fun lemma-x () Real)\n(assert lemma-p)\n(assert (not lemma-q))\n(assert lemma-p)\n(assert (= lemma-x lemma-y))\n(assert lemma-q)\n"
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	if f.s.lemmaProblem != nil || f.m.Stats().LiveReferences != before {
		t.Fatal("completed lexical roots leaked", f.s.lemmaProblem, before, f.m.Stats().LiveReferences)
	}
	out.Reset()
	astTestMust(t, f.s.DisplayLemmaAsSMTProblem(&out, nil, nil, SMTFalseLiteral, usymbol.Symbol{}))
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
	out.Reset()
	astTestMust(t, f.s.DisplayLemmaAsSMTProblem(&out, nil, nil, SMTTrueLiteral, usymbol.Symbol{}))
	if out.String() != "(assert false)\n" {
		t.Fatal(out.String())
	}
}

func TestSMTLemmaProblemEveryWriterPrefixAndGuards(t *testing.T) {
	f, ls, eqs := smtLemmaFixture(t)
	logic := astTestSymbol(t, f.m, "QF_UFLRA")
	var out bytes.Buffer
	astTestMust(t, f.s.DisplayLemmaAsSMTProblem(&out, ls, eqs, ls[0], logic))
	want := out.String()
	before := f.m.Stats().LiveReferences
	for cut := 0; cut < len(want); cut++ {
		w := &smtLRAOutputPrefix{remaining: cut}
		err := f.s.DisplayLemmaAsSMTProblem(w, ls, eqs, ls[0], logic)
		if !errors.Is(err, io.ErrClosedPipe) || w.text.String() != want[:cut] || f.s.lemmaProblem != nil || f.m.Stats().LiveReferences != before {
			t.Fatal(cut, err, w.text.String(), want[:cut], f.m.Stats().LiveReferences, before)
		}
	}
	if err := f.s.DisplayLemmaAsSMTProblem(nil, nil, nil, SMTFalseLiteral, usymbol.Symbol{}); err != ast.ErrASTHandle {
		t.Fatal(err)
	}
	if err := f.s.DisplayLemmaAsSMTProblem(&out, nil, []SMTEnodePair{{}}, SMTFalseLiteral, logic); !errors.Is(err, ast.ErrASTDomain) || f.s.lemmaProblem != nil {
		t.Fatal(err)
	}
	if err := f.s.DisplayLemmaAsSMTProblem(&out, []SMTLiteral{SMTNullLiteral}, nil, SMTFalseLiteral, logic); err == nil || f.s.lemmaProblem != nil {
		t.Fatal(err)
	}
	if err := f.s.DisplayLemmaAsSMTProblem(smtLRAShortOutput{}, ls, nil, SMTFalseLiteral, logic); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

func TestSMTLemmaProblemSharedFileSequenceFailuresAndWrap(t *testing.T) {
	t.Chdir(t.TempDir())
	old := smtLemmaID
	defer func() { smtLemmaID = old }()
	smtLemmaID = 0
	f, ls, eqs := smtLemmaFixture(t)
	for i, pairs := range [][]SMTEnodePair{nil, eqs} {
		name, err := f.s.DumpLemmaAsSMTProblem(ls, pairs, SMTFalseLiteral, usymbol.Symbol{})
		astTestMust(t, err)
		if name != fmt.Sprintf("lemma_%d.smt2", i) {
			t.Fatal(name)
		}
		text, err := os.ReadFile(name)
		astTestMust(t, err)
		var expected bytes.Buffer
		astTestMust(t, f.s.DisplayLemmaAsSMTProblem(&expected, ls, pairs, SMTFalseLiteral, usymbol.Symbol{}))
		if string(text) != expected.String() {
			t.Fatal(string(text), expected.String())
		}
	}
	astTestMust(t, os.Mkdir("lemma_2.smt2", 0700))
	name, err := f.s.DumpLemmaAsSMTProblem(ls, nil, SMTFalseLiteral, usymbol.Symbol{})
	if err == nil || name != "lemma_2.smt2" || smtLemmaID != 3 || f.s.lemmaProblem != nil {
		t.Fatal("open refusal did not finish source operands/counter", name, err, smtLemmaID)
	}
	_, err = f.s.DumpLemmaAsSMTProblem([]SMTLiteral{SMTNullLiteral}, nil, SMTFalseLiteral, usymbol.Symbol{})
	if err == nil || smtLemmaID != 3 {
		t.Fatal("AST exception consumed source counter", err, smtLemmaID)
	}
	for _, n := range []uint32{1 << 31, ^uint32(0), 0} {
		smtLemmaID = n
		name, err = f.s.DumpLemmaAsSMTProblem(nil, nil, SMTFalseLiteral, usymbol.Symbol{})
		astTestMust(t, err)
		if name != fmt.Sprintf("lemma_%d.smt2", int32(n)) || smtLemmaID != n+1 {
			t.Fatal(name, smtLemmaID, n)
		}
		if _, err = os.Stat(filepath.Clean(name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSMTLemmaProblemPhysicalRootFailures(t *testing.T) {
	failures, successes := 0, 0
	for _, stop := range []int64{-1, 0, 1, 2, 4, 8, 16} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f, ls, eqs := smtLemmaFixture(t)
			var padding []ast.ASTRef
			t.Cleanup(func() {
				f.m.Budget().FailAfter(-1)
				for i := range padding {
					astTestMust(t, f.m.DelRef(&padding[i]))
				}
			})
			expression, err := f.s.BoolExpression(ls[0].Var())
			astTestMust(t, err)
			f.m.Budget().FailAfter(0)
			cold := false
			for i := 0; i < 8192; i++ {
				r, err := f.m.NewRef(expression)
				if err != nil {
					cold = true
					break
				}
				padding = append(padding, r)
			}
			f.m.Budget().FailAfter(-1)
			if !cold {
				t.Fatal("reference directory never reached a cold page")
			}
			before := f.m.Stats().LiveReferences
			f.m.Budget().FailAfter(stop)
			var out bytes.Buffer
			err = f.s.DisplayLemmaAsSMTProblem(&out, ls, eqs, ls[0], usymbol.Symbol{})
			f.m.Budget().FailAfter(-1)
			if err != nil {
				var failure umemory.Failure
				if stop < 0 || (!errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == umemory.FailureMemout)) {
					t.Fatal("unexpected failure", stop, err)
				}
				failures++
			} else {
				successes++
				if !strings.Contains(out.String(), "(assert (not lemma-p))\n") {
					t.Fatal(out.String())
				}
			}
			if f.s.lemmaProblem != nil {
				astTestMust(t, f.s.lemmaProblem.close())
				f.s.lemmaProblem = nil
			}
			if f.m.Stats().LiveReferences != before {
				t.Fatal("lexical reference owner leak", stop, err, before, f.m.Stats().LiveReferences)
			}
		})
	}
	if failures == 0 || successes == 0 {
		t.Fatal("physical root campaign", failures, successes)
	}
}
