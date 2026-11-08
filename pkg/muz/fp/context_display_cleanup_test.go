package fp

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func contextDisplayLiveCounts(stats ast.ASTManagerStats) [4]uint64 {
	return [4]uint64{uint64(stats.LiveNodes), uint64(stats.LiveInfos), uint64(stats.LiveParameters), uint64(stats.LiveReferences)}
}

type contextDisplayCleanupFixture struct {
	t       *testing.T
	m       *ast.ASTManager
	e       *PropositionalEngine
	runtime *memory.Runtime
	numbers *numeral.RationalService
	value   numeral.OwnedRational
	roots   []ast.ASTRef
	pending func()
	called  bool
}

func newContextDisplayCleanupFixture(t *testing.T) *contextDisplayCleanupFixture {
	t.Helper()
	f := &contextDisplayCleanupFixture{t: t}
	budget := arena.NewBudget(1 << 27)
	var err error
	f.runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if f.pending != nil && !f.called {
			f.called = true
			f.pending()
		}
		return failure
	}})
	fragmentMust(t, err)
	f.numbers, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: f.runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	fragmentMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	fragmentMust(t, err)
	f.m, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: f.numbers, Budget: budget})
	fragmentMust(t, err)
	fragmentMust(t, f.m.RegisterDeclarationPlugins())
	f.e, err = NewPropositionalEngine(f.m)
	fragmentMust(t, err)
	f.value, err = f.numbers.NewValue()
	fragmentMust(t, err)
	fragmentMust(t, f.numbers.SetString(&f.value, "18446744073709551617/18446744073709551619"))
	t.Cleanup(func() {
		fragmentMust(t, f.e.Close())
		fragmentMust(t, closeASTRefs(f.m, f.roots))
		fragmentMust(t, f.numbers.Del(&f.value))
		fragmentMust(t, f.m.Close())
		fragmentMust(t, f.numbers.Close())
		fragmentMust(t, symbols.Close())
		fragmentMust(t, f.runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("context display cleanup leaked native bytes", budget.Used())
		}
	})
	boolean := f.own(f.m.MkBoolSort())
	truth := f.own(f.m.MkTrue())
	x := f.own(f.m.MkVar(0, boolean))
	declare := func(name string) ast.ASTHandle {
		s, err := symbols.Intern(name)
		fragmentMust(t, err)
		d := f.own(f.m.MkFuncDecl(s, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
		fragmentMust(t, f.e.RegisterPredicate(d))
		return d
	}
	p, q, r := declare("display-P"), declare("display-Q"), declare("display-R")
	app := func(d, v ast.ASTHandle) ast.ASTHandle { return f.own(f.m.MkApp(d, []ast.ASTHandle{v})) }
	fragmentMust(t, f.e.addRuleEager(app(p, truth), "closed-prefix"))
	f.e.flushRawDisplayRules()
	fragmentMust(t, f.e.addRuleEager(f.own(f.m.MkImplies(app(p, x), app(q, x))), "open-output"))
	fragmentMust(t, f.e.set.SetOutputPredicate(q))
	fragmentMust(t, f.e.addRuleEager(app(r, truth), "closed-pending"))
	// Stabilize source open-rule promotion before capturing exact ownership
	// counts. A closed pending formula still forces a display-only snapshot.
	fragmentMust(t, f.e.normalizePendingOpenRules())
	return f
}

func (f *contextDisplayCleanupFixture) own(value ast.ASTHandle, cause error) ast.ASTHandle {
	f.t.Helper()
	fragmentMust(f.t, cause)
	ref, err := f.m.NewRef(value)
	fragmentMust(f.t, err)
	f.roots = append(f.roots, ref)
	return value
}

func (f *contextDisplayCleanupFixture) refuse(cleanup func()) {
	f.t.Helper()
	f.pending, f.called = cleanup, false
	fragmentMust(f.t, f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount)+1))
	var invocation ast.ASTArithmeticNumeralInvocation
	_, cause := f.m.ArithmeticNumeralWithInvocation(&f.value, false, &invocation)
	f.pending = nil
	fragmentMust(f.t, f.runtime.SetMaxCount(0))
	fragmentMust(f.t, f.m.CleanupArithmeticNumeralInvocation(&invocation))
	if cause == nil || !f.called {
		f.t.Fatal("context display fixture missed the native cleanup refusal", cause, f.called)
	}
}

type contextDisplayErrorWriter struct{ cause error }

func (w contextDisplayErrorWriter) Write([]byte) (int, error) { return 0, w.cause }

func TestContextDisplayPublicWriterErrorPreservesRuleOwners(t *testing.T) {
	f := newContextDisplayCleanupFixture(t)
	var baseline bytes.Buffer
	fragmentMust(t, f.e.WriteContextSMT2(&baseline, nil, nil))
	// Source display creates a fresh nullary name for an expression query on
	// each call. Compare its structure independently of that increasing ID.
	freshQuery := regexp.MustCompile(`q!![0-9]+`)
	formulas, err := f.e.RuleFormulas()
	fragmentMust(t, err)
	fragmentMust(t, closeASTRefs(f.m, formulas))
	before := contextDisplayLiveCounts(f.m.Stats())
	original := errors.New("context display writer failed")
	for retry := 0; retry < 2; retry++ {
		cause := f.e.WriteContextSMT2(contextDisplayErrorWriter{original}, nil, nil)
		if !errors.Is(cause, original) {
			t.Fatal("public display lost the writer error", cause)
		}
		var actual bytes.Buffer
		fragmentMust(t, f.e.WriteContextSMT2(&actual, nil, nil))
		if freshQuery.ReplaceAllString(actual.String(), "q!!N") != freshQuery.ReplaceAllString(baseline.String(), "q!!N") {
			t.Fatal("display retry changed source rules", actual.String(), baseline.String())
		}
		formulas, err = f.e.RuleFormulas()
		fragmentMust(t, err)
		if len(formulas) != 3 {
			t.Fatal("display error lost normalized, pending, or query formula", len(formulas))
		}
		fragmentMust(t, closeASTRefs(f.m, formulas))
		if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
			t.Fatal("public display retry leaked AST owners", before, after)
		}
	}
}

func TestContextDisplayOwnersSurviveNativeCleanupRefusal(t *testing.T) {
	for _, withNested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested=%v", withNested), func(t *testing.T) {
			f := newContextDisplayCleanupFixture(t)
			var beforeText bytes.Buffer
			fragmentMust(t, f.e.WriteContextSMT2(&beforeText, nil, nil))
			warm, err := f.e.RuleFormulas()
			fragmentMust(t, err)
			fragmentMust(t, closeASTRefs(f.m, warm))
			before := contextDisplayLiveCounts(f.m.Stats())
			persistent := f.e.set
			snapshot, pending, names, err := f.e.contextFormulaRules()
			fragmentMust(t, err)
			if snapshot == persistent || len(pending) != 1 || len(names) != 1 {
				t.Fatal("native cleanup fixture did not reach a pending-rule snapshot")
			}
			borrowed, err := snapshot.Rule(0)
			fragmentMust(t, err)
			attempt := &contextFormulaAttempt{manager: f.m, sets: []*base.RuleSet{snapshot}, rules: []*base.Rule{borrowed}}
			var formulas, queries []ast.ASTRef
			var displayNames []string
			fragmentMust(t, snapshot.AppendDisplayFormulas(&formulas, &queries, &displayNames))
			if len(formulas) != 1 || len(queries) != 1 {
				t.Fatal("display finalizer fixture missed its rule/query roots", len(formulas), len(queries))
			}
			attempt.references = append(attempt.references, formulas...)
			attempt.references = append(attempt.references, ast.ASTRef{})
			attempt.references = append(attempt.references, queries...)
			closed, err := f.m.NewRef(pending[0])
			fragmentMust(t, err)
			attempt.references = append(attempt.references, closed, ast.ASTRef{})
			var nestedRefs []ast.ASTRef
			if withNested {
				extra, err := f.m.CopyRef(formulas[0])
				fragmentMust(t, err)
				nestedRefs = []ast.ASTRef{extra}
			}
			original := errors.New("context display construction failed")
			var receipt memory.ConstructionFailure
			var child *queryReferenceCleanupFailure
			f.refuse(func() {
				cause := original
				if withNested {
					nested := closeASTRefs(f.m, nestedRefs)
					nestedRefs = nil
					if !errors.As(nested, &child) {
						t.Fatal("display nested temporary owner disappeared", nested)
					}
					cause = errors.Join(cause, nested)
				}
				failure := attempt.finish(cause)
				if !errors.Is(failure, original) || !errors.Is(failure, ast.ErrASTPending) || !errors.As(failure, &receipt) {
					t.Fatal("context display finalizer lost its operation or cleanup owner", failure)
				}
				if receipt.RuntimeOwner() != f.runtime || receipt.CleanupCause() == nil {
					t.Fatal("context display finalizer dropped a concrete ownership frontier")
				}
				if withNested {
					if attempt.nested != child || len(attempt.sets) != 1 || len(attempt.rules) != 1 || len(attempt.references) != 5 {
						t.Fatal("display outer cleanup crossed a refused nested owner")
					}
				} else if len(attempt.sets) != 1 || len(attempt.rules) != 0 || len(attempt.references) != 5 {
					// Shared rule copies decrement their wrapper counts,
					// but the snapshot's predicate roots encounter the guard
					// before the later standalone formula references.
					t.Fatal("display cleanup did not preserve its remaining root frontier", len(attempt.sets), len(attempt.rules), len(attempt.references))
				}
				for retry := 0; retry < 2; retry++ {
					if err := receipt.Cleanup(); !errors.Is(err, ast.ErrASTPending) {
						t.Fatal("display cleanup retry crossed the active native guard", err)
					}
				}
			})
			if receipt == nil {
				t.Fatal("native display refusal did not publish a retryable owner")
			}
			origin, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
			fragmentMust(t, receipt.Cleanup())
			fragmentMust(t, receipt.Cleanup())
			if receipt.CleanupCause() != nil || receipt.RuntimeOwner() != f.runtime || receipt.OriginalCause() != origin || errors.Unwrap(receipt) != exposed || receipt.Error() != message {
				t.Fatal("display cleanup completion changed its stable failure receipt")
			}
			if len(attempt.sets) != 0 || len(attempt.rules) != 0 || len(attempt.references) != 0 || attempt.nested != nil || withNested && len(child.references) != 0 {
				t.Fatal("display cleanup completion left an ownership frontier")
			}
			if f.e.set != persistent {
				t.Fatal("display snapshot cleanup replaced the engine's persistent rule set")
			}
			if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
				t.Fatal("display native cleanup retry leaked AST owners", before, after)
			}
			var afterText bytes.Buffer
			fragmentMust(t, f.e.WriteContextSMT2(&afterText, nil, nil))
			freshQuery := regexp.MustCompile(`q!![0-9]+`)
			if freshQuery.ReplaceAllString(beforeText.String(), "q!!N") != freshQuery.ReplaceAllString(afterText.String(), "q!!N") {
				t.Fatal("public display retry changed the source context")
			}
			formulas, err = f.e.RuleFormulas()
			fragmentMust(t, err)
			if len(formulas) != 3 {
				t.Fatal("public formula retry lost normalized/pending/query entries", len(formulas))
			}
			fragmentMust(t, closeASTRefs(f.m, formulas))
			if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
				t.Fatal("public display/formula retry leaked AST owners", before, after)
			}
		})
	}
}
