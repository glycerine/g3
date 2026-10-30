package qe

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type conjunctionPartitionPlugin struct {
	family        ast.FamilyID
	uninterpreted bool
}

func (p *conjunctionPartitionPlugin) FamilyID() ast.FamilyID { return p.family }
func (p *conjunctionPartitionPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	return ast.ASTHandle{}, false, nil
}
func (p *conjunctionPartitionPlugin) IsUninterpreted(ast.ASTHandle) (bool, error) {
	return p.uninterpreted, nil
}
func (p *conjunctionPartitionPlugin) Close() error { return nil }

func partitionTerms(t *testing.T, manager *ast.ASTManager, ref ast.ASTRef) map[ast.ASTHandle]bool {
	t.Helper()
	h, err := manager.RefGet(ref)
	fmMust(t, err)
	terms, err := manager.FlattenAnd([]ast.ASTHandle{h})
	fmMust(t, err)
	result := make(map[ast.ASTHandle]bool, len(terms))
	for _, term := range terms {
		result[term] = true
	}
	return result
}

func TestConjunctionPartitionSourceClassification(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("partition-x", f.integer)
	zero := f.number("0")
	closedTerm := f.constant("partition-closed", f.boolean)
	pure := f.own(m.MkArithmeticLE(x, zero))
	mixed := f.predicate("partition-mixed", x)
	formula := f.own(m.MkAndArray([]ast.ASTHandle{closedTerm, pure, mixed}))

	partitioner, err := NewConjunctionPartitioner(m)
	fmMust(t, err)
	plugin := &conjunctionPartitionPlugin{family: f.r.arithmetic}
	fmMust(t, partitioner.AddPlugin(plugin))
	if err := partitioner.AddPlugin(plugin); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("duplicate plugin error = %v", err)
	}
	closed, mixedPart, open, err := partitioner.GetPartition(formula, []ast.ASTHandle{x})
	fmMust(t, err)
	if got := partitionTerms(t, m, closed); !reflect.DeepEqual(got, map[ast.ASTHandle]bool{closedTerm: true}) {
		t.Fatalf("closed partition = %v", got)
	}
	if got := partitionTerms(t, m, mixedPart); !reflect.DeepEqual(got, map[ast.ASTHandle]bool{mixed: true}) {
		t.Fatalf("mixed partition = %v", got)
	}
	if got := partitionTerms(t, m, open); !reflect.DeepEqual(got, map[ast.ASTHandle]bool{pure: true, mixed: true}) {
		t.Fatalf("open partition = %v", got)
	}
	for _, ref := range []*ast.ASTRef{&open, &mixedPart, &closed} {
		fmMust(t, m.DelRef(ref))
	}
	fmMust(t, partitioner.Close())
	if _, _, _, err = partitioner.GetPartition(formula, nil); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed partitioner error = %v", err)
	}
}

func TestConjunctionPartitionMajorityAndUnionOrdering(t *testing.T) {
	f := newFMFixture(t)
	x := f.constant("partition-vx", f.integer)
	y := f.constant("partition-vy", f.integer)
	z := f.constant("partition-vz", f.integer)
	w := f.constant("partition-vw", f.integer)
	variables := []ast.ASTHandle{x, y, z, w}
	contains := make([]*ast.ContainsApp, len(variables))
	for i, variable := range variables {
		var err error
		contains[i], err = ast.NewContainsApp(f.r.manager, variable)
		fmMust(t, err)
	}
	defer func() {
		for i := len(contains) - 1; i >= 0; i-- {
			fmMust(t, contains[i].Close())
		}
	}()
	conjuncts := []ast.ASTHandle{
		f.predicate("partition-c0", x, y, w),
		f.predicate("partition-c1", x, y, w),
		f.predicate("partition-c2", z, w),
		f.constant("partition-c3", f.boolean),
		f.constant("partition-c4", f.boolean),
	}
	partitioner, err := NewConjunctionPartitioner(f.r.manager)
	fmMust(t, err)
	partition, yes, err := partitioner.PartitionVariables(contains, conjuncts)
	fmMust(t, err)
	if !yes || !reflect.DeepEqual(partition, [][]uint32{{3}, {0, 1}, {2}}) {
		t.Fatalf("partition = %v, %v", partition, yes)
	}

	// Two occurrences out of four are not a strict majority. The first two
	// variables therefore remain in union-find and preserve source order.
	half, yes, err := partitioner.PartitionVariables(contains[:3], conjuncts[:4])
	fmMust(t, err)
	if !yes || !reflect.DeepEqual(half, [][]uint32{{0, 1}, {2}}) {
		t.Fatalf("half partition = %v, %v", half, yes)
	}
	one, yes, err := partitioner.PartitionVariables(contains[:2], conjuncts[:2])
	fmMust(t, err)
	if yes || !reflect.DeepEqual(one, [][]uint32{{0, 1}}) {
		t.Fatalf("single component = %v, %v", one, yes)
	}
	fmMust(t, partitioner.Close())
}

func declarationOf(t *testing.T, manager *ast.ASTManager, application ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	declaration, err := manager.GetDecl(application)
	fmMust(t, err)
	return declaration
}

func TestDefinitionVectorNormalizeProjectAndOwnership(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("definition-x", f.integer)
	y := f.constant("definition-y", f.integer)
	z := f.constant("definition-z", f.integer)
	two := f.number("2")
	dx, dy, dz := declarationOf(t, m, x), declarationOf(t, m, y), declarationOf(t, m, z)

	definitions, err := NewDefinitionVector(m, f.r.config)
	fmMust(t, err)
	fmMust(t, definitions.PushBack(dx, y))
	fmMust(t, definitions.PushBack(dy, two))
	fmMust(t, definitions.Normalize())
	for i := uint32(0); i < 2; i++ {
		got, cause := definitions.Definition(i)
		fmMust(t, cause)
		if got != two {
			t.Fatalf("normalized definition %d = %v, want %v", i, got, two)
		}
	}
	clone, err := definitions.Clone()
	fmMust(t, err)
	fmMust(t, definitions.Reset())
	if n, err := clone.Size(); err != nil || n != 2 {
		t.Fatalf("clone after original reset = %d, %v", n, err)
	}
	fmMust(t, definitions.Close())
	fmMust(t, clone.Close())

	projected, err := NewDefinitionVector(m, f.r.config)
	fmMust(t, err)
	fmMust(t, projected.PushBack(dx, y))
	fmMust(t, projected.PushBack(dx, two))
	fmMust(t, projected.PushBack(dy, x))
	fmMust(t, projected.PushBack(dz, two))
	fmMust(t, projected.Project([]ast.ASTHandle{x, z}))
	if n, err := projected.Size(); err != nil || n != 2 {
		t.Fatalf("projected size = %d, %v", n, err)
	}
	for i, want := range []ast.ASTHandle{dx, dz} {
		got, cause := projected.Variable(uint32(i))
		fmMust(t, cause)
		if got != want {
			t.Fatalf("projected variable %d = %v, want %v", i, got, want)
		}
	}
	if got, err := projected.Definition(0); err != nil || got != y {
		t.Fatalf("first recycled definition = %v, %v", got, err)
	}
	if err := projected.PushBack(x, two); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("application as declaration error = %v", err)
	}
	copyValue := *projected
	if _, err := copyValue.Size(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("copied owner error = %v", err)
	}
	fmMust(t, projected.Close())
	if _, err := projected.Size(); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed definitions error = %v", err)
	}
}

func TestGuardedDefinitionsOwnsNormalizedCopies(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("guarded-x", f.integer)
	y := f.constant("guarded-y", f.integer)
	two := f.number("2")
	guard := f.constant("guarded-condition", f.boolean)
	original, err := NewDefinitionVector(m, f.r.config)
	fmMust(t, err)
	fmMust(t, original.PushBack(declarationOf(t, m, x), y))
	fmMust(t, original.PushBack(declarationOf(t, m, y), two))
	guarded, err := NewGuardedDefinitions(m, f.r.config)
	fmMust(t, err)
	fmMust(t, guarded.Add(guard, original))
	fmMust(t, original.Reset())
	fmMust(t, original.Close())
	if n, err := guarded.Size(); err != nil || n != 1 {
		t.Fatalf("guarded size = %d, %v", n, err)
	}
	if got, err := guarded.Guard(0); err != nil || got != guard {
		t.Fatalf("stored guard = %v, %v", got, err)
	}
	for i := uint32(0); i < 2; i++ {
		got, cause := guarded.Definition(0, i)
		fmMust(t, cause)
		if got != two {
			t.Fatalf("stored normalized definition %d = %v", i, got)
		}
	}
	fmMust(t, guarded.Project([]ast.ASTHandle{x}))
	if n, err := guarded.DefinitionSize(0); err != nil || n != 1 {
		t.Fatalf("projected guarded size = %d, %v", n, err)
	}
	fmMust(t, guarded.Close())
	if _, err := guarded.Size(); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed guarded definitions error = %v", err)
	}
}

func TestBoolPluginBranchAndSubstitutionContracts(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("bool-plugin-x", f.boolean)
	p := f.constant("bool-plugin-p", f.boolean)
	formula := f.own(m.MkAndArray([]ast.ASTHandle{x, p}))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	context, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	plugin, err := NewBoolPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	if plugin.FamilyID() != m.BasicFamilyID() {
		t.Fatalf("Boolean family = %d, want %d", plugin.FamilyID(), m.BasicFamilyID())
	}
	if uninterpreted, err := plugin.IsUninterpreted(x); err != nil || uninterpreted {
		t.Fatalf("Boolean uninterpreted = %v, %v", uninterpreted, err)
	}
	if _, handled, err := plugin.MkAtom(x, true); err != nil || handled {
		t.Fatalf("default atom = %v, %v", handled, err)
	}

	service := m.NumeralService()
	branches, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.Del(&branches)) }()
	if handled, err := plugin.GetNumBranches(contains, formula, &branches); err != nil || !handled {
		t.Fatalf("branches handled = %v, %v", handled, err)
	}
	if two, err := service.EqualInt(&branches, 2); err != nil || !two {
		t.Fatalf("branch count is two = %v, %v", two, err)
	}
	zero, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.Del(&zero)) }()
	fmMust(t, service.SetUnsigned(&zero, 0))
	one, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.Del(&one)) }()
	fmMust(t, service.SetUnsigned(&one, 1))
	half, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.Del(&half)) }()
	fmMust(t, service.SetFraction(&half, 1, 2))
	fmMust(t, plugin.Assign(contains, formula, &zero))
	fmMust(t, plugin.Assign(contains, formula, &one))
	if err := plugin.Assign(contains, formula, &half); err != nil {
		t.Fatalf("source release assign ignores its assertion-only branch value: %v", err)
	}

	formulaRef, err := m.NewRef(formula)
	fmMust(t, err)
	definition, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	fmMust(t, plugin.Substitute(contains, &zero, formulaRef, &definition))
	rewritten, err := m.RefGet(formulaRef)
	fmMust(t, err)
	stillContains, err := contains.Check(rewritten)
	fmMust(t, err)
	if stillContains {
		t.Fatal("Boolean substitution retained eliminated variable")
	}
	defined, err := m.RefGet(definition)
	fmMust(t, err)
	if !m.IsFalse(defined) {
		t.Fatalf("Boolean definition = %v, want false", defined)
	}
	fmMust(t, m.DelRef(&definition))
	fmMust(t, m.DelRef(&formulaRef))
	formulaRef, err = m.NewRef(formula)
	fmMust(t, err)
	definition, err = m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	fmMust(t, plugin.Substitute(contains, &half, formulaRef, &definition))
	defined, err = m.RefGet(definition)
	fmMust(t, err)
	if !m.IsFalse(defined) {
		t.Fatal("source release substitution must map non-one values to false")
	}
	fmMust(t, m.DelRef(&definition))
	fmMust(t, m.DelRef(&formulaRef))
	fmMust(t, context.Close())
	if err := plugin.Assign(contains, formula, &zero); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed Boolean plugin error = %v", err)
	}

	// Ensure the concrete type remains the SolverPlugin contract consumed by
	// SolverContext, without adding placeholder solve or model behavior.
	var _ SolverPlugin = (*BoolPlugin)(nil)
}
