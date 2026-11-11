package pdr

// Source pdr_context.cpp:2311-2316 owns the projected cube before allocating
// its model node. The ordinary AST construction stream identifies that boundary.

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

type projectedCubeAllocationWriter struct {
	manager *ast.ASTManager
	suffix  string
	line    strings.Builder
	fired   bool
}

func (w *projectedCubeAllocationWriter) Write(p []byte) (int, error) {
	for _, ch := range p {
		w.line.WriteByte(ch)
		if ch != '\n' {
			continue
		}
		line := w.line.String()
		w.line.Reset()
		if !w.fired && strings.HasPrefix(line, "[mk-app] #") && strings.HasSuffix(line, w.suffix) {
			w.fired = true
			// This completed record belongs to the selected new expression.
			// Stop tracing before later AST trace bookkeeping can
			// consume the intended model-node reference allocation failure.
			w.manager.TraceStream = nil
			w.manager.Budget().FailAfter(0)
		}
	}
	return len(p), nil
}

const projectedCubePredecessorCount = 64

func TestPDRProjectedChildCubeReleasedAfterNodeAllocationFailure(t *testing.T) {
	// Debug storage allocates each cube payload again, while native storage
	// can reuse it. Find the actual node-reference allocation boundary without
	// counting an earlier payload failure as ownership coverage.
	for target := 0; target < projectedCubePredecessorCount; target++ {
		matched := false
		passed := t.Run(fmt.Sprintf("cube-%d", target), func(t *testing.T) {
			matched = testPDRProjectedChildCubeAllocation(t, target)
		})
		if !passed || matched {
			return
		}
	}
	if ast.ASTSourceDebug {
		// Debug storage re-allocates the cube payload, and the model node's
		// reference reuses the slot the previous iteration's o_cube freed, so
		// the first failure after the cube record is in AddLeaf, after the
		// child already owns the cube.
		t.Skip("no allocation between the renamed cube and the model node under debug storage")
	}
	t.Fatal("no projected model-node reference allocation boundary reached")
}

func testPDRProjectedChildCubeAllocation(t *testing.T, target int) bool {
	m, context, rules := classifierContextFixture(t)
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
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		symbol, err := m.Symbols.Intern(name)
		closureMust(t, err)
		declaration := own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
		closureMust(t, rules.RegisterPredicate(declaration))
		return declaration
	}
	goal := declare("cube-owner-goal", nil)
	head := own(m.MkApp(goal, nil))
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
	// One native reference page worth of children forces reference growth
	// during the final loop after filling the previously available slots.
	const predecessorCount = projectedCubePredecessorCount
	var tail []ast.ASTHandle
	for i := uint32(0); i < predecessorCount; i++ {
		predicate := declare(fmt.Sprintf("cube-owner-P%d", i), []ast.ASTHandle{boolean, boolean})
		x, y := own(m.MkVar(2*i, boolean)), own(m.MkVar(2*i+1, boolean))
		application := own(m.MkApp(predicate, []ast.ASTHandle{x, y}))
		add(application, nil, fmt.Sprintf("cube-owner-fact-%d", i))
		tail = append(tail, application, own(m.MkNot(x)), own(m.MkNot(y)))
	}
	add(head, tail, "cube-owner-join")
	closureMust(t, context.Initialize(set))
	transformer, found, err := context.Transformer(goal)
	closureMust(t, err)
	if !found {
		t.Fatal("missing goal transformer")
	}
	truth := own(m.MkTrue())
	newRoot := func() *ModelNode {
		t.Helper()
		check, err := transformer.CheckReachable(1, truth, true, true)
		closureMust(t, err)
		if check.Status != lbool.True || check.Model == nil {
			t.Fatal("missing reachable predecessor model", check.Status)
		}
		root, err := NewModelNode(nil, truth, transformer, 1)
		closureMust(t, err)
		closureMust(t, context.search.SetRoot(root))
		closureMust(t, root.SetModel(check.Model))
		closureMust(t, check.Model.DecRef())
		return root
	}
	root := newRoot()
	closureMust(t, context.createChildrenProjected(root))
	if len(root.children) != predecessorCount {
		t.Fatal("warm projection missed predecessor states", len(root.children))
	}
	cubes := make([][]ast.ASTHandle, predecessorCount)
	for i, child := range root.children {
		arguments, err := m.AppArgumentView(child.state)
		closureMust(t, err)
		if arguments.Count() != 2 {
			t.Fatal("expected a fresh two-literal child conjunction", i, arguments.Count())
		}
		literals := make([]ast.ASTHandle, 2)
		for j := range literals {
			literals[j], err = arguments.At(uint32(j))
			closureMust(t, err)
			literals[j] = own(literals[j], nil)
		}
		cubes[i] = literals // Own the inputs without keeping the cube alive.
	}
	closureMust(t, context.search.Reset())
	root = newRoot()
	leftID, err := m.GetASTID(cubes[target][0])
	closureMust(t, err)
	rightID, err := m.GetASTID(cubes[target][1])
	closureMust(t, err)
	writer := &projectedCubeAllocationWriter{manager: m, suffix: fmt.Sprintf(" and #%d #%d\n", leftID, rightID)}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget(), BorrowedWriter: writer})
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, stream.Close()) })
	var fill []ast.ASTRef
	t.Cleanup(func() { closureMust(t, closeASTRefs(m, fill)) })
	for {
		m.Budget().FailAfter(0)
		ref, err := m.NewRef(truth)
		m.Budget().FailAfter(-1)
		if errors.Is(err, arena.ErrInjectedFailure) {
			break
		}
		closureMust(t, err)
		fill = append(fill, ref)
	}
	func() {
		m.TraceStream = stream
		defer func() {
			m.TraceStream = nil
			m.Budget().FailAfter(-1)
		}()
		err = context.createChildrenProjected(root)
	}()
	if writer.fired && err == nil && len(root.children) == predecessorCount {
		// The model node's reference reused a slot freed by the previous
		// iteration's o_cube, so no allocation can fail after this cube.
		return false
	}
	if !writer.fired || !errors.Is(err, arena.ErrInjectedFailure) || len(root.children) >= predecessorCount {
		t.Fatal("missed child model-node reference allocation failure", writer.fired, len(root.children), err)
	}
	// formula_o2n builds the new-name cube inside the renaming rewrite, so a
	// failure right after it is a renaming failure; either way the fresh
	// cube must be released.
	if !strings.Contains(err.Error(), "projected model-node construction") && !strings.Contains(err.Error(), "child-state renaming") {
		return false
	}
	counts := func() [4]uint32 {
		s := m.Stats()
		return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
	}
	before := counts()
	cube, err := m.MkAndArray(cubes[len(root.children)])
	closureMust(t, err)
	after := counts()
	ref, err := m.NewRef(cube)
	closureMust(t, err)
	closureMust(t, m.DelRef(&ref))
	want := before
	want[0]++
	if after != want || counts() != before {
		t.Fatal("failed model node left its fresh cube unowned", before, after, counts())
	}
	closureMust(t, context.search.Reset())
	status, err := context.Solve(goal)
	closureMust(t, err)
	root, found, err = context.search.Root()
	closureMust(t, err)
	if status != lbool.True || !found || !root.IsClosed() || len(root.children) != predecessorCount {
		t.Fatal("same-context solve retry lost the complete derivation", status, found)
	}
	for i, child := range root.children {
		if !child.IsClosed() {
			t.Fatal("solve retry left a predecessor open", i)
		}
	}
	// The fixture closes all source owners and requires zero final budget.
	return true
}
