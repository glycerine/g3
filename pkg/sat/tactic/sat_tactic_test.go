package tactic

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func satTacticFixture(t *testing.T) (*ast.ASTManager, *SATTactic, *bytes.Buffer) {
	t.Helper()
	m, services := satModelFixture(t)
	previous, err := uparams.BindProcessGlobalParamsService(services.Globals)
	satOK(t, err)
	t.Cleanup(func() { _, err := uparams.BindProcessGlobalParamsService(previous); satOK(t, err) })
	var output bytes.Buffer
	r, err := NewSATTactic(services.Context, nil, services, &output)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, r.Close()) })
	return m, r, &output
}

func TestSATTacticBooleanTruthTablesAndStatistics(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			m, r, _ := satTacticFixture(t)
			p := goalTestAtom(t, m, "p")
			q := goalTestAtom(t, m, "q")
			g := goalTestNew(t, m, false, false)
			for clause := 0; clause < 4; clause++ {
				if mask&(1<<clause) == 0 {
					continue
				}
				a, b := p, q
				if clause&1 != 0 {
					a = goalTestApp(t, m, ast.ASTOpNot, a)
				}
				if clause&2 != 0 {
					b = goalTestApp(t, m, ast.ASTOpNot, b)
				}
				satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, a, b)))
			}
			var result tactic.TacticResult
			t.Cleanup(func() { satOK(t, result.Close()) })
			satOK(t, r.Apply(g, &result))
			wantSAT := false
			valid := func(pv, qv bool) bool {
				for c := 0; c < 4; c++ {
					if mask&(1<<c) != 0 && (pv == (c&1 != 0)) && (qv == (c&2 != 0)) {
						return false
					}
				}
				return true
			}
			for values := 0; values < 4; values++ {
				wantSAT = wantSAT || valid(values&1 != 0, values&2 != 0)
			}
			if result.DecidedSat() != wantSAT || result.DecidedUnsat() == wantSAT {
				t.Fatal("truth table", wantSAT, result.DecidedSat(), result.DecidedUnsat())
			}
			if wantSAT {
				var md tactic.ModelRef
				t.Cleanup(func() { satOK(t, md.Close()) })
				satOK(t, result.ModelConverter.ApplyUnindexed(&md))
				model, err := md.Get()
				satOK(t, err)
				read := func(atom ast.ASTHandle) bool {
					d, err := m.GetDecl(atom)
					satOK(t, err)
					v, err := model.ConstantInterpretation(d)
					satOK(t, err)
					return m.IsTrue(v)
				}
				if !valid(read(p), read(q)) {
					t.Fatal("SAT tactic published an invalid model")
				}
			}
		})
	}
	m, r, _ := satTacticFixture(t)
	p := goalTestAtom(t, m, "counter")
	var counts []float64
	for i := 0; i < 2; i++ {
		g := goalTestNew(t, m, false, false)
		satOK(t, g.Assert(p))
		var result tactic.TacticResult
		satOK(t, r.Apply(g, &result))
		satOK(t, result.Close())
		counts = append(counts, r.CollectStatistics()["mk bool var"])
	}
	if counts[0] == 0 || counts[1] != 2*counts[0] {
		t.Fatal("statistics must accumulate across fresh solvers", counts)
	}
	satOK(t, r.ResetStatistics())
	if len(r.CollectStatistics()) != 0 {
		t.Fatal(r.CollectStatistics())
	}
}

func TestSATTacticDependencyKeysSurviveGoalReset(t *testing.T) {
	for _, kind := range []string{"positive", "negative", "compound"} {
		t.Run(kind, func(t *testing.T) {
			m, r, _ := satTacticFixture(t)
			boolean, err := m.MkBoolSort()
			satOK(t, err)
			label, err := m.MkConst(astTestSymbol(t, m, "label"), boolean)
			satOK(t, err)
			if kind == "negative" {
				label, err = m.MkNot(label)
				satOK(t, err)
			}
			if kind == "compound" {
				q, err := m.MkConst(astTestSymbol(t, m, "q"), boolean)
				satOK(t, err)
				label, err = m.MkOr(label, q)
				satOK(t, err)
			}
			dep, err := m.DependencyLeaf(label)
			satOK(t, err)
			g := goalTestNew(t, m, false, true)
			f, err := m.MkFalse()
			satOK(t, err)
			satOK(t, g.AssertWithProof(f, ast.ASTHandle{}, dep))
			var result tactic.TacticResult
			t.Cleanup(func() { satOK(t, result.Close()) })
			satOK(t, r.Apply(g, &result))
			if !result.DecidedUnsat() {
				t.Fatal("guarded false must be UNSAT")
			}
			dependency, err := g.Dependency(0)
			satOK(t, err)
			labels, err := m.DependencyLinearize(dependency, nil)
			satOK(t, err)
			if len(labels) != 1 || labels[0] != label {
				t.Fatal("original signed/compound core label", labels)
			}
			if _, err = m.GetKind(label); err != nil {
				t.Fatal("label was retired with the input goal", err)
			}
		})
	}
}

func TestSATTacticCancellationBoundaryAndResidualTranslation(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprint(nested), func(t *testing.T) {
			m, r, _ := satTacticFixture(t)
			p := goalTestAtom(t, m, "p")
			g := goalTestNew(t, m, false, false)
			f := p
			if nested {
				f = goalTestApp(t, m, ast.ASTOpOr, p, goalTestAtom(t, m, "q"))
			}
			satOK(t, g.Assert(f))
			var result tactic.TacticResult
			t.Cleanup(func() { satOK(t, result.Close()) })
			m.ResourceLimit().IncCancel()
			err := r.Apply(g, &result)
			m.ResourceLimit().DecCancel()
			var ex tactic.TacticException
			if !errors.As(err, &ex) || err.Error() != "canceled" {
				t.Fatal("source exception boundary", err)
			}
			count := r.CollectStatistics()["mk bool var"]
			if (!nested && count == 0) || (nested && count != 0) {
				t.Fatal("only caught solver_exception collects interrupted statistics", nested, count)
			}
			satOK(t, r.Cleanup())
		})
	}
	m, r, _ := satTacticFixture(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	params, err := r.context.NewValue()
	satOK(t, err)
	defer func() { satOK(t, r.context.Del(&params)) }()
	key := "max_conflicts"
	satOK(t, r.context.SetUIntName(&params, &key, 0))
	satOK(t, r.UpdateParams(&params))
	other, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	satOK(t, err)
	t.Cleanup(func() { satOK(t, other.Close()) })
	translated, err := r.Translate(other)
	satOK(t, err)
	tr := translated.(*SATTactic)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	satOK(t, r.UpdateParams(nil))
	satOK(t, r.Close())
	g := goalTestNew(t, m, false, false)
	satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, p, q)))
	var result tactic.TacticResult
	t.Cleanup(func() { satOK(t, result.Close()) })
	// The source factory/translation is manager-independent.
	satOK(t, tr.Apply(g, &result))
	if result.DecidedSat() || result.DecidedUnsat() || result.ModelConverter == nil {
		t.Fatal("max_conflicts snapshot must recover an undecided goal")
	}
}

func TestSATTacticCPPGoalRecoveryAndLiveDiagnostics(t *testing.T) {
	for _, dimacs := range []bool{false, true} {
		t.Run(fmt.Sprint(dimacs), func(t *testing.T) {
			m, r, output := satTacticFixture(t)
			var traces bytes.Buffer
			r.services.Trace = diagnostic.NewTraceTagsWithWriter(&traces)
			for _, tag := range []string{"before_sat_solver", "sat_solver_unknown", "sat_dimacs", "sat_stats", "sat_tactic"} {
				r.services.Trace.Enable(tag)
			}
			p := goalTestAtom(t, m, "p")
			q := goalTestAtom(t, m, "q")
			f := goalTestApp(t, m, ast.ASTOpIff, goalTestApp(t, m, ast.ASTOpOr, p, q), goalTestAtom(t, m, "r"))
			params, err := r.context.NewValue()
			satOK(t, err)
			defer func() { satOK(t, r.context.Del(&params)) }()
			key := "max_conflicts"
			satOK(t, r.context.SetUIntName(&params, &key, 0))
			if dimacs {
				key = "dimacs.display"
				satOK(t, r.context.SetBoolName(&params, &key, true))
			}
			satOK(t, r.UpdateParams(&params))
			g := goalTestNew(t, m, false, false)
			satOK(t, g.Assert(f))
			var result tactic.TacticResult
			t.Cleanup(func() { satOK(t, result.Close()) })
			satOK(t, r.Apply(g, &result))
			var actual bytes.Buffer
			actual.Write(output.Bytes())
			satOK(t, g.Display(&actual))
			actual.WriteByte('\n')
			satOK(t, result.ModelConverter.Display(&actual))
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				option := ""
				if dimacs {
					option = " :dimacs.display true"
				}
				script := "(set-option :produce-models true)(declare-const p Bool)(declare-const q Bool)(declare-const r Bool)(assert (= (or p q) r))(apply (using-params sat :max_conflicts 0" + option + ") :print true :print_model_converter true)"
				cmd := exec.Command(reference, "-in")
				cmd.Stdin = strings.NewReader(script)
				cpp, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatal(err, string(cpp))
				}
				// C++ apply prints goal attributes; compare its formula body and converter.
				expected := string(cpp)
				expected = strings.ReplaceAll(expected, "(goals\n", "")
				attributes := regexp.MustCompile(`\n\s*:precision [^\n]+\)\n\)`)
				expected = attributes.ReplaceAllString(expected, ")")
				normalize := func(s string) string {
					s = regexp.MustCompile(`k![0-9]+`).ReplaceAllString(s, "helper")
					return strings.Join(strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(s)), " ")
				}
				if normalize(actual.String()) != normalize(expected) {
					t.Fatalf("Go: %s\nC++: %s", actual.String(), cpp)
				}
			}
			if r.services.Trace.ShouldTrace("sat_stats") {
				for _, line := range []string{"sat_tactic.cpp:51", "sat_tactic.cpp:58", "sat_tactic.cpp:68", "sat_tactic.cpp:198"} {
					if !strings.Contains(traces.String(), line) {
						t.Fatal("missing live trace", line, traces.String())
					}
				}
			}
		})
	}
}

func TestSATTacticDescriptorsProofProfilesAndBorrowedGoals(t *testing.T) {
	m, r, _ := satTacticFixture(t)
	d, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	satOK(t, err)
	defer func() { satOK(t, d.Close()) }()
	satOK(t, r.CollectParamDescrs(d))
	satOK(t, r.CollectParamDescrs(d))
	n, err := d.Size()
	satOK(t, err)
	if n != 48 {
		t.Fatal("two additional translator descriptors plus 46 solver descriptors", n)
	}
	for _, key := range []string{"ite_extra", "learned", "max_memory", "resolution", "dimacs.display"} {
		yes, err := d.ContainsName(key)
		satOK(t, err)
		if !yes {
			t.Fatal("missing descriptor", key)
		}
	}
	g := goalTestNew(t, m, false, false)
	p := goalTestAtom(t, m, "borrowed")
	satOK(t, g.Assert(p))
	refs := g.RefCount
	var result tactic.TacticResult
	defer func() { satOK(t, result.Close()) }()
	satOK(t, r.Apply(g, &result))
	if g.RefCount != refs || len(result.Goals) != 1 || result.Goals[0] != g {
		t.Fatal("caller goal reference changed")
	}
	depth, err := g.Depth()
	satOK(t, err)
	if depth != 1 {
		t.Fatal("source depth publication", depth)
	}
	satOK(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proofGoal, err := tactic.NewGoal(m, true, false, false)
	satOK(t, err)
	defer func() { satOK(t, proofGoal.Close()) }()
	var ex tactic.TacticException
	err = r.Apply(proofGoal, &result)
	if ast.ASTSourceDebug {
		if !errors.Is(err, sat.ErrSATAssertion) || len(result.Goals) != 1 {
			t.Fatal("imp constructor assertion precedes output reset", err)
		}
	} else if !errors.As(err, &ex) || err.Error() != "sat does not support proof production" || len(result.Goals) != 0 {
		t.Fatal(err)
	}
	satOK(t, m.ToggleProofMode(ast.ASTProofDisabled))
	copy := *r
	if err = copy.Close(); !errors.Is(err, sat.ErrSATOwner) {
		t.Fatal("copied tactic owner", err)
	}
	r.active = true
	err = r.Cleanup()
	r.active = false
	if ast.ASTSourceDebug {
		if !errors.Is(err, sat.ErrSATAssertion) {
			t.Fatal(err)
		}
	} else {
		satOK(t, err)
	}
}

func TestSATPreprocessorFactoryParameterPrecedence(t *testing.T) {
	m, base, _ := satTacticFixture(t)
	p, err := base.context.NewValue()
	satOK(t, err)
	defer func() { satOK(t, base.context.Del(&p)) }()
	key := "max_conflicts"
	satOK(t, base.context.SetUIntName(&p, &key, 100))
	preprocessor, err := MakeSATPreprocessorTactic(base.context, &p, base.services, io.Discard)
	satOK(t, err)
	satOK(t, preprocessor.IncRef())
	defer func() { satOK(t, preprocessor.DecRef()) }()
	a := goalTestAtom(t, m, "pre-p")
	b := goalTestAtom(t, m, "pre-q")
	for i := 0; i < 2; i++ {
		g := goalTestNew(t, m, false, false)
		satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, a, b)))
		var result tactic.TacticResult
		satOK(t, tactic.Exec(preprocessor, g, &result))
		if result.DecidedSat() || result.DecidedUnsat() || result.ModelConverter == nil {
			t.Fatal("fixed max_conflicts=0 lost to caller update")
		}
		satOK(t, result.Close())
		satOK(t, preprocessor.UpdateParams(&p))
	}
}

func TestSATTacticConditionalInvariantAndVerboseStatus(t *testing.T) {
	m, r, _ := satTacticFixture(t)
	level, err := diagnostic.ProcessVerbosityLevel()
	satOK(t, err)
	previous, err := diagnostic.BorrowProcessVerboseStream()
	satOK(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	satOK(t, err)
	defer func() {
		satOK(t, diagnostic.SetProcessVerbosityLevel(level))
		satOK(t, diagnostic.SetProcessVerboseStream(previous))
		satOK(t, stream.Close())
	}()
	satOK(t, diagnostic.SetProcessVerboseStream(stream))
	satOK(t, diagnostic.SetProcessVerbosityLevel(10))
	if ast.ASTSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		wasEnabled, err := d.IsDebugEnabled("sat_solver")
		satOK(t, err)
		satOK(t, d.EnableDebug("sat_solver"))
		defer func() {
			if !wasEnabled {
				satOK(t, d.DisableDebug("sat_solver"))
			}
		}()
	}
	p := goalTestAtom(t, m, "status-p")
	q := goalTestAtom(t, m, "status-q")
	g := goalTestNew(t, m, false, false)
	satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpOr, p, q)))
	var result tactic.TacticResult
	defer func() { satOK(t, result.Close()) }()
	satOK(t, r.Apply(g, &result))
	text, err := stream.String()
	satOK(t, err)
	if !strings.Contains(text, "(sat-status\n") || !strings.Contains(text, ":vars            2\n") {
		t.Fatal(text)
	}
	if !result.DecidedSat() {
		t.Fatal("diagnostics changed the Boolean result")
	}
}

func TestSATTacticAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "preprocessor", "translate", "sat", "core", "residual", "close"} {
		success := false
		for stop := int64(0); stop < 1024; stop++ {
			t.Run(fmt.Sprint(phase, "/", stop), func(t *testing.T) {
				m, r, _ := satTacticFixture(t)
				p := goalTestAtom(t, m, "frontier-p")
				q := goalTestAtom(t, m, "frontier-q")
				g := goalTestNew(t, m, false, phase == "core")
				if phase == "core" {
					label := goalTestApp(t, m, ast.ASTOpNot, p)
					dep, err := m.DependencyLeaf(label)
					satOK(t, err)
					f, err := m.MkFalse()
					satOK(t, err)
					satOK(t, g.AssertWithProof(f, ast.ASTHandle{}, dep))
				} else {
					satOK(t, g.Assert(goalTestApp(t, m, ast.ASTOpIff, goalTestApp(t, m, ast.ASTOpOr, p, q), goalTestAtom(t, m, "frontier-r"))))
				}
				if phase == "residual" {
					params, err := r.context.NewValue()
					satOK(t, err)
					defer func() { satOK(t, r.context.Del(&params)) }()
					key := "max_conflicts"
					satOK(t, r.context.SetUIntName(&params, &key, 0))
					satOK(t, r.UpdateParams(&params))
				}
				var result tactic.TacticResult
				defer func() { satOK(t, result.Close()) }()
				var fresh *SATTactic
				var wrapped tactic.Tactic
				run := func() error { return r.Apply(g, &result) }
				if phase == "constructor" {
					run = func() (err error) {
						fresh, err = NewSATTactic(r.context, &r.params, r.services, io.Discard)
						return err
					}
				}
				if phase == "preprocessor" {
					run = func() (err error) {
						wrapped, err = MakeSATPreprocessorTactic(r.context, &r.params, r.services, io.Discard)
						return err
					}
				}
				if phase == "translate" {
					run = func() (err error) { wrapped, err = r.Translate(m); return err }
				}
				if phase == "close" {
					satOK(t, r.Apply(g, &result))
					run = r.Close
				}
				m.Budget().FailAfter(stop)
				err := run()
				m.Budget().FailAfter(-1)
				if fresh != nil {
					satOK(t, fresh.Close())
				}
				if wrapped != nil {
					satOK(t, wrapped.IncRef())
					satOK(t, wrapped.DecRef())
				}
				satOK(t, result.Close())
				var stopped *SATApplicationFailure
				if errors.As(err, &stopped) {
					message := err.Error()
					satOK(t, stopped.Cleanup())
					if err.Error() != message {
						t.Fatal("lost source failure history")
					}
				}
				if phase == "close" || r.closed {
					satOK(t, r.Close())
				} else {
					satOK(t, r.Cleanup())
				}
				if err == nil {
					success = true
				} else if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
			})
			if success {
				break
			}
		}
		if !success {
			t.Fatal("no successful reservation frontier", phase)
		}
	}
}
