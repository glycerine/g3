package tactic

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func lazyFixture(t *testing.T) (*ast.ASTManager, *uparams.ParamsContext) {
	t.Helper()
	m := astTestManager(t)
	c, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	return m, c
}
