package pdr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRModelSubstitutionColdFailureAndIncompleteModelRetry(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		t.Run(fmt.Sprintf("incomplete=%t", incomplete), func(t *testing.T) {
			succeeded, failures := false, 0
			for stop := int64(0); stop < 2048 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m, owner, transformer, _, closeBase := closureFixture(t)
					defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
					c, err := NewContext(m, nil, 2, ReachableNoCache, owner.contexts.config)
					closureMust(t, err)
					defer func() { closureMust(t, c.Close()) }()
					var pins []ast.ASTRef
					defer func() { closureMust(t, closeASTRefs(m, pins)) }()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						t.Helper()
						closureMust(t, err)
						ref, err := m.NewRef(h)
						closureMust(t, err)
						pins = append(pins, ref)
						return h
					}
					boolean := own(m.MkBoolSort())
					truth, falsity := own(m.MkTrue()), own(m.MkFalse())
					variables := make([]ast.ASTHandle, 32)
					var sources []ast.ASTHandle
					md, err := model.NewModel(m)
					closureMust(t, err)
					defer func() { closureMust(t, md.Close()) }()
					for i := range variables {
						variables[i] = own(m.MkFreshConst("pdr-substitution", boolean))
						sources = append(sources, variables[i], variables[i])
						if i%2 == 0 || !incomplete {
							decl, err := m.GetDecl(variables[i])
							closureMust(t, err)
							value := falsity
							if i%2 == 0 {
								value = truth
							}
							closureMust(t, md.Core().RegisterConstant(decl, value))
						}
					}
					formula := own(m.MkAndArray(variables))
					mode := ast.ASTProofDisabled // create_children disables proofs before projection.
					counts := make([]uint32, len(variables))
					for i, variable := range variables {
						counts[i], err = m.GetRefCount(variable)
						closureMust(t, err)
					}
					before := m.Stats().LiveReferences
					m.Budget().FailAfter(stop)
					result, err := c.substituteModelValues(md.Core(), formula, sources)
					m.Budget().FailAfter(-1)
					if result != (ast.ASTRef{}) {
						closureMust(t, m.DelRef(&result))
					}
					if err != nil {
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							closureMust(t, cleanup.Cleanup())
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected substitution failure", err)
						}
						failures++
					} else {
						succeeded = true
					}
					check := func(completed bool) {
						t.Helper()
						if got := m.Stats().LiveReferences; got != before {
							t.Fatal("substitution leaked reference slots", got, before)
						}
						for i, variable := range variables {
							count, err := m.GetRefCount(variable)
							closureMust(t, err)
							if count != counts[i] {
								t.Fatal("substitution leaked source ownership", i, count, counts[i])
							}
							decl, err := m.GetDecl(variable)
							closureMust(t, err)
							found, err := md.Core().HasConstantInterpretation(decl)
							closureMust(t, err)
							if !found && (completed || i%2 == 0 || !incomplete) {
								t.Fatal("substitution lost a model interpretation", i)
							}
							if found {
								value, err := md.Core().ConstantInterpretation(decl)
								closureMust(t, err)
								expected := falsity
								if i%2 == 0 {
									expected = truth
								}
								if value != expected {
									t.Fatal("wrong completed interpretation", i)
								}
							}
						}
						gotMode, err := m.ProofMode()
						closureMust(t, err)
						if gotMode != mode {
							t.Fatal("substitution changed proof mode", gotMode, mode)
						}
					}
					check(false)
					retry, err := c.substituteModelValues(md.Core(), formula, sources)
					closureMust(t, err)
					defer func() {
						if retry != (ast.ASTRef{}) {
							closureMust(t, m.DelRef(&retry))
						}
					}()
					root, err := m.RefGet(retry)
					closureMust(t, err)
					if !m.IsFalse(root) {
						t.Fatal("source simplifying substitution did not collapse the false conjunction")
					}
					closureMust(t, m.DelRef(&retry))
					check(true)
				})
				if t.Failed() {
					break
				}
			}
			if !succeeded || failures == 0 {
				t.Fatal("missing substitution failure/success frontier", failures, succeeded)
			}
		})
	}
}
