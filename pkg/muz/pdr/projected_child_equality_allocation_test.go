package pdr

// Source pdr_context.cpp:2297 owns each equality in child_states before the
// formula_o2n operation at 2316 can allocate its rewriting machinery.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDRCoupledStateEqualityReleasedAfterRenameAllocationFailure(t *testing.T) {
	m, context, rules := classifierContextFixture(t)
	closureMust(t, context.SetUseModelGeneralizer(false))
	var pins []ast.ASTRef
	t.Cleanup(func() { closureMust(t, closeASTRefs(m, pins)) })
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		closureMust(t, err)
		ref, err := m.NewRef(h)
		closureMust(t, err)
		pins = append(pins, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	arithmetic, err := m.ArithmeticFamilyID()
	closureMust(t, err)
	real := own(m.MkFamilySort(arithmetic, 0, nil))
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		symbol, err := m.Symbols.Intern(name)
		closureMust(t, err)
		declaration := own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
		closureMust(t, rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("equality-owner-P", []ast.ASTHandle{real})
	q := declare("equality-owner-Q", []ast.ASTHandle{real})
	goal := declare("equality-owner-goal", nil)
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	px, qy := own(m.MkApp(p, []ast.ASTHandle{x})), own(m.MkApp(q, []ast.ASTHandle{y}))
	head := own(m.MkApp(goal, nil))
	seven := own(m.ArithmeticRealNumeralText("7"))
	equality := own(m.MkEq(x, y))
	lowerBound := own(m.MkArithmeticGE(y, seven))
	set, err := base.NewRuleSet(rules)
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, set.Close()) })
	add := func(head ast.ASTHandle, tail []ast.ASTHandle, name string) {
		t.Helper()
		rule, err := rules.NewRule(head, tail, nil, name)
		closureMust(t, err)
		closureMust(t, set.Add(rule))
		closureMust(t, rule.Close())
	}
	// The facts admit all reals. Only the coupled transition constrains the
	// model value, so the later Q-variable equality is a fresh expression.
	add(px, nil, "equality-owner-p")
	add(qy, nil, "equality-owner-q")
	add(head, []ast.ASTHandle{px, qy, equality, lowerBound}, "equality-owner-join")
	closureMust(t, context.Initialize(set))
	transformer, found, err := context.Transformer(goal)
	closureMust(t, err)
	if !found {
		t.Fatal("missing goal transformer")
	}
	// Match Solve's proof-mode scope while obtaining and projecting its
	// arithmetic predecessor model through the internal source operations.
	restoreProofMode, err := context.enterPDRProofMode()
	closureMust(t, err)
	defer func() { closureMust(t, restoreProofMode()) }()
	truth := own(m.MkTrue())
	check, err := transformer.CheckReachable(1, truth, true, true)
	closureMust(t, err)
	if check.Status != lbool.True || check.Model == nil {
		t.Fatal("missing reachable coupled model", check.Status)
	}
	root, err := NewModelNode(nil, truth, transformer, 1)
	closureMust(t, err)
	closureMust(t, context.search.SetRoot(root))
	closureMust(t, root.SetModel(check.Model))
	closureMust(t, check.Model.DecRef())
	second, found, err := context.Transformer(q)
	closureMust(t, err)
	if !found {
		t.Fatal("missing second predecessor transformer")
	}
	declaration, err := context.owner.OldToOld(second.signature[0], 0, 1)
	closureMust(t, err)
	variable := own(m.MkApp(declaration, nil))
	valueRef, err := m.NewRef(ast.ASTHandle{})
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, m.DelRef(&valueRef)) })
	found, err = second.modelConstant(root.model, variable, valueRef)
	closureMust(t, err)
	if !found {
		t.Fatal("missing second predecessor model assignment")
	}
	value, err := m.RefGet(valueRef)
	closureMust(t, err)
	variableID, err := m.GetASTID(variable)
	closureMust(t, err)
	valueID, err := m.GetASTID(value)
	closureMust(t, err)
	// Reuse the ordinary AST boundary writer with this exact equality's
	// operands. No equality is constructed by the test before the operation.
	writer := &projectedCubeAllocationWriter{manager: m, suffix: fmt.Sprintf(" = #%d #%d\n", variableID, valueID)}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget(), BorrowedWriter: writer})
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, stream.Close()) })
	func() {
		m.TraceStream = stream
		defer func() {
			m.TraceStream = nil
			m.Budget().FailAfter(-1)
		}()
		err = context.createChildrenProjected(root)
	}()
	if !writer.fired || !errors.Is(err, arena.ErrInjectedFailure) || !strings.Contains(err.Error(), "child-state renaming") || len(root.children) != 0 {
		t.Fatal("missed fresh coupled equality rename failure", writer.fired, len(root.children), err)
	}
	counts := func() [4]uint32 {
		s := m.Stats()
		return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
	}
	before := counts()
	recreated, err := m.MkEq(variable, value)
	closureMust(t, err)
	after := counts()
	ref, err := m.NewRef(recreated)
	closureMust(t, err)
	closureMust(t, m.DelRef(&ref))
	want := before
	want[0]++
	if after != want || counts() != before {
		t.Fatal("rename failure left its equality unowned", before, after, counts())
	}
	closureMust(t, context.search.Reset())
	status, err := context.Solve(goal)
	closureMust(t, err)
	root, found, err = context.search.Root()
	closureMust(t, err)
	if status != lbool.True || !found || !root.IsClosed() || len(root.children) != 2 {
		t.Fatal("same-context solve retry lost the coupled derivation", status, found)
	}
	for i, predicate := range []ast.ASTHandle{p, q} {
		if !root.children[i].IsClosed() || root.children[i].transformer.head != predicate {
			t.Fatal("solve retry lost a closed predecessor", i)
		}
	}
	// The fixture closes all source owners and requires zero final budget.
}
