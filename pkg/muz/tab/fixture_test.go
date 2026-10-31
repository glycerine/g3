package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type tabTestContext struct {
	budget   *arena.Budget
	symbols  *usymbol.Manager
	numerals *unumeral.RationalService
	params   *uparams.ParamsContext
	catalog  *uparams.GlobalParamsCatalog
	globals  *uparams.GlobalParamsService
	manager  *ast.ASTManager
}

func newTabTestContext() (c *tabTestContext, err error) {
	c = &tabTestContext{budget: arena.NewBudget(1 << 30)}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
			c = nil
		}
	}()
	c.symbols, err = usymbol.NewManager(usymbol.Config{Budget: c.budget})
	if err != nil {
		return c, err
	}
	c.numerals, err = unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: c.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: c.budget}}}})
	if err != nil {
		return c, err
	}
	c.params, err = uparams.NewParamsContext(c.symbols, c.numerals, uparams.ParamsConfig{Budget: c.budget})
	if err != nil {
		return c, err
	}
	c.catalog, err = uparams.NewGlobalParamsCatalog(c.symbols, uparams.GlobalParamsCatalogConfig{Budget: c.budget})
	if err != nil {
		return c, err
	}
	c.globals, err = uparams.NewGlobalParamsService(c.symbols, c.params, uparams.GlobalParamsConfig{Budget: c.budget, Catalog: c.catalog})
	if err != nil {
		return c, err
	}
	if err = c.globals.InitializeSource(); err != nil {
		return c, err
	}
	c.manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: c.symbols, Numerals: c.numerals, Budget: c.budget})
	return c, err
}
func (c *tabTestContext) Manager() (*ast.ASTManager, error) { return c.manager, nil }
func (c *tabTestContext) GlobalParameters() (*uparams.GlobalParamsService, error) {
	return c.globals, nil
}
func (c *tabTestContext) NewSimpleSolver() (solver.Solver, error) {
	return smtcore.NewSMTSolver(c.manager, nil, "", smtcore.SMTSolverConfig{Params: c.params, Globals: c.globals})
}
func (c *tabTestContext) Close() (err error) {
	if c == nil {
		return nil
	}
	if c.manager != nil {
		err = errors.Join(err, c.manager.Close())
	}
	if c.globals != nil {
		err = errors.Join(err, c.globals.Close())
	}
	if c.catalog != nil {
		err = errors.Join(err, c.catalog.Close())
	}
	if c.params != nil {
		err = errors.Join(err, c.params.Close())
	}
	if c.numerals != nil {
		err = errors.Join(err, c.numerals.Close())
	}
	if c.symbols != nil {
		err = errors.Join(err, c.symbols.Close())
	}
	return err
}
