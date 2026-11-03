package ast

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestASTFormatGroupFailurePreservesRawBorrowedInput(t *testing.T) {
	failed, completed := 0, false
	for allowance := int64(0); allowance < 128 && !completed; allowance++ {
		if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			m := astTestManager(t)
			input, err := m.FormatString("raw-borrowed-format-input")
			astTestMust(t, err)
			formats, err := m.GetFormatManager()
			astTestMust(t, err)
			// Fill the actual declaration-info page. Flattening this string
			// reuses its declaration; the subsequent Choice needs a new info.
			for index := 0; (formats.nextInfo-1)%astPageSize != 0 || formats.freeInfo != 0; index++ {
				if index >= 4096 {
					t.Fatal("format declaration-info page frontier not reached")
				}
				filler, cause := m.FormatString(fmt.Sprintf("format-page-filler-%d", index))
				astTestMust(t, cause)
				astTestPin(t, formats, filler)
			}
			counts := func() [4]uint32 {
				s := formats.Stats()
				return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
			}
			before := counts()
			m.Budget().FailAfter(allowance)
			result, cause := m.FormatGroup(input)
			m.Budget().FailAfter(-1)
			if cause != nil {
				failed++
				var receipt memory.ConstructionFailure
				if errors.As(cause, &receipt) {
					astTestMust(t, receipt.Cleanup())
					astTestMust(t, receipt.Cleanup())
				}
				if !errors.Is(cause, arena.ErrInjectedFailure) || !result.IsNull() {
					t.Fatal("unexpected format group failure", result, cause)
				}
				if refs, err := formats.GetRefCount(input); err != nil || refs != 0 {
					t.Fatal("failed FormatGroup consumed its raw borrowed input", refs, err)
				}
				if after := counts(); after != before {
					t.Fatal("failed FormatGroup lost format ownership", before, after)
				}
				result, err = m.FormatGroup(input)
				astTestMust(t, err)
			} else {
				completed = true
			}
			if refs, err := formats.GetRefCount(result); err != nil || refs != 0 {
				t.Fatal("FormatGroup changed its raw result contract", refs, err)
			}
			for column := uint32(0); column < 2; column++ {
				child, err := formats.GetArg(result, column)
				astTestMust(t, err)
				if child != input {
					t.Fatal("group or retry replaced its borrowed string input", column, child, input)
				}
			}
			// Protect the input while disposing the generated group, so exact
			// counts distinguish generated documents from the caller's input.
			inputRoot, err := formats.NewRef(input)
			astTestMust(t, err)
			outputRoot, err := formats.NewRef(result)
			astTestMust(t, err)
			astTestMust(t, formats.DelRef(&outputRoot))
			want := before
			want[3]++
			if after := counts(); after != want {
				t.Fatal("successful FormatGroup left generated documents", want, after)
			}
			astTestMust(t, formats.DelRef(&inputRoot))
		}) {
			return
		}
	}
	if failed == 0 || !completed {
		t.Fatal("format group allocation campaign missed failure or completion", failed, completed)
	}
}
