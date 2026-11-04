//go:build g3trace

package pdr

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type ruleTraceWriter struct {
	bytes.Buffer
	refuse         bool
	failAllocation bool
	manager        *ast.ASTManager
}

func (w *ruleTraceWriter) Write(p []byte) (int, error) {
	if w.refuse {
		if w.failAllocation {
			w.failAllocation = false
			w.manager.Budget().FailAfter(0)
		}
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(p)
}

func TestPDRRuleTraceOrderGatingAndNonGroundFailure(t *testing.T) {
	for _, mode := range []string{"disabled", "enabled", "refused", "refused-allocate", "nonground", "variable", "flatten"} {
		t.Run(mode, func(t *testing.T) {
			baseManager, spare, old, _, closeBase := closureFixture(t)
			globals := spare.contexts.config.Globals
			writer := &ruleTraceWriter{refuse: mode == "refused" || mode == "refused-allocate", failAllocation: mode == "refused-allocate"}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: baseManager.Symbols, Numerals: baseManager.NumeralService(), Budget: baseManager.Budget(), TraceTags: tags, Params: globals.Context, Globals: globals})
			closureMust(t, err)
			writer.manager = m
			closureMust(t, m.RegisterDeclarationPlugins())
			owner, err := NewManager(m, nil, 2, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			closureMust(t, err)
			boolean, err := m.MkBoolSort()
			closureMust(t, err)
			name, err := m.Symbols.Intern("rule-trace-head")
			closureMust(t, err)
			var domain []ast.ASTHandle
			if mode == "variable" {
				domain = []ast.ASTHandle{boolean}
			}
			head, err := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
			closureMust(t, err)
			manager, err := base.NewRuleManager(m)
			closureMust(t, err)
			closureMust(t, manager.RegisterPredicate(head))
			var headArgs []ast.ASTHandle
			if mode == "variable" {
				v, x := m.MkVar(0, boolean)
				closureMust(t, x)
				headArgs = append(headArgs, v)
			}
			headApp, err := m.MkApp(head, headArgs)
			closureMust(t, err)
			name, err = m.Symbols.Intern("rule-trace-tail")
			closureMust(t, err)
			tailDecl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			closureMust(t, err)
			tail, err := m.MkApp(tailDecl, nil)
			closureMust(t, err)
			if mode == "variable" {
				tail, err = m.MkNot(headArgs[0])
				closureMust(t, err)
			}
			if mode == "flatten" {
				otherName, x := m.Symbols.Intern("rule-trace-other")
				closureMust(t, x)
				otherDecl, x := m.MkFuncDecl(otherName, nil, boolean, ast.ASTFuncDeclInfo{})
				closureMust(t, x)
				other, x := m.MkApp(otherDecl, nil)
				closureMust(t, x)
				disjunction, x := m.MkOr(tail, other)
				closureMust(t, x)
				tail, x = m.MkNot(disjunction)
				closureMust(t, x)
			}
			if mode == "nonground" {
				truth, x := m.MkTrue()
				closureMust(t, x)
				quantified, x := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: truth})
				closureMust(t, x)
				tail, x = m.MkEq(quantified, truth)
				closureMust(t, x)
			}
			rule, err := manager.NewRule(headApp, []ast.ASTHandle{tail}, nil, "rule-trace")
			closureMust(t, err)
			transformer, err := NewPredicateTransformer(owner, head, ReachableNoCache)
			closureMust(t, err)
			if mode != "disabled" {
				tags.Enable("pdr")
			}
			initErr := transformer.InitializeRules(map[ast.ASTHandle]*PredicateTransformer{head: transformer}, []*base.Rule{rule})
			m.Budget().FailAfter(-1)
			if mode == "refused-allocate" {
				if !errors.Is(initErr, arena.ErrInjectedFailure) {
					t.Fatal("trace did not evaluate the first expression after output refusal", initErr)
				}
			} else if mode == "nonground" {
				if initErr == nil || !strings.Contains(initErr.Error(), "PDR cannot solve non-ground tails:") {
					t.Fatal("missing non-ground rejection", initErr)
				}
			} else {
				closureMust(t, initErr)
			}
			text := writer.String()
			if mode == "disabled" && text != "" {
				t.Fatal("disabled rule trace emitted output", text)
			}
			if mode == "enabled" {
				a, b, c := strings.Index(text, "src/muz/pdr/pdr_context.cpp:602"), strings.Index(text, "src/muz/pdr/pdr_context.cpp:616"), strings.Index(text, "src/muz/pdr/pdr_context.cpp:519")
				if a < 0 || b <= a || c <= b || strings.Count(text, "rule-trace-tail") < 3 {
					t.Fatal("rule trace source ordering or content", text)
				}
			}
			if mode == "variable" {
				a, b := strings.Index(text, "src/muz/pdr/pdr_context.cpp:602"), strings.Index(text, "src/muz/pdr/pdr_context.cpp:616")
				if a < 0 || b <= a {
					t.Fatal("missing variable trace", text)
				}
				part := text[a:b]
				original, grounded := strings.Index(part, "(:var 0)"), strings.Index(part, "rule-trace-head")
				if original < 0 || grounded <= original {
					t.Fatal("original and grounded trace order", part)
				}
			}
			if mode == "flatten" {
				if strings.Count(text, "src/muz/pdr/pdr_context.cpp:602") != 2 || !strings.Contains(text, "(not rule-trace-tail)") || !strings.Contains(text, "(not rule-trace-other)") {
					t.Fatal("flattened tail traces missing", text)
				}
			}
			if mode == "nonground" {
				if !strings.Contains(text, "src/muz/pdr/pdr_context.cpp:602") || !strings.Contains(text, "(forall") || strings.Contains(text, "src/muz/pdr/pdr_context.cpp:616") {
					t.Fatal("non-ground trace did not precede rejection", text)
				}
			}
			closureMust(t, transformer.Close())
			closureMust(t, rule.Close())
			closureMust(t, manager.Close())
			closureMust(t, owner.Close())
			closureMust(t, m.Close())
			closureMust(t, old.Close())
			closureMust(t, spare.Close())
			closeBase()
		})
	}
}
