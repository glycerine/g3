package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestModelImplicantArrayExtractionSourcePrecondition(t *testing.T) {
	for _, choice := range []string{"T", "C"} {
		for _, assertions := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/assertions=%t", choice, assertions), func(t *testing.T) {
				m, md, implicant, trace, tags := modelImplicantDiagnosticFixture(t)
				output, input := modelImplicantVerificationFixture(t, m, choice)
				arrayTableMust(t, diagnostic.BorrowProcessDebugContext().EnableAssertions(assertions))
				scalar, err := m.MkFalse()
				arrayTableMust(t, err)
				tags.Enable("pdr")
				before := m.Stats()
				arrayTableMust(t, implicant.setupModel(md))
				frame, ok, err := implicant.extractArray(scalar)
				asserted := ast.ASTSourceDebug && assertions
				thrown := asserted && choice == "T"
				if thrown {
					var source z3exception.DefaultException
					if !errors.As(err, &source) {
						t.Fatal("scalar extraction lost its source assertion", err)
					}
				} else {
					arrayTableMust(t, err)
				}
				if ok {
					t.Fatal("scalar input became an array interpretation")
				}
				if frame != nil {
					arrayTableMust(t, implicant.closeArrayFrame(frame))
				}
				bytes, err := os.ReadFile(output.Name())
				arrayTableMust(t, err)
				position := uint64(0)
				if asserted {
					position = 1
					if !strings.Contains(string(bytes), "Line: 478\nm_array.is_array(a)\n") {
						t.Fatal("wrong array source assertion", string(bytes))
					}
				} else if len(bytes) != 0 {
					t.Fatal("erased array assertion reached debug action", string(bytes))
				}
				state, err := input.State()
				arrayTableMust(t, err)
				if state.Position != position {
					t.Fatal("wrong array assertion command count", state)
				}
				if tags.ShouldTrace("pdr") {
					if thrown {
						if trace.Len() != 0 {
							t.Fatal("throwing precondition reached array trace body", trace.String())
						}
					} else if !strings.Contains(trace.String(), "no translation: false\n") {
						t.Fatal("continued/erased assertion lost source fallback", trace.String())
					}
				}
				arrayTableMust(t, implicant.reset())
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("array precondition leaked frame ownership", after, before)
				}
			})
		}
	}
}
