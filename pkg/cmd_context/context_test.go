package cmd_context

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func contextMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func contextFixture(t *testing.T) *Context {
	t.Helper()
	c, err := NewStandaloneContext()
	contextMust(t, err)
	budget := c.owned.budget
	t.Cleanup(func() {
		contextMust(t, c.Close())
		if n := budget.Used(); n != 0 {
			t.Errorf("command context retained %d bytes", n)
		}
	})
	return c
}

func TestCommandContextUnsignedOptionsValidateTypesAndUpdateLiveSolver(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetUIntOption("smt.relevancy", 0))
	contextMust(t, c.SetUIntOption("rlimit", ^uint32(0)))
	contextMust(t, c.SetUIntOption("timeout", ^uint32(0)))
	for _, test := range []struct {
		name string
		want uint32
	}{{"rlimit", ^uint32(0)}, {"timeout", ^uint32(0)}} {
		value, err := c.config.Params.GetUIntName(&c.params, &test.name, 17)
		contextMust(t, err)
		if value != test.want {
			t.Fatal("unsigned option value", test.name, value, test.want)
		}
	}
	global, err := c.config.Globals.SourceGetValueName("smt.relevancy")
	contextMust(t, err)
	if global != "0" {
		t.Fatal("module unsigned value", global)
	}
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	contextMust(t, c.SetUIntOption("smt.relevancy", 2))
	global, err = c.config.Globals.SourceGetValueName("smt.relevancy")
	contextMust(t, err)
	if global != "2" {
		t.Fatal("live unsigned update", global)
	}
	contextMust(t, c.SetUIntOption("smt.restart_factor", 2))
	if err := c.SetUIntOption("smt.ematching", 1); err == nil {
		t.Fatal("unsigned value accepted for a Boolean option")
	}
	if err := c.SetUIntOption("smt.unported_uint_option", 1); err == nil {
		t.Fatal("unknown unsigned option accepted")
	}
}

func TestCommandContextDoubleOptionsValidateTypesAndUpdateLiveSolver(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetDoubleOption("smt.restart_factor", 1.25))
	global, err := c.config.Globals.SourceGetValueName("smt.restart_factor")
	contextMust(t, err)
	if global != "1.25" {
		t.Fatal("module double value", global)
	}
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	contextMust(t, c.SetDoubleOption("smt.qi.eager_threshold", 12.5))
	if err = c.SetDoubleOption("smt.relevancy", 1.5); err == nil {
		t.Fatal("double value accepted for an unsigned option")
	}
	if err = c.SetDoubleOption("smt.unported_double_option", 1.5); err == nil {
		t.Fatal("unknown double option accepted")
	}
}

func TestCommandContextGenericBooleanValuePreservesCatalogSymbolType(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("smt.string_solver", true))
	module, err := c.config.Globals.SourceGetModuleName("smt")
	contextMust(t, err)
	defer func() { contextMust(t, c.config.Params.Del(&module)) }()
	key := "string_solver"
	value, err := c.config.Params.GetSymbolName(&module, &key, symbol.Symbol{})
	contextMust(t, err)
	text, _, err := c.config.Params.SymbolsField.BareString(value)
	contextMust(t, err)
	shadow, err := c.config.Params.ContainsName(&c.params, &key)
	contextMust(t, err)
	if text != "true" || shadow {
		t.Fatal("generic Boolean spelling lost its catalog type or created a local shadow", text, shadow)
	}
	contextMust(t, c.SetSymbolOption("smt.string_solver", "seq"))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("generic selector recovery", status)
	}
}

func TestCommandContextGlobalUpdatesPreserveDisabledLocalAutoConfig(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("auto_config", false))
	state, err := c.parameters.State()
	contextMust(t, err)
	if state.AutoConfig {
		t.Fatal("generic update did not refresh context parameters")
	}
	_, err = c.Manager()
	contextMust(t, err)
	contextMust(t, c.SetBoolOption("auto_config", true))
	state, err = c.parameters.State()
	contextMust(t, err)
	key := "auto_config"
	local, err := c.config.Params.GetBoolName(&c.params, &key, true)
	contextMust(t, err)
	if !state.AutoConfig || local {
		t.Fatal("global update changed an existing disabled local solver parameter", state.AutoConfig, local)
	}
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("updated context", status)
	}
}

func TestCommandContextGlobalUpdatesRefreshProcessEnvironment(t *testing.T) {
	c := contextFixture(t)
	warnings := diagnostic.BorrowProcessWarningService()
	oldWarnings := warnings.Enabled()
	oldVerbosity, err := diagnostic.ProcessVerbosityLevel()
	contextMust(t, err)
	t.Cleanup(func() {
		contextMust(t, warnings.Enable(oldWarnings))
		contextMust(t, diagnostic.SetProcessVerbosityLevel(oldVerbosity))
	})
	contextMust(t, c.SetUIntOption("verbose", 17))
	contextMust(t, c.SetBoolOption("warning", false))
	verbosity, err := diagnostic.ProcessVerbosityLevel()
	contextMust(t, err)
	if verbosity != 17 || warnings.Enabled() {
		t.Fatal("process environment was not refreshed", verbosity, warnings.Enabled())
	}
	contextMust(t, c.SetBoolOption("warning", true))
	if !warnings.Enabled() {
		t.Fatal("process warning service did not refresh")
	}
}

func TestCommandContextDefinitionOwnershipAndScopes(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	b, err := c.LookupSort("Bool")
	contextMust(t, err)
	truth, err := m.MkTrue()
	contextMust(t, err)
	contextMust(t, c.DefineFunction("choose", []ast.ASTHandle{b}, truth))
	out, err := m.NewRef(ast.ASTHandle{})
	contextMust(t, err)
	defer func() { contextMust(t, m.DelRef(&out)) }()
	contextMust(t, c.MakeAppInto("choose", []ast.ASTHandle{truth}, nil, ast.ASTHandle{}, out))
	contextMust(t, c.Push(1))
	u, err := c.DeclareSort("UnusedParameter")
	contextMust(t, err)
	// The body does not mention its parameter's sort.
	contextMust(t, c.DefineFunction("choose", []ast.ASTHandle{u}, truth))
	if err := c.DefineFunction("choose", []ast.ASTHandle{u}, truth); err == nil {
		t.Fatal("duplicate macro accepted")
	}
	if _, err := c.DeclareFunction("choose", []ast.ASTHandle{u}, b); err == nil {
		t.Fatal("function/macro domain collision accepted")
	}
	contextMust(t, c.DefineFunction("temporary", nil, truth))
	contextMust(t, c.MakeAppInto("temporary", nil, nil, ast.ASTHandle{}, out))
	contextMust(t, c.Pop(1))
	value, err := m.RefGet(out)
	contextMust(t, err)
	if value != truth {
		t.Fatal("substitution output lost after scope cleanup")
	}
	if err := c.MakeAppInto("temporary", nil, nil, ast.ASTHandle{}, out); err == nil {
		t.Fatal("scoped macro survived pop")
	}
	contextMust(t, c.MakeAppInto("choose", []ast.ASTHandle{truth}, nil, ast.ASTHandle{}, out))
}

func TestCommandContextScopesDeclarationsAndModel(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	b, err := c.LookupSort("Bool")
	contextMust(t, err)
	_, err = c.DeclareFunction("p", nil, b)
	contextMust(t, err)
	p, err := c.MakeApp("p", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(p, ""))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	var values bytes.Buffer
	contextMust(t, c.WriteValues(&values, []ast.ASTHandle{p}))
	if values.String() != "((p true))\n" {
		t.Fatal(values.String())
	}
	contextMust(t, c.Push(1))
	_, err = c.DeclareFunction("local", nil, b)
	contextMust(t, err)
	negative, err := c.manager.MkNot(p)
	contextMust(t, err)
	contextMust(t, c.Assert(negative, ""))
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	contextMust(t, c.Pop(1))
	if _, err := c.MakeApp("local", nil, nil, ast.ASTHandle{}); err == nil {
		t.Fatal("local declaration survived pop")
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	if err := c.SetLogic("QF_LIA"); err == nil {
		t.Fatal("logic changed after initialization")
	}
}

func TestCommandContextNamedCoreAndProof(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetBoolOption("produce-proofs", true))
	contextMust(t, c.SetBoolOption("produce-unsat-cores", true))
	b, err := c.LookupSort("Bool")
	contextMust(t, err)
	_, err = c.DeclareFunction("p", nil, b)
	contextMust(t, err)
	p, err := c.MakeApp("p", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(p, "positive"))
	negative, err := c.manager.MkNot(p)
	contextMust(t, err)
	contextMust(t, c.Assert(negative, "negative"))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	var out bytes.Buffer
	contextMust(t, c.WriteUnsatCore(&out))
	if !bytes.Contains(out.Bytes(), []byte("positive")) || !bytes.Contains(out.Bytes(), []byte("negative")) {
		t.Fatal(out.String())
	}
	out.Reset()
	contextMust(t, c.WriteProof(&out))
	proof, err := c.solver.Proof()
	contextMust(t, err)
	fact, err := c.manager.GetProofFact(proof)
	contextMust(t, err)
	if !c.manager.IsFalse(fact) {
		t.Fatal("printed proof does not derive false")
	}
	if out.Len() == 0 {
		t.Fatal("missing proof")
	}
}

// AUDIT.md smtcore_14-F1: cmd_context::check_sat runs the solver under
// scoped_rlimit(m_params.rlimit()); push/pop push and pop the limit.
func TestCommandContextCheckSatHonorsResourceLimit(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetUIntOption("rlimit", 1))
	integer, err := c.LookupSort("Int")
	contextMust(t, err)
	_, err = c.DeclareFunction("x", nil, integer)
	contextMust(t, err)
	_, err = c.DeclareFunction("y", nil, integer)
	contextMust(t, err)
	x, err := c.MakeApp("x", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	y, err := c.MakeApp("y", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	zero, err := c.manager.ArithmeticIntegerNumeralText("0")
	contextMust(t, err)
	positive, err := c.MakeApp(">", []ast.ASTHandle{x, zero}, nil, ast.ASTHandle{})
	contextMust(t, err)
	below, err := c.MakeApp("<", []ast.ASTHandle{y, x}, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(positive, ""))
	contextMust(t, c.Assert(below, ""))
	limit := c.manager.ResourceLimit()
	depth := len(limit.Limits)
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.Undef {
		t.Fatal("rlimit 1 check-sat", status)
	}
	if len(limit.Limits) != depth {
		t.Fatal("check-sat left a pushed resource limit", len(limit.Limits), depth)
	}
	contextMust(t, c.SetUIntOption("rlimit", 0))
	contextMust(t, c.Push(2))
	if len(limit.Limits) != depth+2 {
		t.Fatal("push did not push the resource limit", len(limit.Limits), depth)
	}
	contextMust(t, c.Pop(2))
	if len(limit.Limits) != depth {
		t.Fatal("pop did not pop the resource limit", len(limit.Limits), depth)
	}
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("unlimited check-sat", status)
	}
}

// AUDIT.md cmd_context_01-F1: func_decls::find returns the first set entry
// whose arity and domain match; overloads that differ only in range are
// not ambiguous.
func TestMakeAppOverloadPicksFirstSetEntry(t *testing.T) {
	c := contextFixture(t)
	integer, err := c.LookupSort("Int")
	contextMust(t, err)
	realSort, err := c.LookupSort("Real")
	contextMust(t, err)
	_, err = c.DeclareFunction("g", []ast.ASTHandle{integer}, integer)
	contextMust(t, err)
	_, err = c.DeclareFunction("g", []ast.ASTHandle{integer}, realSort)
	contextMust(t, err)
	one, err := c.manager.ArithmeticIntegerNumeralText("1")
	contextMust(t, err)
	app, err := c.MakeApp("g", []ast.ASTHandle{one}, nil, ast.ASTHandle{})
	contextMust(t, err)
	decl, err := c.manager.GetDecl(app)
	contextMust(t, err)
	entries := c.funcDictionary().find("g").funcs.entries()
	if len(entries) != 2 || decl != entries[0] {
		t.Fatal("g resolved to", decl, "set order", entries)
	}
	// build/z3 picks g : Int -> Int for this script.
	sort, err := c.manager.GetSort(app)
	contextMust(t, err)
	if sort != integer {
		t.Fatal("g resolved to the Real overload")
	}
	// A qualified range still selects the other overload.
	app, err = c.MakeApp("g", []ast.ASTHandle{one}, nil, realSort)
	contextMust(t, err)
	sort, err = c.manager.GetSort(app)
	contextMust(t, err)
	if sort != realSort {
		t.Fatal("(as g Real) did not select the Real overload")
	}
}
