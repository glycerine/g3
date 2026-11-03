package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// The source owns qhead/args in dl_rule.cpp:334-342 and the relation-query
// argument vector in dl_engine_base.h:54-70. Registered fresh declarations may
// persist until the rule manager closes; they are counted at that boundary.
func TestQueryHeadAndSpacerRelationColdConstructionOwnership(t *testing.T) {
	for _, stage := range []string{"head", "spacer"} {
		for _, width := range []int{0, 512} {
			t.Run(fmt.Sprintf("%s/%d", stage, width), func(t *testing.T) {
				failures, completed := 0, false
				for allowance := int64(0); allowance < 512 && !completed; allowance++ {
					t.Run(fmt.Sprint(allowance), func(t *testing.T) {
						var budget *arena.Budget
						t.Cleanup(func() {
							if budget != nil && budget.Used() != 0 {
								t.Fatal("query fixture leaked bytes", budget.Used())
							}
						})
						m, g := newPDRLifecycleServices(t)
						budget = m.Budget()
						initial := m.Stats()
						e, err := NewPropositionalEngineWithServices(m, g.Context, g)
						fragmentMust(t, err)
						t.Cleanup(func() { fragmentMust(t, e.Close()) })
						boolean, err := m.MkBoolSort()
						fragmentMust(t, err)
						domain := make([]ast.ASTHandle, width)
						for i := range domain {
							domain[i] = boolean
						}
						name, err := m.Symbols.Intern("cold-owned-spacer-query")
						fragmentMust(t, err)
						declaration, err := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
						fragmentMust(t, err)
						fragmentMust(t, e.RegisterPredicate(declaration))
						var query ast.ASTRef
						var body ast.ASTHandle
						if stage == "head" {
							query, err = e.makeRelationQueryExpression(declaration)
							fragmentMust(t, err)
							body, err = m.RefGet(query)
							fragmentMust(t, err)
						}
						before := m.Stats()
						run := func(inject bool) error {
							if inject {
								budget.FailAfter(allowance)
							}
							if stage == "head" {
								decl, head, cause := e.makeQueryHead(body)
								budget.FailAfter(-1)
								if cause != nil {
									if !decl.IsNull() || head != (ast.ASTRef{}) {
										t.Fatal("failed head was published")
									}
									return cause
								}
								h, err := m.RefGet(head)
								fragmentMust(t, err)
								for i := uint32(0); i < uint32(width); i++ {
									arg, err := m.GetArg(h, i)
									fragmentMust(t, err)
									index, err := m.GetVarIndex(arg)
									fragmentMust(t, err)
									if index != uint32(width)-i-1 {
										t.Fatal("head variable order", i, index)
									}
								}
								return m.DelRef(&head)
							}
							decl, rule, cause := e.makeSpacerRelationQuery(declaration, false)
							budget.FailAfter(-1)
							if cause != nil {
								if !decl.IsNull() || rule != nil {
									t.Fatal("failed Spacer query was published")
								}
								return cause
							}
							head, err := rule.Head()
							fragmentMust(t, err)
							tail, negative, err := rule.Tail(0)
							fragmentMust(t, err)
							if negative {
								t.Fatal("negative query tail")
							}
							for i := uint32(0); i < uint32(width); i++ {
								ha, err := m.GetArg(head, i)
								fragmentMust(t, err)
								hi, err := m.GetVarIndex(ha)
								fragmentMust(t, err)
								ta, err := m.GetArg(tail, i)
								fragmentMust(t, err)
								ti, err := m.GetVarIndex(ta)
								fragmentMust(t, err)
								if hi != uint32(width)-i-1 || ti != i {
									t.Fatal("Spacer query binding order", i, hi, ti)
								}
							}
							return rule.Close()
						}
						failure := run(true)
						if failure != nil {
							failures++
							if !errors.Is(failure, arena.ErrInjectedFailure) {
								t.Fatal("unexpected construction error", failure)
							}
							var cleanup memory.ConstructionFailure
							if errors.As(failure, &cleanup) {
								fragmentMust(t, cleanup.Cleanup())
							}
						} else {
							completed = true
						}
						if stage == "head" && (m.Stats().LiveNodes != before.LiveNodes || m.Stats().LiveReferences != before.LiveReferences) {
							t.Fatal("head construction leaked temporary nodes", allowance, before.LiveNodes, m.Stats().LiveNodes)
						}
						fragmentMust(t, run(false))
						if query != (ast.ASTRef{}) {
							fragmentMust(t, m.DelRef(&query))
						}
						fragmentMust(t, e.Close())
						if m.Stats().LiveNodes != initial.LiveNodes || m.Stats().LiveReferences != initial.LiveReferences {
							t.Fatal("query owners survived engine disposal", allowance, initial.LiveNodes, m.Stats().LiveNodes, initial.LiveReferences, m.Stats().LiveReferences)
						}
					})
					if t.Failed() {
						break
					}
				}
				if !completed || (width != 0 && failures == 0) {
					t.Fatal("missing construction frontier", failures, completed)
				}
				t.Log("injected failures through completion", failures)
			})
		}
	}
}
