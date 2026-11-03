package base_test

// Source: dl_context.cpp display_smt2 owns each generated query, declared
// variable and pretty-print document until its enclosing expression completes.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type contextSMT2AllocationFixture struct {
	manager, formats *ast.ASTManager
	set              *base.RuleSet
	config           base.ContextSMT2Config
}

func newContextSMT2AllocationFixture(t *testing.T, extensions bool) *contextSMT2AllocationFixture {
	return newContextSMT2AllocationFixtureWidth(t, extensions, 128)
}

func newContextSMT2AllocationFixtureWidth(t *testing.T, extensions bool, width int) *contextSMT2AllocationFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	ruleMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	ruleMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	set, err := base.NewRuleSet(rules)
	ruleMust(t, err)
	var roots []ast.ASTRef
	t.Cleanup(func() {
		budget.FailAfter(-1)
		ruleMust(t, set.Close())
		ruleMust(t, rules.Close())
		for i := len(roots) - 1; i >= 0; i-- {
			ruleMust(t, manager.DelRef(&roots[i]))
		}
		ruleMust(t, manager.Close())
		ruleMust(t, numbers.Close())
		ruleMust(t, symbols.Close())
		if budget.Used() != 0 {
			t.Fatal("context dump allocation fixture leaked bytes", budget.Used())
		}
	})
	own := func(h ast.ASTHandle, cause error) ast.ASTHandle {
		t.Helper()
		ruleMust(t, cause)
		ref, cause := manager.NewRef(h)
		ruleMust(t, cause)
		roots = append(roots, ref)
		return h
	}
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	// Initialize only the persistent format plugin sort. In particular, do not
	// warm a complete dump: its temporary reference pages are part of the test.
	_, err = manager.FormatSort()
	ruleMust(t, err)
	formats, err := manager.GetFormatManager()
	ruleMust(t, err)
	f := &contextSMT2AllocationFixture{manager: manager, formats: formats, set: set, config: base.DefaultContextSMT2Config()}
	f.config.FixedpointExtensions = extensions
	if !extensions {
		for i := 0; i < width; i++ {
			name, cause := symbols.Intern(fmt.Sprintf("allocation-query-%d", i))
			ruleMust(t, cause)
			declaration, cause := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			ruleMust(t, cause)
			ruleMust(t, rules.RegisterPredicate(declaration))
			f.config.Queries = append(f.config.Queries, own(manager.MkApp(declaration, nil)))
		}
		return f
	}
	domain, arguments, names := make([]ast.ASTHandle, width), make([]ast.ASTHandle, width), make([]symbol.Symbol, width)
	for i := range arguments {
		domain[i] = boolean
		arguments[i] = own(manager.MkVar(uint32(i), boolean))
		names[i], err = symbols.Intern(fmt.Sprintf("input-%d", i))
		ruleMust(t, err)
	}
	name, err := symbols.Intern("allocation-wide")
	ruleMust(t, err)
	declaration, err := manager.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, rules.RegisterPredicate(declaration))
	body := own(manager.MkApp(declaration, arguments))
	formula := own(manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: domain, Names: names, Body: body}))
	f.config.PendingRules = []ast.ASTHandle{formula}
	f.config.PendingNames = []string{"wide-rule"}
	// This permutation requires the source fresh query bridge, including a
	// separate owning slot for every free variable and its new declaration.
	arguments[0], arguments[1] = arguments[1], arguments[0]
	f.config.Queries = []ast.ASTHandle{own(manager.MkApp(declaration, arguments))}
	return f
}

func contextSMT2LiveCounts(stats ast.ASTManagerStats) [4]uint32 {
	return [4]uint32{stats.LiveNodes, stats.LiveInfos, stats.LiveParameters, stats.LiveReferences}
}

func TestContextSMT2AllocationExactASTOwnership(t *testing.T) {
	for _, extensions := range []bool{false, true} {
		t.Run(fmt.Sprintf("extensions=%t", extensions), func(t *testing.T) {
			width := 128
			if ast.ASTSourceDebug && !extensions {
				// Debug AST allocation bypasses the source small-object pool.
				// Exhaust every allocation with the smallest query vector that
				// grows the 64-slot main reference page: 44 input roots plus 22
				// query copies and one temporary negation root. The separate
				// debug test preserves the cold 128-query page boundaries.
				width = 22
			}
			failures, completed := 0, false
			for allowance := int64(0); allowance < 4096 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					completed = checkContextSMT2Allocation(t, extensions, width, allowance, false)
					if !completed {
						failures++
					}
				}) {
					return
				}
			}
			if !completed || failures == 0 {
				t.Fatal("context dump allocation campaign missed failure or completion", failures, completed)
			}
		})
	}
}

// checkContextSMT2Allocation always uses a cold fixture and verifies the same
// complete owner, receipt and successful-retry contract for every profile.
func checkContextSMT2Allocation(t *testing.T, extensions bool, width int, allowance int64, atNegation bool) bool {
	t.Helper()
	f := newContextSMT2AllocationFixtureWidth(t, extensions, width)
	before, beforeFormat := contextSMT2LiveCounts(f.manager.Stats()), contextSMT2LiveCounts(f.formats.Stats())
	var output bytes.Buffer
	var out io.Writer = &output
	boundary := &contextSMT2NegationAllocationWriter{output: &output, budget: f.manager.Budget()}
	if atNegation {
		out = boundary
	}
	f.manager.Budget().FailAfter(allowance)
	failure := f.set.WriteContextSMT2(out, f.config)
	f.manager.Budget().FailAfter(-1)
	if atNegation && (!boundary.armed || failure == nil || strings.Contains(output.String(), "(check-sat)")) {
		t.Fatal("missed the first negated query owner allocation", boundary.armed, failure)
	}
	if failure != nil {
		var receipt memory.ConstructionFailure
		if errors.As(failure, &receipt) {
			shown, exposed := receipt.Error(), receipt.Unwrap()
			ruleMust(t, receipt.Cleanup())
			ruleMust(t, receipt.Cleanup())
			if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil {
				t.Fatal("context dump cleanup changed its receipt")
			}
		}
		var resource memory.Failure
		if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
			t.Fatal("unexpected context dump allocation failure", failure)
		}
	}
	if after := contextSMT2LiveCounts(f.manager.Stats()); after != before {
		t.Fatal("context dump lost main AST ownership", before, after, failure)
	}
	if after := contextSMT2LiveCounts(f.formats.Stats()); after != beforeFormat {
		t.Fatal("context dump lost format AST ownership", beforeFormat, after, failure)
	}
	output.Reset()
	ruleMust(t, f.set.WriteContextSMT2(&output, f.config))
	if extensions {
		if strings.Count(output.String(), "(declare-var ") != width || strings.Count(output.String(), "(query q!") != 1 {
			t.Fatal("retry missed wide declarations or fresh query bridge", output.String())
		}
	} else if strings.Count(output.String(), "(check-sat)") != width {
		t.Fatal("retry missed plain HORN negated queries", output.String())
	}
	if after := contextSMT2LiveCounts(f.manager.Stats()); after != before {
		t.Fatal("context dump retry lost main AST ownership", before, after)
	}
	if after := contextSMT2LiveCounts(f.formats.Stats()); after != beforeFormat {
		t.Fatal("context dump retry lost format AST ownership", beforeFormat, after)
	}
	return failure == nil
}

type contextSMT2NegationAllocationWriter struct {
	output *bytes.Buffer
	budget *arena.Budget
	armed  bool
}

func (w *contextSMT2NegationAllocationWriter) Write(body []byte) (int, error) {
	n, err := w.output.Write(body)
	if err == nil && !w.armed && string(body) == "(assert " {
		// Source display_smt2 reserves the negated query's owner immediately
		// after this actual stream insertion, before constructing its AST.
		w.armed = true
		w.budget.FailAfter(0)
	}
	return n, err
}
