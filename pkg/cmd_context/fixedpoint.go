package cmd_context

// Source: src/muz/fp/dl_cmds.cpp declare-rel/rule/query command context and
// src/muz/base/dl_context.cpp retained orchestration. The current backend is
// the positive finite-domain fragment exposed by muz/fp.

import (
	"errors"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	muzfp "github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (c *Context) fixedpointContext() (*muzfp.PropositionalEngine, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	if c.fixedpoint != nil {
		return c.fixedpoint, nil
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	engine, err := muzfp.NewPropositionalEngineWithServices(manager, c.config.Params, c.config.Globals)
	if err != nil {
		return nil, err
	}
	c.fixedpoint = engine
	return engine, nil
}

func (c *Context) DeclareRelation(name string, domain []ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := c.DeclareFunction(name, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = engine.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	return declaration, nil
}

// SetFixedpointPredicateRepresentation selects native finite-table storage
// names for a registered relation before its first query.
func (c *Context) SetFixedpointPredicateRepresentation(declaration ast.ASTHandle, kinds []string) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.SetPredicateRepresentation(declaration, kinds)
}

func (c *Context) AddFixedpointRule(formula ast.ASTHandle, name string) error {
	return c.AddFixedpointRuleWithBound(formula, name, ^uint32(0))
}

// AddFixedpointRuleWithBound accepts the rule command's retained recursion
// bound. The concrete Datalog engine ignores it, as does the source Datalog
// flush path; the bound is consumed only by the unretained duality engine.
func (c *Context) AddFixedpointRuleWithBound(formula ast.ASTHandle, name string, _ uint32) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddRule(formula, name)
}

// UpdateFixedpointRule adds a named Horn rule or strengthens the unique
// existing rule carrying that name.
func (c *Context) UpdateFixedpointRule(formula ast.ASTHandle, name string) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.UpdateRule(formula, name)
}

// AddFixedpointFact adds a ground relation tuple using concrete AST values.
func (c *Context) AddFixedpointFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddFact(declaration, values)
}

// AddFixedpointTableFact adds a ground relation tuple using native finite
// table element numbers.
func (c *Context) AddFixedpointTableFact(declaration ast.ASTHandle, values []uint32) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddTableFact(declaration, values)
}

// AssertFixedpoint adds a background assertion in the current fixedpoint
// scope.
func (c *Context) AssertFixedpoint(formula ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AssertBackground(formula)
}

// FixedpointAssertions returns owning fixedpoint background assertions in
// insertion order.
func (c *Context) FixedpointAssertions() ([]ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.BackgroundAssertions()
}

// DeclareFixedpointVariable declares and registers an implicit Datalog
// variable, matching the declare-var command's constant-to-bound-variable path.
func (c *Context) DeclareFixedpointVariable(name string, sort ast.ASTHandle) error {
	declaration, err := c.DeclareFunction(name, nil, sort)
	if err != nil {
		return err
	}
	manager, err := c.Manager()
	if err != nil {
		return err
	}
	variable, err := manager.MkApp(declaration, nil)
	if err != nil {
		return err
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.RegisterVariable(variable)
}

func (c *Context) PushFixedpoint() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.Push()
}

func (c *Context) PopFixedpoint() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.Pop()
}

func (c *Context) prepareFixedpointQuery(params *uparams.ParamsRef) (*muzfp.PropositionalEngine, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	if err = engine.SetQueryParameters(params); err != nil {
		return nil, err
	}
	assertions, err := c.Assertions()
	if err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	fixedpointAssertions, err := engine.BackgroundAssertions()
	if err != nil {
		return nil, err
	}
	for i := range fixedpointAssertions {
		handle, x := manager.RefGet(fixedpointAssertions[i])
		if x == nil {
			assertions = append(assertions, handle)
		}
		err = errors.Join(err, x)
	}
	for i := len(fixedpointAssertions) - 1; i >= 0; i-- {
		err = errors.Join(err, manager.DelRef(&fixedpointAssertions[i]))
	}
	if err != nil {
		return nil, err
	}
	background, err := manager.MkAndArray(assertions)
	if err != nil {
		return nil, err
	}
	if err = engine.SetBackground(background); err != nil {
		return nil, err
	}
	if err = engine.SelectConfiguredEngine(); err != nil {
		return nil, err
	}
	return engine, nil
}

func (c *Context) QueryFixedpoint(declaration ast.ASTHandle) (lbool.LBool, error) {
	return c.QueryFixedpointWithParams(declaration, nil)
}

// FixedpointPredicates returns borrowed registered relation declarations in
// source declaration order for frontend saturation and inspection.
func (c *Context) FixedpointPredicates() ([]ast.ASTHandle, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.Predicates()
}

// QueryFixedpointWithParams applies query-local fixedpoint settings before
// module settings and bounds the query with the effective fixedpoint timeout.
func (c *Context) QueryFixedpointWithParams(declaration ast.ASTHandle, params *uparams.ParamsRef) (result lbool.LBool, resultErr error) {
	return c.queryFixedpoint(declaration, params, false)
}

// QueryFixedpointExpression evaluates a Boolean query with the configured timeout.
func (c *Context) QueryFixedpointExpression(query ast.ASTHandle) (lbool.LBool, error) {
	return c.queryFixedpoint(query, nil, true)
}

func (c *Context) queryFixedpoint(query ast.ASTHandle, params *uparams.ParamsRef, expression bool) (result lbool.LBool, resultErr error) {
	engine, err := c.prepareFixedpointQuery(params)
	if err != nil {
		return lbool.Undef, err
	}
	module, err := c.config.Globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.config.Params.Del(&module)) }()
	timeoutName := "timeout"
	timeout := uint32(math.MaxUint32)
	if params == nil {
		timeout, err = c.config.Params.GetUIntName(&module, &timeoutName, timeout)
	} else {
		timeout, err = c.config.Params.GetUIntFallbackName(params, &timeoutName, &module, timeout)
	}
	if err != nil {
		return lbool.Undef, err
	}
	manager, err := c.Manager()
	if err != nil {
		return lbool.Undef, err
	}
	handler := &contextTimeoutHandler{limit: manager.ResourceLimit(), active: true}
	timer, err := control.NewScopedTimer(timeout, handler)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, timer.Close())
		handler.close()
	}()
	if expression {
		return engine.QueryExpression(query)
	}
	return engine.QueryPredicate(query)
}

// QueryFixedpointRelations reports whether at least one requested relation is
// nonempty. PDR and Spacer accept exactly one relation, matching the native
// fixedpoint engine contract.
func (c *Context) QueryFixedpointRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	engine, err := c.prepareFixedpointQuery(nil)
	if err != nil {
		return lbool.Undef, err
	}
	return engine.QueryPredicates(declarations)
}

// QueryFixedpointFromLevel runs a Spacer query beginning at the requested
// proof level.
func (c *Context) QueryFixedpointFromLevel(declaration ast.ASTHandle, level uint32) (lbool.LBool, error) {
	engine, err := c.prepareFixedpointQuery(nil)
	if err != nil {
		return lbool.Undef, err
	}
	return engine.QueryPredicateFromLevel(declaration, level)
}

// FixedpointReasonUnknown returns the execution status of the latest
// fixedpoint query using the native fixedpoint status spellings.
func (c *Context) FixedpointReasonUnknown() (string, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return "", err
	}
	return engine.ReasonUnknown()
}

// FixedpointAnswer returns an owning formula for the most recently saturated
// finite relation. The caller releases it through Manager.DelRef.
func (c *Context) FixedpointAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.Answer(declaration)
}

// FixedpointProof returns the owning proof trace for the latest reachable PDR
// query when fixedpoint.generate_proof_trace is enabled.
func (c *Context) FixedpointProof(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.Proof(declaration)
}

// WriteFixedpointCertificate writes the PDR certificate selected by the
// fixedpoint query command. The Boolean result is false for backends whose
// source certificate display is intentionally empty.
func (c *Context) WriteFixedpointCertificate(out io.Writer, declaration ast.ASTHandle) (bool, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return false, err
	}
	return engine.WriteCertificate(out, declaration, c.WriteExpression)
}

// FixedpointModel returns the inductive model produced by the latest
// unreachable PDR or Spacer query. The caller owns and closes the returned
// model.
func (c *Context) FixedpointModel(declaration ast.ASTHandle) (*model.Model, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.Model(declaration)
}

// EvaluateFixedpointModel evaluates an expression in the inductive model from
// the latest unreachable PDR or Spacer query and returns one owning AST
// reference.
func (c *Context) EvaluateFixedpointModel(declaration, expression ast.ASTHandle, completion bool) (result ast.ASTRef, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	inductiveModel, err := c.FixedpointModel(declaration)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, inductiveModel.Close()) }()
	result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = inductiveModel.Evaluate(expression, result, completion, c.config.ModelEvaluator); err != nil {
		return result, err
	}
	return result, nil
}

func (c *Context) FixedpointNumLevels(declaration ast.ASTHandle) (uint32, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return 0, err
	}
	return engine.NumLevels(declaration)
}

// FixedpointCoverDelta returns an owning formula for exactly one PDR or Spacer
// frame; level -1 selects the invariant frame.
func (c *Context) FixedpointCoverDelta(level int, declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.CoverDelta(level, declaration)
}

// AddFixedpointCover installs a bound-variable property into one PDR or Spacer
// frame.
func (c *Context) AddFixedpointCover(level int, declaration, property ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddCover(level, declaration, property)
}

// AddFixedpointInvariant installs a bound-variable property in the invariant
// frame of a PDR or Spacer predicate.
func (c *Context) AddFixedpointInvariant(declaration, property ast.ASTHandle) error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.AddInvariant(declaration, property)
}

// FixedpointReachable returns the accumulated Spacer reach summary as one
// owning AST reference.
func (c *Context) FixedpointReachable(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.Reachable(declaration)
}

// FixedpointGroundSatAnswer returns the latest successful Spacer ground
// reach-fact answer as one owning AST reference.
func (c *Context) FixedpointGroundSatAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.GroundSatAnswer(declaration)
}

// FixedpointRulesAlongTrace returns owning rule formulas and their names in
// preorder for the latest successful Spacer query.
func (c *Context) FixedpointRulesAlongTrace() ([]ast.ASTRef, []string, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, nil, err
	}
	return engine.RulesAlongTrace()
}

func (c *Context) FixedpointStatistics() (statistics.Statistics, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return statistics.Statistics{}, err
	}
	var result statistics.Statistics
	if err = engine.CollectStatistics(&result); err != nil {
		return statistics.Statistics{}, err
	}
	return result, nil
}

func (c *Context) ResetFixedpointStatistics() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.ResetStatistics()
}

// FixedpointRules returns owning formulas for the current scoped Horn rules in
// insertion order. The caller releases every reference through Manager.DelRef.
func (c *Context) FixedpointRules() ([]ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return nil, err
	}
	return engine.Rules()
}
