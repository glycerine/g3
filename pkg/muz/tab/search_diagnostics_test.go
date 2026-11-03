package tab

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type tabDiagnosticsFixture struct {
	ctx                    *tabTestContext
	rules                  *base.RuleSet
	rw                     *rewriter.TheoryRewriter
	lite                   *qe.Lite
	querySolver, subSolver solver.Solver
	queries                [2]ast.ASTHandle
}

func tabDiagnosticsMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newTabDiagnosticsFixture(t *testing.T) *tabDiagnosticsFixture {
	t.Helper()
	ctx, err := newTabTestContext()
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, ctx.Close()) })
	f := &tabDiagnosticsFixture{ctx: ctx}
	m, g := ctx.manager, ctx.globals
	f.rw, err = rewriter.NewTheoryRewriter(m, g.Context, g, nil)
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, f.rw.Close()) })
	coop, err := cooperate.NewCooperation(m.Budget())
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, coop.Close()) })
	f.lite, err = qe.NewLite(m, nil, false, qe.Config{Context: g.Context, Globals: g, Cooperation: coop})
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, f.lite.Close()) })
	f.querySolver, err = ctx.NewSimpleSolver()
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, f.querySolver.Close()) })
	f.subSolver, err = ctx.NewSimpleSolver()
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, f.subSolver.Close()) })
	rm, err := base.NewRuleManager(m)
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, rm.Close()) })
	f.rules, err = base.NewRuleSet(rm)
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() { tabDiagnosticsMust(t, f.rules.Close()) })
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		t.Cleanup(func() { tabDiagnosticsMust(t, m.DelRef(&ref)) })
		return h
	}
	zero, one := own(m.ArithmeticIntegerNumeralText("0")), own(m.ArithmeticIntegerNumeralText("1"))
	integer, err := m.GetSort(zero)
	tabDiagnosticsMust(t, err)
	boolean := own(m.MkBoolSort())
	declare := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		tabDiagnosticsMust(t, err)
		d := own(m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
		tabDiagnosticsMust(t, rm.RegisterPredicate(d))
		return d
	}
	p, q := declare("diagnostic-p"), declare("diagnostic-q")
	x := own(m.MkVar(0, integer))
	app := func(d, argument ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, []ast.ASTHandle{argument})) }
	for _, spec := range []struct {
		head ast.ASTHandle
		tail []ast.ASTHandle
	}{{app(p, x), []ast.ASTHandle{app(q, x)}}, {app(q, zero), nil}} {
		rule, err := rm.NewRule(spec.head, spec.tail, nil, "")
		tabDiagnosticsMust(t, err)
		tabDiagnosticsMust(t, f.rules.Add(rule))
		tabDiagnosticsMust(t, rule.Close())
	}
	f.queries = [2]ast.ASTHandle{app(p, zero), app(p, one)}
	return f
}

func tabCaptureVerbose(t *testing.T, level uint32, writer io.Writer) *utext.NumericTextStream {
	t.Helper()
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	tabDiagnosticsMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	tabDiagnosticsMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{BorrowedWriter: writer})
	tabDiagnosticsMust(t, err)
	t.Cleanup(func() {
		tabDiagnosticsMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		tabDiagnosticsMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		tabDiagnosticsMust(t, stream.Close())
	})
	tabDiagnosticsMust(t, diagnostic.SetProcessVerbosityLevel(level))
	tabDiagnosticsMust(t, diagnostic.SetProcessVerboseStream(stream))
	return stream
}

func TestTabQuerySelectionVerboseTranscriptAndState(t *testing.T) {
	for _, level := range []uint32{0, 1, 2} {
		for _, refuse := range []bool{false, true} {
			t.Run(fmt.Sprintf("level=%d/refuse=%v", level, refuse), func(t *testing.T) {
				f := newTabDiagnosticsFixture(t)
				var output bytes.Buffer
				var writer io.Writer = &output
				if refuse {
					writer = tabCertificateFailWriter{}
				}
				tabCaptureVerbose(t, level, writer)
				session, err := NewSession(f.ctx.manager, "first")
				tabDiagnosticsMust(t, err)
				defer func() { tabDiagnosticsMust(t, session.Close()) }()
				for i, query := range f.queries {
					var stats Statistics
					status, answer, err := QueryPredicateWithSession(f.rules, query, f.rw, f.lite, f.querySolver, f.subSolver, session, &stats)
					tabDiagnosticsMust(t, err)
					handle, err := f.ctx.manager.RefGet(answer)
					tabDiagnosticsMust(t, err)
					if i == 0 {
						fact, err := f.ctx.manager.GetProofFact(handle)
						tabDiagnosticsMust(t, err)
						if status != lbool.True || !f.ctx.manager.IsFalse(fact) || stats != (Statistics{Unfold: 2}) || session.nextSequence != 3 {
							t.Fatal("verbose changed reachable query", status, stats, session.nextSequence)
						}
					} else if status != lbool.False || !f.ctx.manager.IsTrue(handle) || stats != (Statistics{Unfold: 1, UnfoldFail: 1}) || session.nextSequence != 5 {
						t.Fatal("verbose changed unreachable query", status, stats, session.nextSequence)
					}
					tabDiagnosticsMust(t, f.ctx.manager.DelRef(&answer))
					for _, solver := range []solver.Solver{f.querySolver, f.subSolver} {
						depth, err := solver.ScopeLevel()
						tabDiagnosticsMust(t, err)
						if depth != 0 {
							t.Fatal("query left a solver scope", depth)
						}
					}
				}
				want := ""
				if !refuse {
					switch level {
					case 1:
						want = "g0 (diagnostic-p 0)\ng3 (diagnostic-p 1)\n"
					case 2:
						want = "g0 (diagnostic-p 0)\n(diagnostic-p 0)\n(diagnostic-q 0)\ntrue\ng3 (diagnostic-p 1)\n(diagnostic-p 1)\n(diagnostic-q 1)\n"
					}
				}
				if output.String() != want {
					t.Fatalf("source query/selection transcript\ngot  %q\nwant %q", output.String(), want)
				}
			})
		}
	}
}

func TestTabDisabledVerboseDoesNotEvaluateOperands(t *testing.T) {
	tabCaptureVerbose(t, 0, tabCertificateFailWriter{})
	var search *tabSearch
	tabDiagnosticsMust(t, search.verboseInitialClause())
	tabDiagnosticsMust(t, search.verboseSelectedPredicate(nil))
	tabDiagnosticsMust(t, search.verboseEmptyClause(nil))
}

func TestTabClauseDisplaySourceShapeAndOwnership(t *testing.T) {
	f := newTabDiagnosticsFixture(t)
	m := f.ctx.manager
	truth, err := m.MkTrue()
	tabDiagnosticsMust(t, err)
	falsehood, err := m.MkFalse()
	tabDiagnosticsMust(t, err)
	boolean, err := m.MkBoolSort()
	tabDiagnosticsMust(t, err)
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		tabDiagnosticsMust(t, err)
		ref, err := m.NewRef(h)
		tabDiagnosticsMust(t, err)
		deferRef := ref
		t.Cleanup(func() { tabDiagnosticsMust(t, m.DelRef(&deferRef)) })
		return h
	}
	constant := func(name string) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		tabDiagnosticsMust(t, err)
		return own(m.MkConst(symbol, boolean))
	}
	head, body := constant("display-head"), constant("display-body")
	declaration, err := m.GetDecl(f.queries[0])
	tabDiagnosticsMust(t, err)
	integer, err := m.GetDomain(declaration, 0)
	tabDiagnosticsMust(t, err)
	variable := own(m.MkVar(0, integer))
	variablePredicate := own(m.MkApp(declaration, []ast.ASTHandle{variable}))
	for _, test := range []struct {
		name string
		head ast.ASTHandle
		body []ast.ASTHandle
		want string
	}{
		{"goal", falsehood, []ast.ASTHandle{head, body}, "(and display-head display-body)\n"},
		{"fact", head, nil, "display-head\n"},
		{"rule", head, []ast.ASTHandle{body}, "(=> display-body display-head)\n"},
		{"unquantified-goal", falsehood, []ast.ASTHandle{variablePredicate}, "(diagnostic-p ?0)\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			clause, err := NewClause(m, f.rw, test.head, test.body, truth)
			tabDiagnosticsMust(t, err)
			defer func() { tabDiagnosticsMust(t, clause.Close()) }()
			before := m.Stats()
			var out bytes.Buffer
			tabDiagnosticsMust(t, clause.Display(&out))
			if out.String() != test.want {
				t.Fatalf("display %q, want %q", out.String(), test.want)
			}
			if err := clause.Display(tabCertificateFailWriter{}); err != io.ErrClosedPipe {
				t.Fatal("explicit display lost writer error", err)
			}
			after := m.Stats()
			if before.LiveReferences != after.LiveReferences || before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters {
				t.Fatal("display changed clause ownership", before, after)
			}
		})
	}
}
