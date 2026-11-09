package fp_test

// Source: src/muz/base/dl_context.cpp query orchestration and the native
// fixedpoint Datalog result for the positive propositional Horn fragment.

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func fpMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func fpReference(t *testing.T, script string) []string {
	t.Helper()
	path := os.Getenv("G3_CPP_REFERENCE")
	if path == "" {
		var err error
		path, err = filepath.Abs("../../../build/z3")
		fpMust(t, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("C++ reference executable unavailable:", err)
	}
	command := exec.Command(path, "-in")
	command.Stdin = strings.NewReader(script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	return strings.Fields(string(output))
}

func TestPropositionalEngineMatchesCPPFixedpointQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	declarations := make(map[string]ast.ASTHandle)
	applications := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q", "R", "U"} {
		symbol, e := manager.Symbols.Intern("fp-" + name)
		fpMust(t, e)
		declarations[name], e = manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, e)
		applications[name], e = manager.MkApp(declarations[name], nil)
		fpMust(t, e)
		fpMust(t, engine.RegisterPredicate(declarations[name]))
	}
	bitSymbol, err := manager.Symbols.Intern("fp-Bit")
	fpMust(t, err)
	bit, err := manager.MkFuncDecl(bitSymbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(bit))
	fpMust(t, engine.AddTableFact(bit, []uint32{1}))
	if err = engine.AddTableFact(bit, []uint32{2}); err == nil {
		t.Fatal("out-of-domain table fact succeeded")
	}
	add := func(formula ast.ASTHandle, name string) {
		t.Helper()
		reference, e := manager.NewRef(formula)
		fpMust(t, e)
		t.Cleanup(func() { fpMust(t, manager.DelRef(&reference)) })
		fpMust(t, engine.AddRule(formula, name))
	}
	fpMust(t, engine.AddTableFact(declarations["P"], nil))
	for _, edge := range []struct{ from, to string }{{"P", "Q"}, {"Q", "R"}} {
		formula, e := manager.MkImplies(applications[edge.from], applications[edge.to])
		fpMust(t, e)
		add(formula, edge.from+"-"+edge.to)
	}
	got := make([]lbool.LBool, 2)
	got[0], err = engine.Query(applications["R"])
	fpMust(t, err)
	compiled, err := engine.LastQueryUsedCompiledCode()
	fpMust(t, err)
	if !compiled {
		t.Fatal("supported finite Horn program bypassed relational bytecode")
	}
	got[1], err = engine.Query(applications["U"])
	fpMust(t, err)
	combined, err := engine.QueryPredicates([]ast.ASTHandle{declarations["U"], declarations["R"]})
	fpMust(t, err)
	if combined != lbool.True {
		t.Fatal("relation-vector query missed nonempty relation", combined)
	}
	empty, err := engine.QueryPredicates(nil)
	fpMust(t, err)
	if empty != lbool.True {
		t.Fatal("empty relation-vector query", empty)
	}
	bitStatus, err := engine.QueryPredicate(bit)
	fpMust(t, err)
	if bitStatus != lbool.True {
		t.Fatal("decoded table fact was not retained", bitStatus)
	}

	cpp := fpReference(t, `(set-option :fixedpoint.engine datalog)
(declare-rel P ())
(declare-rel Q ())
(declare-rel R ())
(declare-rel U ())
(rule P)
(rule (=> P Q))
(rule (=> Q R))
(query R :print-answer false)
(query U :print-answer false)
`)
	if len(cpp) != 2 {
		t.Fatal("C++ fixedpoint output", cpp)
	}
	want := map[lbool.LBool]string{lbool.True: "sat", lbool.False: "unsat"}
	for i := range got {
		if cpp[i] != want[got[i]] {
			t.Fatal("fixedpoint differential", i, got[i], cpp[i])
		}
	}
}

func TestPropositionalEngineNamedRuleUpdate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	references := make([]ast.ASTRef, 0)
	pin := func(handle ast.ASTHandle, x error) ast.ASTHandle {
		t.Helper()
		fpMust(t, x)
		reference, x := manager.NewRef(handle)
		fpMust(t, x)
		references = append(references, reference)
		return handle
	}
	t.Cleanup(func() {
		for i := len(references) - 1; i >= 0; i-- {
			fpMust(t, manager.DelRef(&references[i]))
		}
	})
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean := pin(manager.MkBoolSort())
	applications := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q", "U"} {
		symbol, x := manager.Symbols.Intern("update-" + name)
		fpMust(t, x)
		declaration := pin(manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{}))
		applications[name] = pin(manager.MkApp(declaration, nil))
		fpMust(t, engine.RegisterPredicate(declaration))
	}
	oldRule := pin(manager.MkImplies(applications["P"], applications["Q"]))
	body := pin(manager.MkAndArgs([]ast.ASTHandle{applications["P"], applications["U"]}))
	strongerRule := pin(manager.MkImplies(body, applications["Q"]))

	fpMust(t, engine.AddRule(applications["P"], "fact"))
	fpMust(t, engine.AddRule(oldRule, "edge"))
	fpMust(t, engine.Push())
	fpMust(t, engine.UpdateRule(strongerRule, "edge"))
	fpMust(t, engine.Pop())
	status, err := engine.Query(applications["Q"])
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("pop did not restore named rule", status)
	}

	inserted, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, inserted.Close()) })
	for _, name := range []string{"P", "Q", "U"} {
		declaration, x := manager.GetDecl(applications[name])
		fpMust(t, x)
		fpMust(t, inserted.RegisterPredicate(declaration))
	}
	fpMust(t, inserted.UpdateRule(applications["P"], "new-fact"))
	status, err = inserted.Query(applications["P"])
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("update with a fresh name did not insert", status)
	}

	rejected, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, rejected.Close()) })
	for _, name := range []string{"P", "Q", "U"} {
		declaration, x := manager.GetDecl(applications[name])
		fpMust(t, x)
		fpMust(t, rejected.RegisterPredicate(declaration))
	}
	fpMust(t, rejected.AddRule(applications["P"], "fact"))
	fpMust(t, rejected.AddRule(strongerRule, "edge"))
	status, err = rejected.Query(applications["Q"])
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("strong normalized rule reached Q", status)
	}
	if err = rejected.UpdateRule(oldRule, "edge"); err == nil {
		t.Fatal("weakening a named rule succeeded")
	}
	status, err = rejected.Query(applications["Q"])
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("rejected update mutated the rule set", status)
	}

	validation, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, validation.Close()) })
	for _, name := range []string{"P", "Q", "U"} {
		declaration, x := manager.GetDecl(applications[name])
		fpMust(t, x)
		fpMust(t, validation.RegisterPredicate(declaration))
	}
	fpMust(t, validation.AddRule(applications["P"], "duplicate"))
	fpMust(t, validation.AddRule(applications["U"], "duplicate"))
	status, err = validation.Query(applications["P"])
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("duplicate fixtures did not flush", status)
	}
	if err = validation.UpdateRule(applications["Q"], "duplicate"); err == nil {
		t.Fatal("updating a duplicate rule name succeeded")
	}
	multiple := pin(manager.MkAndArgs([]ast.ASTHandle{applications["P"], applications["U"]}))
	if err = validation.UpdateRule(multiple, "multiple"); err == nil {
		t.Fatal("multi-rule update succeeded")
	}
	rules, err := validation.Rules()
	fpMust(t, err)
	if len(rules) != 2 {
		t.Fatal("rejected validation update mutated the rule set", len(rules))
	}
	for i := len(rules) - 1; i >= 0; i-- {
		fpMust(t, manager.DelRef(&rules[i]))
	}
}

func TestPDREngineMatchesCPPIntegerHornQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("pdr"))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	fpMust(t, err)
	declarations := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q", "U"} {
		symbol, x := manager.Symbols.Intern("pdr-integer-" + name)
		fpMust(t, x)
		declarations[name], x = manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, x)
		fpMust(t, engine.RegisterPredicate(declarations[name]))
	}
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fpMust(t, err)
	pZero, err := manager.MkApp(declarations["P"], []ast.ASTHandle{zero})
	fpMust(t, err)
	fpMust(t, engine.AddRule(pZero, "integer-fact"))
	xName, err := manager.Symbols.Intern("pdr-integer-x")
	fpMust(t, err)
	x, err := manager.MkConst(xName, integer)
	fpMust(t, err)
	fpMust(t, engine.RegisterVariable(x))
	pX, err := manager.MkApp(declarations["P"], []ast.ASTHandle{x})
	fpMust(t, err)
	qX, err := manager.MkApp(declarations["Q"], []ast.ASTHandle{x})
	fpMust(t, err)
	step, err := manager.MkImplies(pX, qX)
	fpMust(t, err)
	fpMust(t, engine.AddRule(step, "integer-step"))
	gotReachable, err := engine.QueryPredicate(declarations["Q"])
	fpMust(t, err)
	answer, err := engine.Answer(declarations["Q"])
	fpMust(t, err)
	answerHandle, err := manager.RefGet(answer)
	fpMust(t, err)
	if !manager.IsTrue(answerHandle) {
		t.Fatal("PDR existential declaration answer is not true")
	}
	fpMust(t, manager.DelRef(&answer))
	if unexpected, x := engine.Answer(declarations["U"]); x == nil {
		fpMust(t, manager.DelRef(&unexpected))
		t.Fatal("PDR answer accepted a declaration other than the latest query")
	}
	stats := statistics.New()
	fpMust(t, engine.CollectStatistics(stats))
	if stats.Size() == 0 {
		t.Fatal("PDR engine exposed no statistics after reachable query")
	}
	gotUnreachable, err := engine.QueryPredicate(declarations["U"])
	fpMust(t, err)
	reusedStats := statistics.New()
	fpMust(t, engine.CollectStatistics(reusedStats))
	fpMust(t, engine.ResetStatistics())
	if gotReachable != lbool.True || gotUnreachable != lbool.False {
		t.Fatal("PDR integer Horn results", gotReachable, gotUnreachable)
	}
	certificate, err := engine.Answer(declarations["U"])
	fpMust(t, err)
	certificateHandle, err := manager.RefGet(certificate)
	fpMust(t, err)
	// rel_query leaves m_mc null and every rule is pruned, so the source's
	// inductive property for the unreachable query is empty: true.
	if !manager.IsTrue(certificateHandle) {
		t.Fatal("PDR unsuccessful relation query certificate is not the source's true")
	}
	fpMust(t, manager.DelRef(&certificate))

	cpp := fpReference(t, `(set-option :fixedpoint.engine spacer)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel U (Int))
(declare-var x Int)
(rule (P 0))
(rule (=> (P x) (Q x)))
(query Q :print-answer false)
(query U :print-answer false)
`)
	if len(cpp) != 2 || cpp[0] != "sat" || cpp[1] != "unsat" {
		t.Fatal("C++ PDR fixedpoint output", cpp)
	}
}

func TestSpacerEngineRunsReachableAndInductiveHornQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("spacer"))
	fp.InstallContextModelConverter(engine) // a prior context::query installed m_mc
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	declarations := make(map[string]ast.ASTHandle)
	applications := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q", "U"} {
		symbol, x := manager.Symbols.Intern("spacer-engine-" + name)
		fpMust(t, x)
		declarations[name], x = manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, x)
		applications[name], x = manager.MkApp(declarations[name], nil)
		fpMust(t, x)
		fpMust(t, engine.RegisterPredicate(declarations[name]))
	}
	fpMust(t, engine.AddRule(applications["P"], "spacer-fact"))
	step, err := manager.MkImplies(applications["P"], applications["Q"])
	fpMust(t, err)
	fpMust(t, engine.AddRule(step, "spacer-step"))
	recursive, err := manager.MkImplies(applications["U"], applications["U"])
	fpMust(t, err)
	fpMust(t, engine.AddRule(recursive, "spacer-unreachable"))
	reachable, err := engine.QueryPredicate(declarations["Q"])
	fpMust(t, err)
	if reachable != lbool.True {
		t.Fatal("Spacer reachable query", reachable)
	}
	levels, err := engine.NumLevels(declarations["Q"])
	fpMust(t, err)
	// Default inlining removes Q itself; source get_num_levels returns zero
	// for absent transformers. Live predicate frames are checked separately
	// by the recursive integer lifecycle and pre-query cover regressions.
	if levels != 0 {
		t.Fatal("inlined predicate acquired phantom frames", levels)
	}
	answer, err := engine.Answer(declarations["Q"])
	fpMust(t, err)
	answerHandle, err := manager.RefGet(answer)
	fpMust(t, err)
	if !manager.IsTrue(answerHandle) {
		t.Fatal("Spacer fallback answer", answerHandle)
	}
	fpMust(t, manager.DelRef(&answer))
	ground, err := engine.GroundSatAnswer(declarations["Q"])
	fpMust(t, err)
	if ground == (ast.ASTRef{}) {
		t.Fatal("Spacer reachable query exposed an empty ground answer")
	}
	fpMust(t, manager.DelRef(&ground))
	if unexpected, err := engine.GroundSatAnswer(declarations["P"]); err == nil {
		if unexpected != (ast.ASTRef{}) {
			fpMust(t, manager.DelRef(&unexpected))
		}
		t.Fatal("Spacer ground answer accepted a declaration other than the latest query")
	}
	trace, names, err := engine.RulesAlongTrace()
	fpMust(t, err)
	// Default preprocessing/subsumption simplifies the query to an unnamed
	// fact; source symbol::str renders that null name as "<null>".
	if len(trace) != 1 || len(names) != 1 || names[0] != "<null>" {
		t.Fatal("Spacer preorder trace rules", names, len(trace))
	}
	typedTrace, labels, err := engine.RulesAlongTraceWithSymbols()
	fpMust(t, err)
	if len(typedTrace) != 1 || len(labels) != 1 || !labels[0].IsNull() {
		t.Fatal("synthetic Spacer trace label is not a null symbol", labels)
	}
	textRule, err := manager.RefGet(trace[0])
	fpMust(t, err)
	typedRule, err := manager.RefGet(typedTrace[0])
	fpMust(t, err)
	if typedRule != textRule {
		t.Fatal("typed trace changed the synthetic query formula")
	}
	fpMust(t, manager.DelRef(&typedTrace[0]))
	for index := len(trace) - 1; index >= 0; index-- {
		fpMust(t, manager.DelRef(&trace[index]))
	}
	stats := statistics.New()
	fpMust(t, engine.CollectStatistics(stats))
	if stats.Size() == 0 {
		t.Fatal("Spacer engine exposed no statistics")
	}
	unreachable, err := engine.QueryPredicate(declarations["U"])
	fpMust(t, err)
	if unreachable != lbool.False {
		t.Fatal("Spacer inductive query", unreachable)
	}
	certificate, err := engine.Answer(declarations["U"])
	fpMust(t, err)
	certificateHandle, err := manager.RefGet(certificate)
	fpMust(t, err)
	if manager.IsTrue(certificateHandle) {
		t.Fatal("Spacer unsuccessful query exposed an empty certificate")
	}
	fpMust(t, manager.DelRef(&certificate))
	ground, err = engine.GroundSatAnswer(declarations["U"])
	fpMust(t, err)
	if ground != (ast.ASTRef{}) {
		fpMust(t, manager.DelRef(&ground))
		t.Fatal("Spacer unreachable query exposed a ground answer")
	}
	inductiveModel, err := engine.Model(declarations["U"])
	fpMust(t, err)
	if declarations, err := inductiveModel.Core().NumDeclarations(); err != nil || declarations == 0 {
		t.Fatal("Spacer inductive model", declarations, err)
	}
	fpMust(t, inductiveModel.Close())
}

func TestSpacerEngineUndefinedAnswerAndGroundAvailability(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	fpMust(t, globals.SourceSetName("fixedpoint.spacer.max_level", "0"))
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("spacer"))
	fp.InstallContextModelConverter(engine) // a prior context::query installed m_mc
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("spacer-engine-undefined")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	head, err := manager.MkApp(declaration, nil)
	fpMust(t, err)
	fpMust(t, engine.AddRule(head, "spacer-undefined-fact"))
	status, err := engine.QueryPredicate(declaration)
	fpMust(t, err)
	if status != lbool.Undef {
		t.Fatal("zero-level Spacer query", status)
	}
	reason, err := engine.ReasonUnknown()
	fpMust(t, err)
	if reason != "bounded" {
		t.Fatal("zero-level Spacer reason", reason)
	}
	answer, err := engine.Answer(declaration)
	fpMust(t, err)
	answerHandle, err := manager.RefGet(answer)
	fpMust(t, err)
	if !manager.IsTrue(answerHandle) {
		t.Fatal("undefined Spacer answer is not true", answerHandle)
	}
	fpMust(t, manager.DelRef(&answer))
	ground, err := engine.GroundSatAnswer(declaration)
	fpMust(t, err)
	if ground != (ast.ASTRef{}) {
		fpMust(t, manager.DelRef(&ground))
		t.Fatal("undefined Spacer query exposed a ground answer")
	}
	var certificate bytes.Buffer
	found, err := engine.WriteCertificate(&certificate, declaration, func(io.Writer, ast.ASTHandle) error {
		t.Fatal("empty Spacer certificate invoked expression writer")
		return nil
	})
	fpMust(t, err)
	if found || certificate.Len() != 0 {
		t.Fatal("Spacer certificate presentation is not empty", found, certificate.String())
	}
}

func TestSpacerEngineReachableRefusesSlicing(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("spacer"))
	fp.InstallContextModelConverter(engine) // a prior context::query installed m_mc
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	fpMust(t, err)
	pName, err := manager.Symbols.Intern("spacer-sliced-answer-P")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(pName, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	xName, err := manager.Symbols.Intern("spacer-sliced-answer-x")
	fpMust(t, err)
	x, err := manager.MkConst(xName, integer)
	fpMust(t, err)
	fpMust(t, engine.RegisterVariable(x))
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fpMust(t, err)
	pXZero, err := manager.MkApp(predicate, []ast.ASTHandle{x, zero})
	fpMust(t, err)
	pXZeroRef, err := manager.NewRef(pXZero)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, manager.DelRef(&pXZeroRef)) })
	fpMust(t, engine.AddRule(pXZero, "sliced-answer-fact"))
	status, err := engine.QueryPredicate(predicate)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("Spacer sliced query", status)
	}
	// dl_interface::get_reachable refuses slicing, which is on by default.
	if _, err := engine.Reachable(predicate); err == nil || !strings.Contains(err.Error(), "Invariants are incompatible with slicing") {
		t.Fatal("Spacer reachable did not refuse slicing", err)
	}
}

func TestSpacerEngineReachableInlinedPredicateIsFalse(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	fpMust(t, globals.SourceSetName("fixedpoint.xform.slice", "false"))
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("spacer"))
	fp.InstallContextModelConverter(engine) // a prior context::query installed m_mc
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	fpMust(t, err)
	pName, err := manager.Symbols.Intern("spacer-inline-reachable-P")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(pName, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	goalName, err := manager.Symbols.Intern("spacer-inline-reachable-Goal")
	fpMust(t, err)
	goal, err := manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	fpMust(t, engine.RegisterPredicate(goal))
	xName, err := manager.Symbols.Intern("spacer-inline-reachable-x")
	fpMust(t, err)
	x, err := manager.MkConst(xName, integer)
	fpMust(t, err)
	fpMust(t, engine.RegisterVariable(x))
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fpMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fpMust(t, err)
	pXZero, err := manager.MkApp(predicate, []ast.ASTHandle{x, zero})
	fpMust(t, err)
	fpMust(t, engine.AddRule(pXZero, "inline-reachable-fact"))
	pXOne, err := manager.MkApp(predicate, []ast.ASTHandle{x, one})
	fpMust(t, err)
	goalApp, err := manager.MkApp(goal, nil)
	fpMust(t, err)
	queryRule, err := manager.MkImplies(pXOne, goalApp)
	fpMust(t, err)
	fpMust(t, engine.AddRule(queryRule, "inline-reachable-query"))
	status, err := engine.QueryPredicate(goal)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("Spacer inline reachable query", status)
	}
	reachable, err := engine.Reachable(predicate)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, manager.DelRef(&reachable)) })
	formula, err := manager.RefGet(reachable)
	fpMust(t, err)
	// context::get_reachable: P was inlined away, so it has no transformer.
	if !manager.IsFalse(formula) {
		t.Fatal("Spacer reachable for an inlined predicate is not false", formula)
	}
}

func TestSpacerEngineExpandsSlicedModelPredicates(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("spacer"))
	fp.InstallContextModelConverter(engine) // a prior context::query installed m_mc
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	fpMust(t, err)
	pName, err := manager.Symbols.Intern("spacer-sliced-model-P")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(pName, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	uName, err := manager.Symbols.Intern("spacer-sliced-model-U")
	fpMust(t, err)
	unreachable, err := manager.MkFuncDecl(uName, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	fpMust(t, engine.RegisterPredicate(unreachable))
	xName, err := manager.Symbols.Intern("spacer-sliced-model-x")
	fpMust(t, err)
	x, err := manager.MkConst(xName, integer)
	fpMust(t, err)
	fpMust(t, engine.RegisterVariable(x))
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fpMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fpMust(t, err)
	pXZero, err := manager.MkApp(predicate, []ast.ASTHandle{x, zero})
	fpMust(t, err)
	fpMust(t, engine.AddRule(pXZero, "sliced-model-fact"))
	pXOne, err := manager.MkApp(predicate, []ast.ASTHandle{x, one})
	fpMust(t, err)
	uApp, err := manager.MkApp(unreachable, nil)
	fpMust(t, err)
	uRule, err := manager.MkImplies(pXOne, uApp)
	fpMust(t, err)
	fpMust(t, engine.AddRule(uRule, "sliced-model-query"))
	status, err := engine.QueryPredicate(unreachable)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("Spacer sliced model query", status)
	}
	inductive, err := engine.Model(unreachable)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, inductive.Close()) })
	interpretation, err := inductive.Core().FunctionInterpretation(predicate)
	fpMust(t, err)
	if interpretation == nil || interpretation.Arity() != 2 {
		t.Fatal("Spacer model did not restore the original predicate declaration")
	}
	count := inductive.Core().NumFunctionDeclarations()
	if count != 1 {
		t.Fatal("Spacer model retained a sliced predicate declaration", count)
	}
	declaration, err := inductive.Core().Function(0)
	fpMust(t, err)
	if declaration != predicate {
		t.Fatal("Spacer model exposed the sliced predicate declaration", declaration)
	}
}

func TestPDREngineExpandsSlicedModelsAndCovers(t *testing.T) {
	for _, test := range []struct {
		name          string
		disableInline bool
	}{
		{name: "default-inlining"},
		{name: "live-sliced-transformer", disableInline: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			testPDREngineExpandsSlicedModelsAndCovers(t, test.disableInline)
		})
	}
}

func testPDREngineExpandsSlicedModelsAndCovers(t *testing.T, disableInline bool) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	if disableInline {
		// Keep the sliced predicate live in the backend so the cover assertion
		// exercises pred_transformer::get_cover_delta and its model converter.
		fpMust(t, globals.SourceSetName("fixedpoint.xform.inline_eager", "false"))
		fpMust(t, globals.SourceSetName("fixedpoint.xform.inline_linear", "false"))
	}
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	// Model conversions need m_mc, which a prior context::query installs.
	fp.InstallContextModelConverter(engine)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("pdr"))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	fpMust(t, err)
	pName, err := manager.Symbols.Intern("pdr-sliced-model-P")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(pName, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	uName, err := manager.Symbols.Intern("pdr-sliced-model-U")
	fpMust(t, err)
	unreachable, err := manager.MkFuncDecl(uName, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	fpMust(t, engine.RegisterPredicate(unreachable))
	xName, err := manager.Symbols.Intern("pdr-sliced-model-x")
	fpMust(t, err)
	x, err := manager.MkConst(xName, integer)
	fpMust(t, err)
	fpMust(t, engine.RegisterVariable(x))
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fpMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fpMust(t, err)
	pXZero, err := manager.MkApp(predicate, []ast.ASTHandle{x, zero})
	fpMust(t, err)
	fpMust(t, engine.AddRule(pXZero, "pdr-sliced-model-fact"))
	pXOne, err := manager.MkApp(predicate, []ast.ASTHandle{x, one})
	fpMust(t, err)
	uApp, err := manager.MkApp(unreachable, nil)
	fpMust(t, err)
	uRule, err := manager.MkImplies(pXOne, uApp)
	fpMust(t, err)
	fpMust(t, engine.AddRule(uRule, "pdr-sliced-model-query"))
	status, err := engine.QueryPredicate(unreachable)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("PDR sliced model query", status)
	}
	inductive, err := engine.Model(unreachable)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, inductive.Close()) })
	interpretation, err := inductive.Core().FunctionInterpretation(predicate)
	fpMust(t, err)
	if interpretation == nil || interpretation.Arity() != 2 {
		t.Fatal("PDR model did not restore the original predicate declaration")
	}
	count := inductive.Core().NumFunctionDeclarations()
	if count != 1 {
		t.Fatal("PDR model retained a sliced predicate declaration", count)
	}
	declaration, err := inductive.Core().Function(0)
	fpMust(t, err)
	if declaration != predicate {
		t.Fatal("PDR model exposed the sliced predicate declaration", declaration)
	}
	certificate, err := engine.Answer(unreachable)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, manager.DelRef(&certificate)) })
	certificateHandle, err := manager.RefGet(certificate)
	fpMust(t, err)
	printer, err := ast.NewASTPPUtil(manager)
	fpMust(t, err)
	var certificateText strings.Builder
	fpMust(t, printer.CollectAll([]ast.ASTHandle{certificateHandle}))
	fpMust(t, printer.DisplayAssertions(&certificateText, []ast.ASTHandle{certificateHandle}, true))
	if !strings.Contains(certificateText.String(), "pdr-sliced-model-P") || strings.Contains(certificateText.String(), "!slice!") {
		t.Fatalf("PDR certificate did not restore the original predicate:\n%s", certificateText.String())
	}
	cover, err := engine.CoverDelta(-1, predicate)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, manager.DelRef(&cover)) })
	coverFormula, err := manager.RefGet(cover)
	fpMust(t, err)
	used, err := ast.NewASTUsedVars(manager)
	fpMust(t, err)
	fpMust(t, used.Process(coverFormula))
	if disableInline {
		if used.Contains(0) != (ast.ASTHandle{}) || used.Contains(1) == (ast.ASTHandle{}) {
			t.Fatalf("PDR cover did not restore the original second-column variable: max=%d v0=%v v1=%v", used.MaxFoundVarIndexPlusOne(), used.Contains(0), used.Contains(1))
		}
	} else if !manager.IsTrue(coverFormula) {
		// Eager inlining eliminates P from transformed rules. Source contexts
		// have no transformer in that case and return true before conversion.
		t.Fatal("inlined-away predicate cover is not true", coverFormula)
	}
	truth, err := manager.MkTrue()
	fpMust(t, err)
	if err = engine.AddInvariant(predicate, truth); err == nil {
		t.Fatal("PDR accepted an original-arity cover for a sliced predicate")
	}
}

func TestPDREngineReadsContextAndCacheParameters(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	fpMust(t, globals.SourceSetName("fixedpoint.pdr.max_num_contexts", "1"))
	fpMust(t, globals.SourceSetName("fixedpoint.pdr.cache_mode", "3"))
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("pdr"))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("pdr-invalid-cache")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	fact, err := manager.MkApp(declaration, nil)
	fpMust(t, err)
	fpMust(t, engine.AddRule(fact, "invalid-cache-fact"))
	if _, err = engine.QueryPredicate(declaration); err == nil || !strings.Contains(err.Error(), "invalid pdr.cache_mode 3") {
		t.Fatal("PDR cache parameter was not validated", err)
	}
}

func TestPropositionalEngineCompilesSimpleJoinNormalization(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	applications := make([]ast.ASTHandle, 4)
	for index, name := range []string{"A", "B", "C", "Goal"} {
		symbol, x := manager.Symbols.Intern("simple-join-" + name)
		fpMust(t, x)
		declaration, x := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, x)
		applications[index], x = manager.MkApp(declaration, nil)
		fpMust(t, x)
		fpMust(t, engine.RegisterPredicate(declaration))
	}
	for index := 0; index < 3; index++ {
		fpMust(t, engine.AddRule(applications[index], "simple-join-fact"))
	}
	first, err := manager.MkAnd(applications[0], applications[1])
	fpMust(t, err)
	body, err := manager.MkAnd(first, applications[2])
	fpMust(t, err)
	rule, err := manager.MkImplies(body, applications[3])
	fpMust(t, err)
	fpMust(t, engine.AddRule(rule, "simple-join-goal"))
	status, err := engine.Query(applications[3])
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("three-tail rule was not derived", status)
	}
	compiled, err := engine.LastQueryUsedCompiledCode()
	fpMust(t, err)
	if !compiled {
		t.Fatal("simple-join normalization bypassed relational bytecode")
	}
}

func TestPDREngineCertificateHidesSimpleJoinAuxiliaries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	// Model conversions need m_mc, which a prior context::query installs.
	fp.InstallContextModelConverter(engine)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	fpMust(t, engine.SelectEngine("pdr"))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	applications := make([]ast.ASTHandle, 4)
	declarations := make([]ast.ASTHandle, 4)
	for index, name := range []string{"A", "B", "C", "Goal"} {
		symbol, x := manager.Symbols.Intern("certificate-join-" + name)
		fpMust(t, x)
		declarations[index], x = manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, x)
		applications[index], x = manager.MkApp(declarations[index], nil)
		fpMust(t, x)
		fpMust(t, engine.RegisterPredicate(declarations[index]))
	}
	fpMust(t, engine.AddRule(applications[0], "certificate-a"))
	fpMust(t, engine.AddRule(applications[1], "certificate-b"))
	left, err := manager.MkAnd(applications[0], applications[1])
	fpMust(t, err)
	body, err := manager.MkAnd(left, applications[2])
	fpMust(t, err)
	rule, err := manager.MkImplies(body, applications[3])
	fpMust(t, err)
	fpMust(t, engine.AddRule(rule, "certificate-goal"))
	status, err := engine.QueryPredicate(declarations[3])
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("three-tail unreachable query", status)
	}
	certificate, err := engine.Answer(declarations[3])
	fpMust(t, err)
	handle, err := manager.RefGet(certificate)
	fpMust(t, err)
	printer, err := ast.NewASTPPUtil(manager)
	fpMust(t, err)
	var text strings.Builder
	fpMust(t, printer.CollectAll([]ast.ASTHandle{handle}))
	fpMust(t, printer.DisplayAssertions(&text, []ast.ASTHandle{handle}, true))
	if strings.Contains(text.String(), "simple-join") || !strings.Contains(text.String(), "certificate-join-A") {
		t.Fatalf("converted PDR certificate:\n%s", text.String())
	}
	fpMust(t, manager.DelRef(&certificate))
}

func TestPropositionalEngineExecutesStratifiedNegation(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	applications := make(map[string]ast.ASTHandle)
	declarations := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q"} {
		symbol, e := manager.Symbols.Intern("fp-refuse-" + name)
		fpMust(t, e)
		declaration, e := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		fpMust(t, e)
		declarations[name] = declaration
		applications[name], e = manager.MkApp(declaration, nil)
		fpMust(t, e)
		fpMust(t, engine.RegisterPredicate(declaration))
	}
	negated, err := manager.MkNot(applications["P"])
	fpMust(t, err)
	formula, err := manager.MkImplies(negated, applications["Q"])
	fpMust(t, err)
	formulaRef, err := manager.NewRef(formula)
	fpMust(t, err)
	defer func() { fpMust(t, manager.DelRef(&formulaRef)) }()
	fpMust(t, engine.AddRule(formula, "negative"))
	status, err := engine.Query(applications["Q"])
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("stratified negative Horn rule was not derived", status)
	}
	status, err = engine.Query(applications["Q"])
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("repeated stratified negative query lost its result", status)
	}
	fpMust(t, engine.AddFact(declarations["P"], nil))
	status, err = engine.Query(applications["Q"])
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("negation-dependent result survived a new positive fact", status)
	}
	compiled, err := engine.LastQueryUsedCompiledCode()
	fpMust(t, err)
	if !compiled {
		t.Fatal("stratified finite negation bypassed relational bytecode")
	}
}

func TestPDRAndSpacerRejectNegativePredicates(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "qbmc", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, context.Close()) })
			manager, err := context.Manager()
			fpMust(t, err)
			globals, err := context.GlobalParameters()
			fpMust(t, err)
			engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, engine.Close()) })
			fpMust(t, engine.SelectEngine(backend))
			fp.InstallContextModelConverter(engine) // a prior context::query installed m_mc
			boolean, err := manager.MkBoolSort()
			fpMust(t, err)
			applications := make(map[string]ast.ASTHandle)
			declarations := make(map[string]ast.ASTHandle)
			for _, predicate := range []string{"P", "Q"} {
				symbol, x := manager.Symbols.Intern("fp-" + backend + "-reject-negative-" + predicate)
				fpMust(t, x)
				declarations[predicate], x = manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
				fpMust(t, x)
				applications[predicate], x = manager.MkApp(declarations[predicate], nil)
				fpMust(t, x)
				fpMust(t, engine.RegisterPredicate(declarations[predicate]))
			}
			body, err := manager.MkNot(applications["P"])
			fpMust(t, err)
			formula, err := manager.MkImplies(body, applications["Q"])
			fpMust(t, err)
			formulaRef, err := manager.NewRef(formula)
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, manager.DelRef(&formulaRef)) })
			fpMust(t, engine.AddRule(formula, "negative"))
			if _, err = engine.QueryPredicate(declarations["Q"]); !errors.Is(err, fp.ErrPropositionalHorn) {
				t.Fatal("unsupported PDR rule was accepted", err)
			}
		})
	}
}

func TestPDRAndSpacerRejectPartialDatatypeAccessors(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, context.Close()) })
			manager, err := context.Manager()
			fpMust(t, err)
			globals, err := context.GlobalParameters()
			fpMust(t, err)
			engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, engine.Close()) })
			fpMust(t, engine.SelectEngine(backend))
			fp.InstallContextModelConverter(engine) // a prior context::query installed m_mc
			boolean, err := manager.MkBoolSort()
			fpMust(t, err)
			name := func(suffix string) usymbol.Symbol {
				t.Helper()
				symbol, x := manager.Symbols.Intern("fp-" + backend + "-partial-datatype-" + suffix)
				fpMust(t, x)
				return symbol
			}
			sorts, err := manager.MkDatatypes([]ast.DatatypeDefinition{{
				Name: name("D"),
				Constructors: []ast.DatatypeConstructorDefinition{
					{Name: name("A"), Recognizer: name("is-A"), Accessors: []ast.DatatypeAccessorDefinition{{Name: name("value"), Range: boolean}}},
					{Name: name("B"), Recognizer: name("is-B")},
				},
			}})
			fpMust(t, err)
			constructors, err := manager.DatatypeConstructors(sorts[0])
			fpMust(t, err)
			accessors, err := manager.DatatypeConstructorAccessors(constructors[0])
			fpMust(t, err)
			x, err := manager.MkConst(name("x"), sorts[0])
			fpMust(t, err)
			fpMust(t, engine.RegisterVariable(x))
			q, err := manager.MkFuncDecl(name("Q"), nil, boolean, ast.ASTFuncDeclInfo{})
			fpMust(t, err)
			qApp, err := manager.MkApp(q, nil)
			fpMust(t, err)
			fpMust(t, engine.RegisterPredicate(q))
			value, err := manager.MkApp(accessors[0], []ast.ASTHandle{x})
			fpMust(t, err)
			truth, err := manager.MkTrue()
			fpMust(t, err)
			body, err := manager.MkEq(value, truth)
			fpMust(t, err)
			formula, err := manager.MkImplies(body, qApp)
			fpMust(t, err)
			formulaRef, err := manager.NewRef(formula)
			fpMust(t, err)
			t.Cleanup(func() { fpMust(t, manager.DelRef(&formulaRef)) })
			fpMust(t, engine.AddRule(formula, "partial-datatype-accessor"))
			if _, err = engine.QueryPredicate(q); !errors.Is(err, fp.ErrPropositionalHorn) {
				t.Fatal("partial datatype accessor was accepted", err)
			}
		})
	}
}

func TestDatalogRejectsUnsupportedInfinitePredicateSorts(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	fpMust(t, engine.SelectEngine("datalog"))
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	sortName, err := manager.Symbols.Intern("fp-datalog-unsupported-sort")
	fpMust(t, err)
	integer, err := manager.MkUninterpretedSort(sortName, nil)
	fpMust(t, err)
	name, err := manager.Symbols.Intern("fp-datalog-infinite-P")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	valueName, err := manager.Symbols.Intern("fp-datalog-unsupported-value")
	fpMust(t, err)
	zero, err := manager.MkConst(valueName, integer)
	fpMust(t, err)
	fact, err := manager.MkApp(predicate, []ast.ASTHandle{zero})
	fpMust(t, err)
	factRef, err := manager.NewRef(fact)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, manager.DelRef(&factRef)) })
	fpMust(t, engine.AddRule(fact, "infinite-sort-fact"))
	if _, err = engine.QueryPredicate(predicate); !errors.Is(err, fp.ErrPropositionalHorn) || !strings.Contains(err.Error(), "infinite") {
		t.Fatal("Datalog accepted an infinite predicate sort", err)
	}
}

func TestDatalogRejectsQuantifiedInterpretedTails(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	fpMust(t, engine.SelectEngine("datalog"))
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	predicateName, err := manager.Symbols.Intern("fp-datalog-quantified-P")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(predicateName, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	predicateApp, err := manager.MkApp(predicate, nil)
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	variable, err := manager.MkVar(0, boolean)
	fpMust(t, err)
	variableName, err := manager.Symbols.Intern("fp-datalog-quantified-x")
	fpMust(t, err)
	quantifier, err := manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{variableName}, Body: variable,
	})
	fpMust(t, err)
	formula, err := manager.MkImplies(quantifier, predicateApp)
	fpMust(t, err)
	formulaRef, err := manager.NewRef(formula)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, manager.DelRef(&formulaRef)) })
	fpMust(t, engine.AddRule(formula, "quantified-tail"))
	if _, err = engine.QueryPredicate(predicate); !errors.Is(err, fp.ErrPropositionalHorn) || !strings.Contains(err.Error(), "quantifier") {
		t.Fatal("Datalog accepted a quantified interpreted tail", err)
	}
}

func TestPropositionalEngineScopeGuards(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	if err = engine.Pop(); err == nil || !strings.Contains(err.Error(), "no backtracking points") {
		t.Fatal("scope underflow accepted", err)
	}
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	symbol, err := manager.Symbols.Intern("fp-scope-query")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	query, err := manager.MkApp(declaration, nil)
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	fpMust(t, engine.Push())
	status, err := engine.Query(query)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("unexpected empty-program query", status)
	}
	if err = engine.Pop(); err == nil || !strings.Contains(err.Error(), "only supported by duality engine") {
		t.Fatal("post-query pop accepted", err)
	}
}

func TestPropositionalEngineSelection(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, context.Close()) })
	manager, err := context.Manager()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngine(manager)
	fpMust(t, err)
	t.Cleanup(func() { fpMust(t, engine.Close()) })
	for _, name := range []string{"auto-config", "datalog"} {
		if err = engine.SelectEngine(name); err != nil {
			t.Fatal(name, err)
		}
	}
	for _, name := range []string{"spacer", "unknown"} {
		if err = engine.SelectEngine(name); err == nil {
			t.Fatal("unavailable engine accepted", name)
		}
	}
}
