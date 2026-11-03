package ast

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func printerFormatLiveCounts(m *ASTManager) [4]uint32 {
	s := m.Stats()
	return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
}

type astPPRefusingWriter struct{}

func (astPPRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestASTPPUtilDisplayOwnsFormatsThroughWriterFailure(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkFreshConst("owned-printer-p", boolean)
	astTestMust(t, err)
	astTestPin(t, m, p)
	q, err := m.MkFreshConst("owned-printer-q", boolean)
	astTestMust(t, err)
	astTestPin(t, m, q)
	formula, err := m.MkAnd(p, q)
	astTestMust(t, err)
	astTestPin(t, m, formula)
	util, err := NewASTPPUtil(m)
	astTestMust(t, err)
	astTestMust(t, util.Collect(formula))
	_, err = m.FormatSort()
	astTestMust(t, err)
	formats, err := m.GetFormatManager()
	astTestMust(t, err)
	before := printerFormatLiveCounts(formats)
	for _, declarations := range []bool{true, false} {
		var output bytes.Buffer
		for _, refuse := range []bool{false, true, false} {
			var writer io.Writer = &output
			if refuse {
				writer = astPPRefusingWriter{}
			}
			if declarations {
				err = util.DisplayDeclarations(writer)
			} else {
				err = util.DisplayAssertions(writer, []ASTHandle{formula}, true)
			}
			if refuse {
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Fatal("writer failure lost", err)
				}
			} else {
				astTestMust(t, err)
			}
			if after := printerFormatLiveCounts(formats); after != before {
				t.Fatal("display left format documents alive", declarations, refuse, before, after)
			}
		}
		if declarations {
			if strings.Count(output.String(), "(declare-fun ") != 4 {
				t.Fatal(output.String())
			}
		} else if strings.Count(output.String(), "(assert (and ") != 2 {
			t.Fatal(output.String())
		}
	}
}
