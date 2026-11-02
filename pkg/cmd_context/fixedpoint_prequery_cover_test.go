package cmd_context

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointExpressionQueryPreservesPrequeryPDRCover(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetSymbolOption("fixedpoint.engine", "pdr"))
	for _, option := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
		contextMust(t, c.SetBoolOption(option, false))
	}
	m, err := c.Manager()
	contextMust(t, err)
	arithmetic, err := m.RegisterArithmeticPlugin()
	contextMust(t, err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	contextMust(t, err)
	declaration, err := c.DeclareRelation("command-covered-integer", []ast.ASTHandle{integer})
	contextMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	contextMust(t, c.AddFixedpointFact(declaration, []ast.ASTHandle{zero}))
	variable, err := m.MkVar(0, integer)
	contextMust(t, err)
	property, err := m.MkEq(variable, zero)
	contextMust(t, err)
	propertyRef, err := m.NewRef(property)
	contextMust(t, err)
	defer func() { contextMust(t, m.DelRef(&propertyRef)) }()
	query, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	contextMust(t, err)
	queryRef, err := m.NewRef(query)
	contextMust(t, err)
	defer func() { contextMust(t, m.DelRef(&queryRef)) }()
	contextMust(t, c.AddFixedpointCover(2, declaration, property))
	levels, err := c.FixedpointNumLevels(declaration)
	contextMust(t, err)
	if levels != 3 {
		t.Fatal("pre-query cover did not create three frames", levels)
	}
	status, err := c.QueryFixedpointExpression(query)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("zero fact is unreachable", status)
	}
	cover, err := c.FixedpointCoverDelta(2, declaration)
	contextMust(t, err)
	defer func() { contextMust(t, m.DelRef(&cover)) }()
	handle, err := m.RefGet(cover)
	contextMust(t, err)
	if handle != property {
		t.Fatal("query preparation lost the pre-query cover", handle, property)
	}
	contextMust(t, c.SetUIntOption("fixedpoint.pdr.cache_mode", 99))
	if _, err := c.FixedpointNumLevels(declaration); err == nil || !strings.Contains(err.Error(), "invalid pdr.cache_mode 99") {
		t.Fatal("module update did not apply the new constructor parameters", err)
	}
}
