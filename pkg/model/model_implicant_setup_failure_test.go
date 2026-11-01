package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestModelImplicantColdSetupOwnsConstantBeforeReferenceGrowth(t *testing.T) {
	for _, gap := range []int{4, 8} {
		t.Run(fmt.Sprint(gap), func(t *testing.T) {
			succeeded, failures := false, 0
			for stop := int64(0); stop < 128 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
					boolean, err := m.MkBoolSort()
					arrayTableMust(t, err)
					yes, err := m.MkTrue()
					arrayTableMust(t, err)
					// Register declarations without creating their zero-arity applications.
					for n := 0; n < 16; n++ {
						symbol, err := m.Symbols.Intern(fmt.Sprintf("cold-setup-%d", n))
						arrayTableMust(t, err)
						decl, err := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
						arrayTableMust(t, err)
						arrayTableMust(t, md.Core().RegisterConstant(decl, yes))
					}
					var padding []ast.ASTRef
					defer func() {
						for n := range padding {
							arrayTableMust(t, m.DelRef(&padding[n]))
						}
					}()
					for n := 0; n < 1024; n++ {
						ref, err := m.NewRef(ast.ASTHandle{})
						arrayTableMust(t, err)
						padding = append(padding, ref)
					}
					m.Budget().FailAfter(0)
					var pressureErr error
					for len(padding) < 16384 {
						ref, err := m.NewRef(ast.ASTHandle{})
						if err != nil {
							pressureErr = err
							break
						}
						padding = append(padding, ref)
					}
					m.Budget().FailAfter(-1)
					if !errors.Is(pressureErr, arena.ErrInjectedFailure) {
						t.Fatal("reference pressure did not reach growth", pressureErr)
					}
					for n := 0; n < gap; n++ {
						last := len(padding) - 1
						arrayTableMust(t, m.DelRef(&padding[last]))
						padding = padding[:last]
					}
					before := m.Stats()
					m.Budget().FailAfter(stop)
					err = implicant.setupModel(md)
					m.Budget().FailAfter(-1)
					arrayTableMust(t, implicant.reset())
					if err != nil {
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							arrayTableMust(t, cleanup.Cleanup())
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected setup error", err)
						}
						failures++
					} else {
						succeeded = true
					}
					check := func() {
						t.Helper()
						after := m.Stats()
						if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
							t.Fatal("cold model setup leaked ownership", after, before, err)
						}
					}
					check()
					arrayTableMust(t, implicant.setupModel(md))
					if len(implicant.states) != 16 {
						t.Fatal("retry missed registered constants", len(implicant.states))
					}
					arrayTableMust(t, implicant.reset())
					check()
				})
				if t.Failed() {
					break
				}
			}
			if !succeeded && !t.Failed() {
				t.Fatal("cold setup never completed")
			}
			if failures == 0 {
				t.Fatal("no allocation failures exercised")
			}
		})
	}
}
