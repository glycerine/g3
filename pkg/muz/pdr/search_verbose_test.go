package pdr_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// AUDIT.md muz_pdr_01-F4: solve_impl prints "Entering level N" and
// check_reachability "Expand node: <level>" to the verbose stream.
func TestSolveEmitsSourceSearchVerbose(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	defer func() { pdrMust(t, command.Close()) }()
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	context, err := pdr.NewContext(manager, nil, 3, pdr.ReachableHashCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	defer func() { pdrMust(t, context.Close()) }()
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	predicate := pdrFunction(t, manager, "verbose-unreachable", []ast.ASTHandle{boolean}, boolean)
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	defer func() { pdrMust(t, ruleManager.Close()) }()
	pdrMust(t, ruleManager.RegisterPredicate(predicate))
	set, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)
	defer func() { pdrMust(t, set.Close()) }()
	variable, err := manager.MkVar(0, boolean)
	pdrMust(t, err)
	head, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	pdrMust(t, err)
	rule, err := ruleManager.NewRule(head, []ast.ASTHandle{head}, nil, "self")
	pdrMust(t, err)
	pdrMust(t, set.Add(rule))
	pdrMust(t, rule.Close())
	pdrMust(t, context.Initialize(set))

	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	pdrMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	pdrMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget()})
	pdrMust(t, err)
	defer func() {
		pdrMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		pdrMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		pdrMust(t, stream.Close())
	}()
	pdrMust(t, diagnostic.SetProcessVerbosityLevel(2))
	pdrMust(t, diagnostic.SetProcessVerboseStream(stream))
	status, err := context.Solve(predicate)
	pdrMust(t, err)
	if status != lbool.False {
		t.Fatal("self-recursive predicate without facts", status)
	}
	output, err := stream.String()
	pdrMust(t, err)
	if !strings.Contains(output, "Expand node: 0\n") || !strings.Contains(output, "Entering level 1\n") {
		t.Fatalf("search verbose output %q", output)
	}
}

// AUDIT.md muz_pdr_02-F3: add_property prints IF_VERBOSE(2,
// pp_level(lvl) << " " << mk_pp(lemma_i, m)), with "oo" for infinity.
func TestPDRAddPropertyVerboseLine(t *testing.T) {
	fixture := newValidationBatchFixture(t, "add_property_verbose")
	transformer, found, err := fixture.context.Transformer(fixture.declaration)
	pdrMust(t, err)
	if !found {
		t.Fatal("missing transformer")
	}
	boolean, err := fixture.manager.MkBoolSort()
	pdrMust(t, err)
	constant := func(name string) ast.ASTHandle {
		symbol, err := fixture.manager.Symbols.Intern(name)
		pdrMust(t, err)
		value, err := fixture.manager.MkConst(symbol, boolean)
		pdrMust(t, err)
		return value
	}
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	pdrMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	pdrMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: fixture.manager.Budget()})
	pdrMust(t, err)
	defer func() {
		pdrMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		pdrMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		pdrMust(t, stream.Close())
	}()
	pdrMust(t, diagnostic.SetProcessVerbosityLevel(2))
	pdrMust(t, diagnostic.SetProcessVerboseStream(stream))
	if _, err = transformer.AddProperty(constant("add-property-a"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err = transformer.AddProperty(constant("add-property-b"), pdr.InfinityLevel); err != nil {
		t.Fatal(err)
	}
	output, err := stream.String()
	pdrMust(t, err)
	if !strings.Contains(output, "0 add-property-a\n") || !strings.Contains(output, "oo add-property-b\n") {
		t.Fatalf("add_property verbose output %q", output)
	}
}
