package pdr

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	params "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source pdr_context.h:190-215 gives each search node an owning model_ref.
// The solved nonlinear trace below produces fresh numeric assignments, so
// native failure exercises those actual models rather than fabricated metadata.
func TestModelSearchResetPreservesSolvedModelsAfterConsumedNativeFailure(t *testing.T) {
	testModelSearchNativeCleanup(t, false, false)
}

func TestPDRRepeatedSolveReleasesUnadoptedRootAfterNativeResetFailure(t *testing.T) {
	testModelSearchNativeCleanup(t, true, false)
}

func TestModelNodeReplacementPreservesBothSolvedModelOwnersAfterNativeFailure(t *testing.T) {
	testModelSearchNativeCleanup(t, false, true)
}

func testModelSearchNativeCleanup(t *testing.T, replacing, replacingModel bool) {
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	budget := arena.NewBudget(1 << 29)
	rt, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: rt, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	ps, err := params.NewParamsContext(symbols, numbers, params.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := params.NewGlobalParamsCatalog(symbols, params.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := params.NewGlobalParamsService(symbols, ps, params.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(m.RegisterDeclarationPlugins())
	c, err := NewContext(m, nil, 1, ReachableNoCache, smtcore.SMTSolverConfig{Params: ps, Globals: globals})
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	set, err := base.NewRuleSet(rm)
	must(err)
	var refs []ast.ASTRef
	var recovery memory.Block
	own := func(h ast.ASTHandle, e error) ast.ASTHandle {
		must(e)
		r, e := m.NewRef(h)
		must(e)
		refs = append(refs, r)
		return h
	}
	defer func() {
		finish := func(close func() error) {
			t.Helper()
			for i := 0; i < 128; i++ {
				blocks := rt.Stats().LiveBlocks
				e := close()
				if e == nil {
					return
				}
				if !errors.Is(e, arena.ErrBounds) || rt.Stats().LiveBlocks >= blocks {
					t.Fatal("cleanup no progress", e, rt.State())
				}
			}
			t.Fatal("cleanup horizon")
		}
		finish(c.Close)
		finish(set.Close)
		finish(rm.Close)
		for i := range refs {
			must(m.DelRef(&refs[i]))
		}
		if live := m.Stats().LiveReferences; live != 0 {
			t.Errorf("public cleanup left unowned AST references before manager shutdown: %d", live)
		}
		finish(m.Close)
		finish(globals.Close)
		finish(catalog.Close)
		finish(ps.Close)
		finish(numbers.Close)
		finish(symbols.Close)
		if recovery != (memory.Block{}) {
			e := rt.Deallocate(&recovery)
			if e != nil && (!errors.Is(e, arena.ErrBounds) || recovery != (memory.Block{})) {
				t.Fatal(e)
			}
		}
		finish(rt.Close)
		if budget.Used() != 0 {
			t.Fatal("budget", budget.Used())
		}
	}()
	boolean := own(m.MkBoolSort())
	zero := own(m.ArithmeticRealNumeralText("0"))
	real, err := m.GetSort(zero)
	must(err)
	number := func(low uint32) ast.ASTHandle {
		v, e := numeral.NewScopedInt(numbers.Manager.Z)
		must(e)
		n, e := v.Get()
		must(e)
		digits := make([]uint32, 128)
		digits[0], digits[len(digits)-1] = low, 1
		must(numbers.Manager.Z.SetLimbs(n, digits))
		r, e := numbers.NewFromScopedInt(v)
		must(e)
		h := own(m.ArithmeticNumeral(&r, false))
		must(numbers.Del(&r))
		must(v.Close())
		return h
	}
	lower, upper := number(17), number(19)
	decl := func(s string, domain []ast.ASTHandle) ast.ASTHandle {
		name, e := symbols.Intern(s)
		must(e)
		d := own(m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{}))
		must(rm.RegisterPredicate(d))
		return d
	}
	p := decl("native-search-P", []ast.ASTHandle{real})
	g := decl("native-search-Goal", nil)
	y, x := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	four := own(m.ArithmeticRealNumeralText("4"))
	offset := own(m.MkArithmeticMul(four, x))
	lo := own(m.MkArithmeticAdd(lower, offset))
	hi := own(m.MkArithmeticAdd(upper, offset))
	gt := own(m.MkArithmeticGT(y, lo))
	lt := own(m.MkArithmeticLT(y, hi))
	px := own(m.MkApp(p, []ast.ASTHandle{x}))
	// The goal's predecessors avoid P(0): with one shared context the goal
	// check leaves the initial x = 0 assignment of P's interval rule in
	// place, so a P(0) predecessor would hold the same numeral (as in z3
	// 4.7.1 with pdr.max_num_contexts=1). P(1) and P(2) each get their own.
	one := own(m.ArithmeticRealNumeralText("1"))
	two := own(m.ArithmeticRealNumeralText("2"))
	p0 := own(m.MkApp(p, []ast.ASTHandle{one}))
	p1 := own(m.MkApp(p, []ast.ASTHandle{two}))
	goal := own(m.MkApp(g, nil))
	add := func(head ast.ASTHandle, tail []ast.ASTHandle, name string) {
		r, e := rm.NewRule(head, tail, nil, name)
		must(e)
		must(set.Add(r))
		must(r.Close())
	}
	// The goal rule bounds its own variable in a second large interval, so
	// the root's solved model owns a large value of its own regardless of
	// which transformer the source initializes first.
	lowerGoal, upperGoal := number(23), number(29)
	z := own(m.MkVar(0, real))
	gtGoal := own(m.MkArithmeticGT(z, lowerGoal))
	ltGoal := own(m.MkArithmeticLT(z, upperGoal))
	add(px, []ast.ASTHandle{gt, lt}, "interval")
	add(goal, []ast.ASTHandle{p0, p1, gtGoal, ltGoal}, "use")
	must(c.Initialize(set))
	status, err := c.Solve(g)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	root, found, err := c.search.Root()
	must(err)
	if !found {
		t.Fatal("missing root")
	}
	nodes := []*ModelNode{root}
	nodes = append(nodes, root.children...)
	if len(nodes) != 3 {
		t.Fatal("source trace lost its two nonlinear predecessors", len(nodes))
	}
	child := nodes[1]
	childModel, ok, err := child.Model()
	must(err)
	rootModel, rootOK, err := root.Model()
	must(err)
	if !ok || !rootOK || childModel == nil || rootModel == nil {
		t.Fatal("source trace is missing its solved models", ok, rootOK)
	}
	if childModel.References != 1 || rootModel.References != 1 {
		t.Fatal("trace models are not owned solely by search nodes", childModel.References, rootModel.References)
	}
	exclusiveLarge := func(node *ModelNode) ast.ASTHandle {
		t.Helper()
		md, found, e := node.Model()
		must(e)
		if !found {
			t.Fatal("missing solved node model")
		}
		for _, d := range md.ConstantDeclarations {
			v, e := md.ConstantInterpretation(d)
			must(e)
			text, _, matched, e := m.ArithmeticNumeralText(v)
			must(e)
			if matched && len(text) > 1000 && v != lower && v != upper && v != lowerGoal && v != upperGoal {
				rc, e := m.GetRefCount(v)
				must(e)
				if rc == 1 {
					return v
				}
			}
		}
		t.Fatal("solver did not produce a model-only large numeric AST")
		return ast.ASTHandle{}
	}
	childValue, rootValue := exclusiveLarge(child), exclusiveLarge(root)
	if childValue == rootValue {
		t.Fatal("source trace unexpectedly shares child and root assignments")
	}
	initialized, err := c.Initialized()
	must(err)
	if !initialized {
		t.Fatal("fixture did not install the actual rules")
	}

	a, err := rt.Allocate(8)
	must(err)
	b, err := rt.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = rt.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("TLS1", err)
	}
	must(rt.Deallocate(&a))
	delta = math.MinInt64 + 1000 - rt.State().GlobalSize + 16
	if err = rt.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("TLS2", err)
	}
	must(rt.Deallocate(&b))
	padding, err := rt.Allocate(8)
	must(err)
	delta = -100000
	if err = rt.Reallocate(&padding, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("TLS3", err)
	}
	must(rt.Deallocate(&padding))
	if st := rt.State(); st.GlobalSize != math.MinInt64+1000 || st.LocalSize != -100000 {
		t.Fatal("bad accounting", st)
	}
	// The later child shares its model with the last solver result. Releasing
	// that reference succeeds; the earlier child has the first exclusive model.
	// Its destructor consumes a numeric block before synchronization refuses.
	before := rt.Stats().LiveBlocks
	var cause error
	if replacingModel {
		cause = root.SetModel(childModel)
	} else if replacing {
		status, cause = c.Solve(g)
		if status != lbool.Undef {
			t.Fatal("failed root replacement published a result", status, cause)
		}
		last, lastErr := c.LastResult()
		must(lastErr)
		if last != lbool.Undef {
			t.Fatal("failed root replacement left a completed result", last)
		}
	} else {
		cause = c.Reset()
	}
	if !errors.Is(cause, arena.ErrBounds) || rt.Stats().LiveBlocks+1 != before {
		t.Fatal("operation missed the consumed model cleanup failure", cause, before, rt.Stats().LiveBlocks)
	}
	if replacingModel {
		if c.search.root != root || root.model != rootModel || rootModel.References != 0 || rootModel.Closed || root.pendingModel != childModel || childModel.References != 2 || child.model != childModel {
			t.Fatal("failed replacement changed source publication order or lost a counted owner", root.model == rootModel, rootModel.References, rootModel.Closed, root.pendingModel == childModel, childModel.References)
		}
		if _, _, err = root.Model(); !errors.Is(err, ast.ErrASTPending) {
			t.Fatal("partly replaced root remained usable", err)
		}
		if err = root.SetModel(nil); !errors.Is(err, ast.ErrASTPending) {
			t.Fatal("interrupted replacement accepted another model mutation", err)
		}
		if _, err = m.GetRefCount(childValue); err != nil {
			t.Fatal("failed replacement disturbed the staged incoming payload", err)
		}
	} else {
		if c.search.root != root || child.model != childModel || childModel.References != 0 || childModel.Closed || rootModel.References != 1 {
			t.Fatal("child failure discarded a model or later root", child.model == childModel, childModel.References, childModel.Closed, rootModel.References)
		}
		pendingChild := false
		for _, node := range c.search.pendingRelease {
			pendingChild = pendingChild || node == child
		}
		if !pendingChild {
			t.Fatal("search lost its pending child release")
		}
	}
	if !replacingModel {
		if _, _, err = c.search.Root(); !errors.Is(err, ast.ErrASTPending) {
			t.Fatal("interrupted search remained observable", err)
		}
		if _, _, err = child.Model(); !errors.Is(err, ast.ErrASTPending) {
			t.Fatal("partly released child remained usable", err)
		}
		if _, err = m.GetRefCount(rootValue); err != nil {
			t.Fatal("child failure disturbed later root payload", err)
		}
	}
	// Retry without repairing the allocator. Cleanup must use Close for the
	// consumed reference. The reset/solve cases then reach the root payload;
	// the replacement case reaches the staged incoming model payload.
	before = rt.Stats().LiveBlocks
	second := c.Reset()
	if !errors.Is(second, arena.ErrBounds) || rt.Stats().LiveBlocks+1 != before {
		t.Fatal("retry missed the consumed root model failure", second, before, rt.Stats().LiveBlocks)
	}
	if replacingModel {
		if !rootModel.Closed || child.model != nil || c.search.root != root || root.model != childModel || root.pendingModel != nil || childModel.References != 0 || childModel.Closed {
			t.Fatal("reset lost staged replacement cleanup or decremented a consumed owner", rootModel.Closed, child.model == nil, root.model == childModel, root.pendingModel == nil, childModel.References, childModel.Closed)
		}
	} else {
		if !childModel.Closed || child.model != nil || c.search.root != root || root.model != rootModel || rootModel.References != 0 || rootModel.Closed {
			t.Fatal("retry lost a root or decremented an already consumed model", childModel.Closed, child.model == nil, root.model == rootModel, rootModel.References, rootModel.Closed)
		}
	}
	if len(c.search.pendingRelease) != 0 || !c.search.rootRemoved {
		t.Fatal("root failure lost its source removal phase", len(c.search.pendingRelease), c.search.rootRemoved)
	}
	if _, _, err = root.Model(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("partly released root remained usable", err)
	}
	// Repair only real allocator accounting. Every model continuation remains
	// owned by Context.Reset; no saved model pointer is used for cleanup.
	recovery, err = rt.Allocate(400000)
	must(err)
	must(c.Reset())
	must(c.Reset())
	if c.search.root != nil || len(c.search.pendingRelease) != 0 || c.search.rootRemoved || !rootModel.Closed || !childModel.Closed || root.model != nil {
		t.Fatal("public reset did not drain its model owners")
	}
	initialized, err = c.Initialized()
	must(err)
	if initialized {
		t.Fatal("reset left the program initialized")
	}
	for _, value := range []ast.ASTHandle{childValue, rootValue} {
		rc, e := m.GetRefCount(value)
		if e == nil && rc != 0 {
			t.Fatal("reset left a model-only numeric value live", rc)
		}
		if e != nil && !errors.Is(e, ast.ErrASTHandle) {
			t.Fatal("unexpected numeric retirement result", e)
		}
	}
	// The caller's rules survive the failed cleanup and install into the same
	// context again; the actual Horn query must still reach both predecessors.
	must(c.Initialize(set))
	status, err = c.Solve(g)
	must(err)
	if status != lbool.True {
		t.Fatal("reinitialized source program changed result", status)
	}
	if replacingModel {
		fresh, found, getErr := c.search.Root()
		must(getErr)
		if !found || len(fresh.children) != 2 {
			t.Fatal("reinitialized trace lost its actual model owners")
		}
		prior, incoming := fresh.model, fresh.children[0].model
		if prior == nil || incoming == nil || prior == incoming || prior.References != 1 || incoming.References != 1 {
			t.Fatal("unexpected solved model owners before successful replacement")
		}
		must(fresh.SetModel(incoming))
		if !prior.Closed || fresh.model != incoming || incoming.References != 2 {
			t.Fatal("successful replacement lost its source model references")
		}
		must(fresh.SetModel(incoming))
		if incoming.References != 2 {
			t.Fatal("self replacement changed its reference count", incoming.References)
		}
		must(fresh.SetModel(nil))
		current, found, getErr := fresh.Model()
		must(getErr)
		if current != nil || found || incoming.References != 1 || incoming.Closed {
			t.Fatal("nil replacement changed the independent child owner")
		}
	}
	must(c.Reset())
	must(c.Close())
	must(c.Close())
}
