package ast

// Source: src/ast/expr_abstract.cpp:112-145 (the result expr_ref scope).
// The Go factory can return a registered node alongside a stream error;
// keep that node reachable through the caller's owning result reference.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestExprAbstractorQuantifierTraceFailureOwnsRegisteredResult(t *testing.T) {
	for _, forall := range []bool{true, false} {
		name := "exists"
		if forall {
			name = "forall"
		}
		t.Run(name, func(t *testing.T) {
			m := astTestManager(t)
			sort, err := m.MkBoolSort()
			astTestMust(t, err)
			constant, err := m.MkConst(astTestSymbol(t, m, "quantifier-trace-x"), sort)
			astTestMust(t, err)
			astTestPin(t, m, constant)
			// Reusing this pinned variable makes the fresh quantifier the first
			// constructor that writes to the failing ordinary AST trace stream.
			variable, err := m.MkVar(0, sort)
			astTestMust(t, err)
			astTestPin(t, m, variable)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
			astTestMust(t, err)
			astTestMust(t, stream.Close())
			counts := func() [4]uint32 {
				s := m.Stats()
				return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
			}
			before := counts()
			for _, failTrace := range []bool{true, false} {
				a, err := NewExprAbstractor(m)
				astTestMust(t, err)
				out, err := m.NewRef(ASTHandle{})
				astTestMust(t, err)
				if failTrace {
					m.TraceStream = stream
				}
				if forall {
					err = a.MkForall([]ASTHandle{constant}, constant, out)
				} else {
					err = a.MkExists([]ASTHandle{constant}, constant, out)
				}
				m.TraceStream = nil
				if failTrace && !errors.Is(err, arena.ErrClosed) || !failTrace && err != nil {
					t.Fatalf("trace failure=%v: %v", failTrace, err)
				}
				result, err := m.RefGet(out)
				astTestMust(t, err)
				kind, err := m.GetKind(result)
				astTestMust(t, err)
				if kind != ASTQuantifier {
					t.Errorf("trace failure=%v: caller owns %v instead of registered quantifier", failTrace, kind)
				} else {
					isForall, err := m.IsForall(result)
					astTestMust(t, err)
					if isForall != forall {
						t.Errorf("quantifier forall=%v, want %v", isForall, forall)
					}
					arity, err := m.GetNumDeclarations(result)
					astTestMust(t, err)
					if arity != 1 {
						t.Errorf("quantifier declarations=%d, want 1", arity)
					}
					body, err := m.GetQuantifierBody(result)
					astTestMust(t, err)
					if body != variable {
						t.Errorf("abstracted body %v, want %v", body, variable)
					}
				}
				astTestMust(t, a.Close())
				astTestMust(t, m.DelRef(&out))
				if after := counts(); after != before {
					t.Fatalf("trace failure=%v: AST counts %v, want %v", failTrace, after, before)
				}
			}
		})
	}
}
