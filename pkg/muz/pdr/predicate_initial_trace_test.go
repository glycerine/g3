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
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type initialTraceWriter struct {
	bytes.Buffer
	transformer             *PredicateTransformer
	seen, beforeCache, fail bool
	failAllocation          bool
}

func (w *initialTraceWriter) Write(p []byte) (int, error) {
	if w.fail && w.failAllocation {
		w.transformer.manager.Budget().FailAfter(0)
		w.failAllocation = false
	}
	if strings.Contains(string(p), "Initial state: ") {
		w.seen = true
		w.beforeCache = w.transformer != nil && w.transformer.reachable.connector == (ast.ASTRef{}) && !w.transformer.initialized
		if w.failAllocation {
			w.transformer.manager.Budget().FailAfter(0)
		}
	}
	if w.fail {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(p)
}

func TestPDRInitialSystemTraceOrderingAndFailure(t *testing.T) {
	for _, mode := range []string{"disabled", "enabled", "refused", "allocate", "refused-allocate"} {
		t.Run(mode, func(t *testing.T) {
			base, spare, old, _, closeBase := closureFixture(t)
			globals := spare.contexts.config.Globals
			writer := &initialTraceWriter{}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags, Params: globals.Context, Globals: globals})
			closureMust(t, err)
			closureMust(t, manager.RegisterDeclarationPlugins())
			owner, err := NewManager(manager, nil, 3, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			closureMust(t, err)
			boolean, err := manager.MkBoolSort()
			closureMust(t, err)
			name, err := manager.Symbols.Intern("trace-initial-head")
			closureMust(t, err)
			head, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			closureMust(t, err)
			transformer, err := NewPredicateTransformer(owner, head, ReachableConstraintCache)
			closureMust(t, err)
			writer.transformer = transformer
			name, err = manager.Symbols.Intern("trace-initial-p")
			closureMust(t, err)
			decl, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			closureMust(t, err)
			initial, err := manager.MkApp(decl, nil)
			closureMust(t, err)
			transition, err := manager.MkNot(initial)
			closureMust(t, err)
			if mode != "disabled" {
				tags.Enable("pdr")
			}
			if mode == "refused" || mode == "refused-allocate" {
				writer.fail = true
			}
			if mode == "allocate" || mode == "refused-allocate" {
				writer.failAllocation = true
			}
			initErr := transformer.InitializeSystem(initial, transition)
			manager.Budget().FailAfter(-1)
			if mode == "allocate" || mode == "refused-allocate" {
				if mode == "allocate" && !writer.seen || !errors.Is(initErr, arena.ErrInjectedFailure) || transformer.initialized || transformer.reachable.connector != (ast.ASTRef{}) {
					t.Fatal("trace computation failure crossed cache initialization", initErr, writer.seen)
				}
			} else {
				closureMust(t, initErr)
				if transformer.reachable.connector == (ast.ASTRef{}) {
					t.Fatal("reachable initial state not installed")
				}
			}
			if mode == "disabled" {
				if writer.Len() != 0 || writer.seen {
					t.Fatal("disabled trace evaluated")
				}
			} else if mode == "enabled" {
				text := writer.String()
				if !writer.seen || !writer.beforeCache || !strings.Contains(text, "src/muz/pdr/pdr_context.cpp:519") || !strings.Contains(text, "Initial state: trace-initial-p\nTransition:    (not trace-initial-p)\n") {
					t.Fatal("unexpected initial trace or ordering", text, writer.beforeCache)
				}
			}
			closureMust(t, transformer.Close())
			closureMust(t, owner.Close())
			closureMust(t, manager.Close())
			closureMust(t, old.Close())
			closureMust(t, spare.Close())
			closeBase()
		})
	}
}
