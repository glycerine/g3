//go:build g3trace

package fp

// Sources: dl_rule.cpp:267-301 drops the existential query prefix;
// pdr_dl_interface.cpp:92-101 traces the original input; ast.cpp:1781-1844
// destroys its last numeric annotation when that lexical input owner ends.

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type expressionQueryNativeDeletionWriter struct {
	cleanup                  ruleUpdateDERCleanupWriter
	engine                   *PropositionalEngine
	id                       string
	fail                     bool
	deleting, selected, seen bool
	completed                bool
}

func (w *expressionQueryNativeDeletionWriter) Write(p []byte) (int, error) {
	text := string(p)
	if text == "del_ast: " {
		w.deleting = true
	} else if w.deleting {
		w.selected = text == w.id
		w.deleting = false
	}
	if w.selected && !w.seen && text == "------------------------------------------------\n" {
		w.seen = true
		// The writer runs inside AST deletion. Read only engine fields here;
		// another AST operation would reenter the manager's locked scope.
		w.completed = w.engine.queryClone != nil && w.engine.queryReady && w.engine.lastResult == lbool.True
		if w.fail {
			w.cleanup.boundary.seen = true
			// This writer succeeds. The following numeric destructor reaches
			// the real native accounting error armed by the existing helper.
			return w.cleanup.boundary.Write(p)
		}
	}
	return len(p), nil
}

type expressionQueryNativeDeletionCounts struct {
	attempt [4]uint64
	resumed [4]uint64
	closed  [4]uint64
}

func TestExpressionQueryResumesConsumedNativeDeletion(t *testing.T) {
	for _, completion := range []string{"query-retry", "engine-close"} {
		t.Run(completion, func(t *testing.T) {
			var counts [2]expressionQueryNativeDeletionCounts
			for index, fail := range []bool{false, true} {
				name := "healthy"
				if fail {
					name = "native-failure"
				}
				if !t.Run(name, func(t *testing.T) {
					counts[index] = checkExpressionQueryNativeDeletion(t, completion, fail)
				}) {
					return
				}
			}
			if counts[0].attempt[3] != counts[1].attempt[3] {
				t.Fatal("consumed lexical deletion lost an AST reference", counts[0].attempt, counts[1].attempt)
			}
			if counts[0].resumed != counts[1].resumed {
				t.Fatal("public completion retained interrupted AST node, info or parameter storage", counts[0].resumed, counts[1].resumed)
			}
			if counts[0].closed != counts[1].closed {
				t.Fatal("engine close retained interrupted numeric AST ownership", counts[0].closed, counts[1].closed)
			}
		})
	}
}

func checkExpressionQueryNativeDeletion(t *testing.T, completion string, fail bool) (counts expressionQueryNativeDeletionCounts) {
	t.Helper()
	f := newRuleUpdateNumeralReadFixture(t)
	fragmentMust(t, f.e.SelectEngine("pdr"))
	for _, key := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, f.e.globals.SourceSetName(key, "false"))
	}
	boolean := f.own(f.m.MkBoolSort())
	atom := f.own(f.m.MkFreshConst("native-deletion-P", boolean))
	declaration, err := f.m.GetDecl(atom)
	fragmentMust(t, err)
	fragmentMust(t, f.e.RegisterPredicate(declaration))
	fragmentMust(t, f.e.AddRule(atom, "fact"))
	wide, err := f.m.ArithmeticRealNumeralText(strings.Repeat("9", 600))
	fragmentMust(t, err)
	numeralDeclaration, err := f.m.GetDecl(wide)
	fragmentMust(t, err)
	id, err := f.m.GetASTID(numeralDeclaration)
	fragmentMust(t, err)
	query, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)},
		Body: atom, NoPatterns: []ast.ASTHandle{wide},
	})
	fragmentMust(t, err)
	// The input is intentionally raw: originalTraceQuery is its only pin.
	// Hoisting removes the unused binder and annotation, so neither the
	// normalized query nor the backend can retain this numeric payload.
	for _, item := range []struct {
		handle ast.ASTHandle
		refs   uint32
	}{{query, 0}, {wide, 1}, {numeralDeclaration, 1}} {
		refs, err := f.m.GetRefCount(item.handle)
		fragmentMust(t, err)
		if refs != item.refs {
			t.Fatal("fixture does not isolate the lexical numeric owner", refs, item.refs)
		}
	}
	writer := &expressionQueryNativeDeletionWriter{
		engine: f.e, fail: fail, id: strconv.FormatUint(uint64(id), 10),
		cleanup: ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}},
	}
	repaired := false
	defer func() {
		if !repaired {
			writer.cleanup.restoreAccounting(t, f.runtime)
		}
	}()
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("pdr")
	tags.Enable("mk_var_bug")
	t.Cleanup(tags.Finalize)
	var status lbool.LBool
	var cause error
	func() {
		process := diagnostic.BorrowProcessTraceTags()
		saved := *process
		*process = *tags
		defer func() { *process = saved }()
		status, cause = f.e.QueryExpression(query)
	}()
	counts.attempt = contextDisplayLiveCounts(f.m.Stats())
	if !writer.seen || !writer.completed || status != lbool.True {
		t.Fatal("fixture missed the final original-query deletion after solving", status, cause, writer.seen, writer.completed)
	}
	previous := f.e.queryClone
	message := ""
	if fail {
		if !writer.cleanup.boundary.interrupted || !errors.Is(cause, arena.ErrBounds) {
			t.Fatal("query missed the native numeric destructor refusal", cause)
		}
		message = cause.Error()
		last, err := f.e.LastResult()
		if last != lbool.Undef || !errors.Is(err, ErrPropositionalEngine) {
			t.Fatal("lexical cleanup failure published the solved result", last, err)
		}
		answer, err := f.e.ExpressionAnswer()
		if answer != (ast.ASTRef{}) {
			f.roots = append(f.roots, answer)
		}
		if answer != (ast.ASTRef{}) || !errors.Is(err, ErrPropositionalEngine) {
			t.Fatal("lexical cleanup failure published an answer", answer, err)
		}
	} else {
		fragmentMust(t, cause)
		last, err := f.e.LastResult()
		fragmentMust(t, err)
		if last != lbool.True {
			t.Fatal("healthy query did not publish its result", last)
		}
	}
	// Repair accounting only. No private cleanup or error receipt may drain
	// the interrupted AST deletion before the selected public operation.
	writer.cleanup.restoreAccounting(t, f.runtime)
	repaired = true
	if completion == "query-retry" {
		status, err = f.e.QueryExpression(atom)
		fragmentMust(t, err)
		if status != lbool.True || f.e.queryClone == nil || f.e.queryClone == previous || !previous.closed {
			t.Fatal("public retry did not replace the completed query owner", status)
		}
		last, err := f.e.LastResult()
		fragmentMust(t, err)
		if last != lbool.True {
			t.Fatal("public retry did not publish a healthy result", last)
		}
	} else {
		fragmentMust(t, f.e.Close())
		if !previous.closed {
			t.Fatal("engine close left the completed query owner open")
		}
	}
	counts.resumed = contextDisplayLiveCounts(f.m.Stats())
	fragmentMust(t, f.e.Close())
	counts.closed = contextDisplayLiveCounts(f.m.Stats())
	fragmentMust(t, f.e.Close())
	if after := contextDisplayLiveCounts(f.m.Stats()); after != counts.closed {
		t.Fatal("repeated engine close changed completed deletion ownership", counts.closed, after)
	}
	if fail && (cause.Error() != message || !errors.Is(cause, arena.ErrBounds)) {
		t.Fatal("public cleanup changed the original numeric deletion failure", cause)
	}
	// These counts precede manager disposal. The shared fixture separately
	// closes all caller roots and services and requires a zero final budget.
	return counts
}
