package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// The source substitution and its expr_ref_vector must both survive an
// independent native destructor refusal after query rewriting has finished.
func TestCompactQueryVariablesCleanupPreservesSubstitutionAndBindings(t *testing.T) {
	f := newContextDisplayCleanupFixture(t)
	m := f.m
	boolean := f.own(m.MkBoolSort())
	x := f.own(m.MkVar(2, boolean))
	before := contextDisplayLiveCounts(m.Stats())
	attempt := &contextFormulaAttempt{manager: m}
	var args []ast.ASTHandle
	for i := 0; i < 3; i++ {
		ref, err := m.NewRef(ast.ASTHandle{})
		fragmentMust(t, err)
		attempt.references = append(attempt.references, ref)
		value, err := m.MkVar(0, boolean)
		fragmentMust(t, err)
		fragmentMust(t, m.SetRef(ref, value))
		args = append(args, value)
	}
	var err error
	attempt.substitution, err = rewriter.NewVarSubstitution(m, false)
	fragmentMust(t, err)
	output, err := m.NewRef(ast.ASTHandle{})
	fragmentMust(t, err)
	attempt.references = append(attempt.references, output)
	fragmentMust(t, attempt.substitution.Substitute(x, args, output))
	substitution := attempt.substitution
	roots := append([]ast.ASTRef(nil), attempt.references...)
	original := errors.New("query compaction failed after rewriting")
	var receipt memory.ConstructionFailure
	f.refuse(func() {
		cause := attempt.finish(original)
		if !errors.Is(cause, original) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &receipt) {
			t.Fatal("compaction lost its native cleanup owner", cause)
		}
		for repeat := 0; repeat < 2; repeat++ {
			if err := receipt.Cleanup(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("cleanup bypassed the active native guard", err)
			}
			if attempt.substitution != substitution || len(attempt.references) != len(roots) {
				t.Fatal("cleanup crossed the unfinished substitution scope")
			}
			for i := range roots {
				if attempt.references[i] != roots[i] {
					t.Fatal("cleanup consumed a borrowed binding before its substitution", i)
				}
			}
		}
	})
	if receipt == nil || receipt.RuntimeOwner() != f.runtime {
		t.Fatal("compaction receipt lost its runtime")
	}
	origin, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
	fragmentMust(t, receipt.Cleanup())
	fragmentMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || receipt.OriginalCause() != origin || errors.Unwrap(receipt) != exposed || receipt.Error() != message {
		t.Fatal("cleanup completion changed the original failure transport")
	}
	if attempt.substitution != nil || len(attempt.references) != 0 {
		t.Fatal("cleanup left query compaction owners")
	}
	if after := contextDisplayLiveCounts(m.Stats()); after != before {
		t.Fatal("native compaction cleanup leaked AST ownership", before, after)
	}
}
