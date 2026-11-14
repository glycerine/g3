package api

// Source: src/api/api_context.cpp:35-55,109-119,215-246;
// src/api/api_goal.h:25-30; src/util/ref.h:79-85.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func apiResultsFixture(t *testing.T, rc bool) (*ast.ASTManager, *apiContextResults) {
	t.Helper()
	m := astTestManager(t)
	c, e := newAPIContextResults(m, rc)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	return m, c
}

func apiResultsAtom(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	h, e := m.MkConst(astTestSymbol(t, m, name), b)
	astTestMust(t, e)
	return h // deliberately no test pin; source result/goal roots must own it.
}

func TestAPIContextResultsExplicitReferencesAndAlias(t *testing.T) {
	m, c := apiResultsFixture(t, true)
	p := apiResultsAtom(t, m, "api-rc-p")
	astTestMust(t, c.saveASTTrail(p))
	for i := 0; i < 3; i++ {
		astTestMust(t, c.saveASTTrail(p)) // p's only root is the old last_result.
		rc, e := m.GetRefCount(p)
		astTestMust(t, e)
		if rc != 1 {
			t.Fatal("same-result assignment lost or duplicated its root", rc)
		}
	}
	q := apiResultsAtom(t, m, "api-rc-q")
	astTestMust(t, c.saveMultipleASTTrail(q))
	n := len(c.lastResult)
	if n != 2 {
		t.Fatal("multiple results did not append", n)
	}
	astTestMust(t, m.IncRef(p)) // user's explicit retain survives next API call.
	astTestMust(t, c.resetLastResult())
	if m.IsLiveHandle(q) {
		t.Fatal("unpinned old result survived reset")
	}
	if rc, e := m.GetRefCount(p); e != nil || rc != 1 {
		t.Fatal("user reference was lost", rc, e)
	}
	astTestMust(t, m.DecRef(p))
	if m.IsLiveHandle(p) {
		t.Fatal("last external release did not retire AST")
	}
}

func TestAPIContextResultsAutomaticReferencesAllASTKinds(t *testing.T) {
	m, c := apiResultsFixture(t, false)
	s, e := m.MkUninterpretedSort(astTestSymbol(t, m, "api-sort"), nil)
	astTestMust(t, e)
	// Use actual Bool declarations/expressions and an uninterpreted sort; the
	// source ast_ref_vector accepts all AST kinds, not just Boolean expressions.
	p := apiResultsAtom(t, m, "api-auto-p")
	d, e := m.GetDecl(p)
	astTestMust(t, e)
	for _, h := range []ast.ASTHandle{s, d, p} {
		astTestMust(t, c.saveASTTrail(h))
	}
	astTestMust(t, c.saveMultipleASTTrail(p))
	astTestMust(t, c.resetLastResult())
	n := len(c.astTrail)
	if n != 4 {
		t.Fatal("automatic reset changed the accumulated AST trail", n)
	}
	astTestMust(t, c.Close())
	for _, h := range []ast.ASTHandle{s, d, p} {
		if m.IsLiveHandle(h) {
			t.Fatal("context result member destruction retained a root", h)
		}
	}
}

func TestAPIContextObjectReferenceAssignmentAndRecycledIDs(t *testing.T) {
	_, c := apiResultsFixture(t, true)
	var objs []*apiGoalObject
	for i := 0; i < 6; i++ {
		o, e := newAPIGoalObject(c)
		astTestMust(t, e)
		if o.object.id != uint32(i) || o.object.references != 0 {
			t.Fatal("source object construction", i, o.object.id, o.object.references)
		}
		astTestMust(t, o.object.incRef())
		objs = append(objs, o)
	}
	astTestMust(t, objs[1].object.decRef())
	astTestMust(t, objs[4].object.decRef())
	for _, want := range []uint32{4, 1} {
		o, e := newAPIGoalObject(c)
		astTestMust(t, e)
		if o.object.id != want {
			t.Fatal("free object identifiers must be reused LIFO", o.object.id, want)
		}
	}
	astTestMust(t, c.saveObject(&objs[0].object))
	astTestMust(t, c.saveObject(&objs[0].object))
	if objs[0].object.references != 2 || objs[0].object.closed {
		t.Fatal("ref assignment must increment before decrement on alias")
	}
	astTestMust(t, objs[0].object.decRef())
	astTestMust(t, c.resetLastResult())
	if !objs[0].object.closed {
		t.Fatal("last API object reference was not retired")
	}
	if e := objs[0].object.incRef(); e == nil {
		t.Fatal("a retired wrapper was resurrected after ID recycling")
	}
}

func TestAPIContextGoalRootSurvivesExplicitResultResetAndDrains(t *testing.T) {
	m, c := apiResultsFixture(t, true)
	p := apiResultsAtom(t, m, "api-goal-p")
	astTestMust(t, c.saveASTTrail(p))
	g, e := tactic.NewGoal(m, false, true, false)
	astTestMust(t, e)
	astTestMust(t, g.Assert(p))
	o, e := newAPIGoalObject(c)
	astTestMust(t, e)
	astTestMust(t, o.setGoal(g))
	astTestMust(t, c.saveObject(&o.object))
	astTestMust(t, o.object.incRef()) // explicit user goal reference.
	astTestMust(t, c.resetLastResult())
	if o.object.references != 1 || g.RefCount != 1 {
		t.Fatal("goal wrapper/root ownership changed", o.object.references, g.RefCount)
	}
	s, _, _ := satSolverCheckFixture(t)
	tr, e := sattactic.NewGoalTranslator(m, s)
	astTestMust(t, e)
	forms, e := g.AppendFormulas(nil)
	astTestMust(t, e)
	astTestMust(t, tr.Compile(forms))
	r, e := s.Check(nil)
	astTestMust(t, e)
	if r != lbool.True {
		t.Fatal("retained assertion did not reach the real SAT solver", r)
	}
	values, e := s.AppendModel(nil)
	astTestMust(t, e)
	md, e := sattactic.BuildBooleanModel(tr, values)
	astTestMust(t, e)
	defer md.Close()
	d, e := m.GetDecl(p)
	astTestMust(t, e)
	v, e := md.ConstantInterpretation(d)
	astTestMust(t, e)
	want, e := m.MkTrue()
	astTestMust(t, e)
	if v != want {
		t.Fatal("retrieved model does not satisfy the retained source assertion")
	}
	astTestMust(t, tr.Close())
	astTestMust(t, c.Close()) // context deletes outstanding user wrappers too.
	if !o.object.closed || !g.Closed {
		t.Fatal("context teardown ignored an outstanding API object")
	}
	if m.IsLiveHandle(p) {
		t.Fatal("goal/translator teardown retained the application")
	}
	v, e = md.ConstantInterpretation(d)
	astTestMust(t, e)
	if v != want {
		t.Fatal("independently retained model binding was lost")
	}
}

func TestAPIContextResultsCopiedAndForeignOwnerGuards(t *testing.T) {
	m, c := apiResultsFixture(t, true)
	var copied apiContextResults
	reflect.ValueOf(&copied).Elem().Set(reflect.ValueOf(c).Elem())
	before := m.BudgetField.Used()
	if e := copied.Close(); e == nil || m.BudgetField.Used() != before {
		t.Fatal("copied result owner obtained destruction authority", e)
	}
	foreign, other := apiResultsFixture(t, true)
	p := apiResultsAtom(t, foreign, "foreign-result")
	if diagnostic.DebugBuild {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := d.AssertionsEnabled()
		astTestMust(t, e)
		astTestMust(t, d.EnableAssertions(false))
		t.Cleanup(func() { astTestMust(t, d.EnableAssertions(enabled)) })
	}
	if e := c.saveASTTrail(p); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("foreign AST entered the context trail", e)
	}
	o, e := newAPIGoalObject(other)
	astTestMust(t, e)
	if e := c.saveObject(&o.object); e == nil || o.object.references != 0 {
		t.Fatal("foreign wrapper entered last-object ownership", e)
	}
}

// A cold test-only derived destructor observes the actual virtual destruction
// boundary and releases another real registered API object during teardown.
type apiResultsTestObject struct {
	object apiObject
	child  *apiObject
	events *[]uint32
}

func (o *apiResultsTestObject) apiDestroy() error {
	*o.events = append(*o.events, o.object.id)
	if o.child != nil {
		child := o.child
		o.child = nil
		return child.decRef()
	}
	return nil
}

func TestAPIContextObjectTeardownUsesStableInsertionOrder(t *testing.T) {
	_, c := apiResultsFixture(t, false)
	var events []uint32
	var objs []*apiResultsTestObject
	for i := 0; i < 160; i++ {
		n := uint64(reflect.TypeOf(apiResultsTestObject{}).Size())
		astTestMust(t, c.manager.BudgetField.Reserve(n))
		o := &apiResultsTestObject{events: &events}
		o.object = apiObject{context: c, destructor: o, ownerBytes: n}
		o.object.self = &o.object
		id, e := c.addObject(&o.object)
		astTestMust(t, e)
		o.object.id = id
		objs = append(objs, o)
	}
	objs[0].child = &objs[100].object
	astTestMust(t, objs[100].object.incRef())
	astTestMust(t, c.Close())
	want := []uint32{0, 100}
	for id := uint32(1); id < 160; id++ {
		if id != 100 {
			want = append(want, id)
		}
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatal("stable insertion order/restarted destructor iteration", events, want)
	}
	for _, o := range objs {
		if !o.object.closed || o.object.ownerBytes != 0 {
			t.Fatal("context missed an uncollected object after hash compaction")
		}
	}
}

func TestAPIContextObjectCloseUsesNativeFreeIDStorage(t *testing.T) {
	m, c := apiResultsFixture(t, true)
	o, e := newAPIGoalObject(c)
	astTestMust(t, e)
	astTestMust(t, c.saveObject(&o.object))
	m.BudgetField.FailAfter(0)
	e = c.Close()
	m.BudgetField.FailAfter(-1)
	_, allocated := c.allocated.Get2(o.object.id)
	if e != nil || o.object.references != 0 || !o.object.closed || allocated || c.lastObject != nil {
		t.Fatal("native free-ID close", e)
	}
	astTestMust(t, c.Close())
	if !o.object.closed {
		t.Fatal("owning close did not drain the refused source prefix")
	}
}

func TestAPIContextNativeResultGrowthAndReferenceRelease(t *testing.T) {
	for _, rc := range []bool{false, true} {
		m, c := apiResultsFixture(t, rc)
		p := apiResultsAtom(t, m, "native-result-growth")
		before := m.Budget().Used()
		m.Budget().FailAfter(0)
		for i := 0; i < 4097; i++ {
			astTestMust(t, c.saveMultipleASTTrail(p))
		}
		if got, e := m.GetRefCount(p); e != nil || got != 4097 || m.Budget().Used() != before {
			t.Fatal("native root growth and one reference per entry", got, e, m.Budget().Used(), before)
		}
		astTestMust(t, c.resetLastResult())
		if rc {
			if len(c.lastResult) != 0 {
				t.Fatal("explicit results not reset")
			}
			if m.IsLiveHandle(p) {
				t.Fatal("reset retained explicit results")
			}
		} else if got, e := m.GetRefCount(p); e != nil || got != 4097 {
			t.Fatal("automatic trail must survive result reset", got, e)
		}
		astTestMust(t, c.Close())
		m.Budget().FailAfter(-1)
		if c.astTrail != nil || c.lastResult != nil {
			t.Fatal("close retained native backing")
		}
		if m.IsLiveHandle(p) {
			t.Fatal("close retained root")
		}
	}
}
