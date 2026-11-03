package fp

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// The public trace regression reaches lower DER cleanup. This separate test
// starts after a real publication and exercises its remaining local owners
// under the native guard, without pretending that ExchangeContents calls back.
func TestFixedpointDeferredUpdatePostPublicationCleanupDoesNotReplay(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "completed-clauses"
		if empty {
			name = "empty-prefix"
		}
		t.Run(name, func(t *testing.T) {
			f := newContextDisplayCleanupFixture(t)
			formula := rejectedUpdateFormula(t, f)
			label, err := f.m.Symbols.Intern("postpublication")
			fragmentMust(t, err)
			clauses, err := base.NewRuleSet(f.e.rules)
			fragmentMust(t, err)
			bound, err := f.m.NewRef(formula)
			fragmentMust(t, err)
			locals := &contextFormulaAttempt{manager: f.m, sets: []*base.RuleSet{clauses}, references: []ast.ASTRef{bound}}
			t.Cleanup(func() { fragmentMust(t, locals.close()) })
			if !empty {
				fragmentMust(t, f.e.addFormula(clauses, formula, label))
			}
			before, err := f.e.set.Len()
			fragmentMust(t, err)
			published, err := f.e.publishRejectedUpdateResult(clauses)
			fragmentMust(t, err)
			if !published {
				t.Fatal("completed publication did not record its checkpoint")
			}
			wantCount := before
			if !empty {
				wantCount += 2
			}
			persistent, revision := f.e.set, f.e.revision
			raw := append([]rawRuleFormula(nil), f.e.rawRules...)
			original := errors.New("completed update conversion failed")
			u := &pendingRuleUpdate{engine: f.e, clauses: clauses, locals: locals, published: published}
			receipt := &ruleUpdateFailure{update: u, runtime: f.runtime, cause: original, reported: errors.Join(original, ast.ErrASTPending)}
			u.failure = receipt
			f.e.pendingUpdate = u
			message, exposed := receipt.Error(), receipt.Unwrap()
			f.refuse(func() {
				for retry := 0; retry < 2; retry++ {
					if err := receipt.Cleanup(); !errors.Is(err, ast.ErrASTPending) {
						t.Fatal("postpublication cleanup crossed the active native guard", err)
					}
				}
				if _, err := f.e.Predicates(); err != receipt {
					t.Fatal("engine access discarded its pending update owner", err)
				}
				if err := f.e.Close(); !errors.Is(err, ast.ErrASTPending) || f.e.closing {
					t.Fatal("engine close crossed pending native ownership", err)
				}
				if !u.published || u.done || f.e.pendingUpdate != u || len(locals.sets) != 1 || len(locals.references) != 1 {
					t.Fatal("native refusal lost its publication checkpoint or local owners")
				}
			})
			fragmentMust(t, receipt.Cleanup())
			fragmentMust(t, receipt.Cleanup())
			count, err := f.e.set.Len()
			fragmentMust(t, err)
			if count != wantCount || f.e.set != persistent || f.e.revision != revision || !reflect.DeepEqual(f.e.rawRules, raw) {
				t.Fatal("cleanup replayed publication or shifted pending inputs again", count, wantCount)
			}
			if !u.done || f.e.pendingUpdate != nil || len(locals.sets) != 0 || len(locals.references) != 0 {
				t.Fatal("completed cleanup left an update owner")
			}
			if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil {
				t.Fatal("cleanup changed the stable update failure")
			}
		})
	}
}
