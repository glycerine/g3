package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSMTInternalizerCloseAfterNativeLRAPopFailure(t *testing.T) {
	failed, completed := false, false
	for _, allowance := range []int64{0, 1, 4, 16} {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			x := f.variable("close-failed-lra-pop-x")
			huge, ok := new(big.Rat).SetString("-" + strings.Repeat("9876543210", 100) + "/7")
			if !ok {
				t.Fatal("invalid large bound fixture")
			}
			bound := f.pin(f.m.ArithmeticNumeralRat(huge, false))
			f.assert(f.app(smtArithOpGE, x, bound), true)
			f.propagate(true)
			astTestMust(t, f.s.PushScope())
			f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
			f.propagate(true)

			// Restoring the large parent bound replaces the child's immediate
			// zero. Fill recycled cells so this native copy reaches allocation.
			manager := f.a.numerals
			var padding []numeral.Rational
			cleanupPadding := func() {
				f.m.Budget().FailAfter(-1)
				for i := range padding {
					astTestMust(t, manager.Del(&padding[i]))
				}
				padding = nil
			}
			t.Cleanup(cleanupPadding)
			for i := 0; i < 8192; i++ {
				stats := manager.Z.Cells.Stats()
				if stats.Slots-stats.Live-stats.Retired == 0 {
					break
				}
				v := numeral.NewRational()
				astTestMust(t, manager.SetInt64(&v, 1<<60))
				padding = append(padding, v)
			}
			stats := manager.Z.Cells.Stats()
			if stats.Slots-stats.Live-stats.Retired != 0 {
				t.Fatal("did not reach native bound-copy allocation", stats)
			}
			f.m.Budget().FailAfter(allowance)
			failure := f.s.PopScopes(1)
			f.m.Budget().FailAfter(-1)
			if failure != nil {
				failed = true
				var physical memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &physical) || physical.Kind != memory.FailureMemout) {
					t.Fatal("unexpected native pop failure", failure)
				}
				if f.s.pendingPop == nil || f.s.scopeLevel != 1 || len(f.a.scopes) != 0 || !f.a.numeric.stopped {
					t.Fatal("failure did not reach the consumed theory scope", f.s.pendingPop, f.s.scopeLevel, len(f.a.scopes))
				}
				if err := f.a.numeric.pop(1); !errors.Is(err, lp.ErrExactVectorPending) {
					t.Fatal("stopped numeric owner accepted another pop", err)
				}
				if err := f.s.PushScope(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("failed pop accepted another scope", err)
				}
			} else {
				completed = true
			}
			cleanupPadding()
			astTestMust(t, f.s.Close())
			astTestMust(t, f.s.Close())
			if !f.s.closed || f.s.pendingPop != nil || f.s.scopeLevel != 0 || f.a.numeric != nil {
				t.Fatal("destruction left the failed numeric owner")
			}
		})
	}
	if !failed || !completed {
		t.Fatal("native allocation campaign missed failure or success", failed, completed)
	}
}

func TestSMTInternalizerCloseDiscardsPreservedLearnedRoots(t *testing.T) {
	var state *SMTInternalizerState
	var failure error
	m, registry, _, trigger := smtQuantifierRefusalFixture(t, func() { failure = state.Close() })
	_, _, state = smtInternalizerFixture(t, m, registry)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	astTestMust(t, state.PushScope())
	var expressions []ast.ASTHandle
	var literals []SMTLiteral
	for _, name := range []string{"flush-preserved-a", "flush-preserved-b", "flush-preserved-c"} {
		expression := smtRelevancyBool(t, m, name)
		variable, err := state.MkBoolVar(expression)
		astTestMust(t, err)
		expressions = append(expressions, expression)
		literals = append(literals, NewSMTLiteral(variable, false))
	}
	formula, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpOr, nil, expressions)
	astTestMust(t, err)
	proof, err := m.MkProofAsserted(formula)
	astTestMust(t, err)
	astTestMust(t, state.addLearnedClauseWithProof(literals, proof, false))
	learned, err := state.preserveLearnedClauses(0)
	astTestMust(t, err)
	state.pendingPop = &smtInternalizerPendingPop{count: 1, preserve: true, stage: 1, learned: learned}
	trigger()
	if !errors.Is(failure, ast.ErrASTPending) || state.pendingPop == nil || state.pendingPop.learned[0].proof.IsNull() || len(state.clauses) != 1 {
		t.Fatal("refused destruction lost preserved or installed clause owners", failure)
	}
	created := state.statistics.NumMkBoolVar
	astTestMust(t, state.Close())
	if state.statistics.NumMkBoolVar != created || state.pendingPop != nil || len(state.clauses) != 0 {
		t.Fatal("destruction restored a discarded learned clause")
	}
	if registry.scopeLevel != 0 || registry.pendingPop != 0 || len(registry.fingerprints.scopes) != 0 {
		t.Fatal("borrowed registry kept destroyed context scopes", registry.scopeLevel, registry.pendingPop)
	}
	if len(state.relevancy.scopes) != 0 {
		t.Fatal("relevancy kept destroyed context scopes")
	}
}
