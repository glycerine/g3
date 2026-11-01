package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestDefinitionVectorNormalizeColdFailureCleanupAndRetry(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 2048 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			definitions, err := NewDefinitionVector(m, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, definitions.Close()) }()
			variables := make([]ast.ASTHandle, 16)
			for i := range variables {
				variables[i] = f.constant(fmt.Sprintf("normalize-failure-%d", i), f.integer)
			}
			one, two := f.number("1"), f.number("2")
			for i, variable := range variables {
				value := two
				if i+1 < len(variables) {
					value = f.own(m.MkArithmeticAdd(variables[i+1], one))
				}
				fmMust(t, definitions.PushBack(declarationOf(t, m, variable), value))
			}
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			err = definitions.Normalize()
			m.Budget().FailAfter(-1)
			if err != nil {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					fmMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected normalization failure", err)
				}
				failures++
			} else {
				succeeded = true
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("normalization leaked reference slots", got, before)
			}
			fmMust(t, definitions.Normalize())
			if size, err := definitions.Size(); err != nil || int(size) != len(variables) {
				t.Fatal("normalization lost definitions", size, err)
			}
			for i, variable := range variables {
				declaration, err := definitions.Variable(uint32(i))
				fmMust(t, err)
				if declaration != declarationOf(t, m, variable) {
					t.Fatal("normalization changed declaration order", i)
				}
				actual, err := definitions.Definition(uint32(i))
				fmMust(t, err)
				expected := f.number(fmt.Sprint(len(variables) + 1 - i))
				if actual != expected {
					t.Fatal("retry lost a dependent substitution", i, actual, expected)
				}
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing normalization failure/success frontier", failures, succeeded)
	}
}
