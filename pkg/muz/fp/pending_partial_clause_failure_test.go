package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Arm the real allocator only after source flush has removed the eager copy
// and published the first normalized clause. The trace stream is a real AST
// output boundary; quantifier elimination for the second clause writes to it.
type pendingPartialFailureWriter struct {
	set             *base.RuleSet
	manager         *ast.ASTManager
	sawEmpty, armed bool
}

func (w *pendingPartialFailureWriter) Write(p []byte) (int, error) {
	n, err := w.set.Len()
	if err == nil && n == 0 {
		w.sawEmpty = true
	}
	if err == nil && n > 0 && w.sawEmpty && !w.armed {
		w.armed = true
		w.manager.TraceStream = nil
		w.manager.Budget().FailAfter(0)
	}
	return len(p), nil
}

func TestPendingFlushPublishesPartialMultiClauseInputOnFailure(t *testing.T) {
	for _, proof := range []bool{false, true} {
		t.Run(fmt.Sprintf("proof=%t", proof), func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			fragmentMust(t, globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "false"))
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.Close()) }()
			var refs []ast.ASTRef
			defer func() {
				for _, ref := range refs {
					fragmentMust(t, m.DelRef(&ref))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				fragmentMust(t, err)
				ref, err := m.NewRef(h)
				fragmentMust(t, err)
				refs = append(refs, ref)
				return h
			}
			boolean := own(m.MkBoolSort())
			x, y := own(m.MkVar(0, boolean)), own(m.MkVar(1, boolean))
			name, err := m.Symbols.Intern("partial-flush-head")
			fragmentMust(t, err)
			pred := own(m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
			fragmentMust(t, e.RegisterPredicate(pred))
			head := own(m.MkApp(pred, []ast.ASTHandle{x}))
			var tails []ast.ASTHandle
			for i := 0; i < 2; i++ {
				name, err := m.Symbols.Intern(fmt.Sprintf("partial-flush-opaque-%d", i))
				fragmentMust(t, err)
				decl := own(m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
				tails = append(tails, own(m.MkApp(decl, []ast.ASTHandle{y})))
			}
			body := own(m.MkOrArray(tails))
			implication := own(m.MkImplies(body, head))
			formula := own(m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{boolean, boolean},
				Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: implication,
			}))
			fragmentMust(t, e.AddRule(formula, "partial-flush"))
			fragmentMust(t, globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "true"))
			fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", fmt.Sprint(proof)))
			w := &pendingPartialFailureWriter{set: e.set, manager: m}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget(), BorrowedWriter: w})
			fragmentMust(t, err)
			defer func() { fragmentMust(t, stream.Close()) }()
			m.TraceStream = stream
			err = e.flushPendingRules()
			m.TraceStream = nil
			m.Budget().FailAfter(-1)
			if !w.armed || !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatalf("flush failure = %v, writer armed = %t", err, w.armed)
			}
			count, err := e.set.Len()
			fragmentMust(t, err)
			if count != 1 || len(e.rawRules) != 1 || !e.rawRules[0].pending {
				t.Fatalf("after failure: %d published clauses, raw rules %+v", count, e.rawRules)
			}
			first, err := e.set.Rule(0)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, first.Close()) }()
			fragmentMust(t, e.flushPendingRules())
			count, err = e.set.Len()
			fragmentMust(t, err)
			if count != 3 || e.rawRules[0].pending {
				t.Fatalf("after retry: %d published clauses, raw rules %+v", count, e.rawRules)
			}
			published, err := e.set.Rule(0)
			fragmentMust(t, err)
			same, err := first.Same(published)
			fragmentMust(t, err)
			fragmentMust(t, published.Close())
			if !same {
				t.Fatal("retry replaced the already published clause")
			}
			repeated, err := e.set.Rule(1)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, repeated.Close()) }()
			firstHead, err := first.Head()
			fragmentMust(t, err)
			repeatedHead, err := repeated.Head()
			fragmentMust(t, err)
			firstSize, err := first.TailSize()
			fragmentMust(t, err)
			repeatedSize, err := repeated.TailSize()
			fragmentMust(t, err)
			_, hasProof, err := repeated.Proof()
			fragmentMust(t, err)
			if firstHead != repeatedHead || firstSize != repeatedSize || hasProof != proof {
				t.Fatalf("retry did not repeat first clause: heads %v/%v, sizes %d/%d, proof %t", firstHead, repeatedHead, firstSize, repeatedSize, hasProof)
			}
			firstName, err := first.Name()
			fragmentMust(t, err)
			repeatedName, err := repeated.Name()
			fragmentMust(t, err)
			if firstName != "partial-flush" || repeatedName != firstName {
				t.Fatalf("rule labels after retry = %q, %q", firstName, repeatedName)
			}
		})
	}
}
