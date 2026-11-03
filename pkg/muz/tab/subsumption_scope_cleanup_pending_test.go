package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

type subsumptionScopeRefusalSolver struct {
	solver.Solver
	pushBefore, pushAfter map[uint32]error
	popBefore, popAfter   map[uint32]error
	pushes, pops          uint32
	popLevels             []uint32
}

func (s *subsumptionScopeRefusalSolver) Push() error {
	s.pushes++
	if err := s.pushBefore[s.pushes]; err != nil {
		return err
	}
	if err := s.Solver.Push(); err != nil {
		return err
	}
	return s.pushAfter[s.pushes]
}

func (s *subsumptionScopeRefusalSolver) Pop(count uint32) error {
	s.pops++
	level, err := s.Solver.ScopeLevel()
	if err != nil {
		return err
	}
	s.popLevels = append(s.popLevels, level)
	if err = s.popBefore[s.pops]; err != nil {
		return err
	}
	if err = s.Solver.Pop(count); err != nil {
		return err
	}
	return s.popAfter[s.pops]
}

func TestSubsumptionValidationOwnsPartiallyPublishedSolverScope(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m := ctx.manager
	rw, err := rewriter.NewTheoryRewriter(m, ctx.params, ctx.globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	coop, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(coop.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: ctx.params, Globals: ctx.globals, Cooperation: coop})
	must(err)
	defer func() { must(lite.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	name, err := m.Symbols.Intern("subsumption-scope-value")
	must(err)
	a, err := m.MkConst(name, integer)
	must(err)
	name, err = m.Symbols.Intern("subsumption-scope-head")
	must(err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	head, err := m.MkApp(decl, []ast.ASTHandle{zero})
	must(err)
	family, err := m.ArithmeticFamilyID()
	must(err)
	compare := func(op int32, left, right ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{left, right})
		must(e)
		return h
	}
	candidate, err := NewClause(m, rw, head, nil, compare(2, x, two))
	must(err)
	defer func() { must(candidate.Close()) }()
	bindings, err := substitution.New(m)
	must(err)
	defer func() { must(bindings.Close()) }()
	must(bindings.Reserve(2, 1))
	must(bindings.Insert(0, 0, substitution.NewExprOffset(a, 1)))

	newSolver := func() solver.Solver {
		smt, e := ctx.NewSimpleSolver()
		must(e)
		must(smt.Assert(compare(2, a, one)))
		must(smt.Assert(compare(3, a, one)))
		return smt
	}
	level := func(s solver.Solver) uint32 {
		value, e := s.ScopeLevel()
		must(e)
		return value
	}

	t.Run("push-before-publication", func(t *testing.T) {
		native := newSolver()
		defer func() { must(native.Close()) }()
		refused := errors.New("subsumption push refused before publication")
		wrapped := &subsumptionScopeRefusalSolver{Solver: native, pushBefore: map[uint32]error{1: refused}}
		valid, failure := validateSubsumption(m, candidate, bindings, nil, lite, wrapped, nil, nil)
		if valid || !errors.Is(failure, refused) || level(native) != 0 || wrapped.pops != 0 {
			t.Fatal("pre-publication push changed scope or result", valid, failure, level(native), wrapped.pops)
		}
	})

	t.Run("push-after-publication", func(t *testing.T) {
		native := newSolver()
		defer func() { must(native.Close()) }()
		refused := errors.New("subsumption push refused after publication")
		wrapped := &subsumptionScopeRefusalSolver{Solver: native, pushAfter: map[uint32]error{1: refused}}
		valid, failure := validateSubsumption(m, candidate, bindings, nil, lite, wrapped, nil, nil)
		if valid || !errors.Is(failure, refused) || level(native) != 0 || wrapped.pops != 1 {
			t.Fatal("published push was not reconciled", valid, failure, level(native), wrapped.pops)
		}
	})

	t.Run("pop-refusal", func(t *testing.T) {
		native := newSolver()
		defer func() { must(native.Close()) }()
		refused := errors.New("subsumption validation pop refused")
		wrapped := &subsumptionScopeRefusalSolver{Solver: native, popBefore: map[uint32]error{1: refused}}
		valid, failure := validateSubsumption(m, candidate, bindings, nil, lite, wrapped, nil, nil)
		if valid || !errors.Is(failure, refused) || level(native) != 1 || wrapped.pops != 1 {
			t.Fatal("pop refusal published validation or lost its scope", valid, failure, level(native), wrapped.pops)
		}
		var receipt *subsumptionScopeFailure
		if !errors.As(failure, &receipt) || !receipt.attempt.scope.pushed || receipt.attempt.post == (ast.ASTRef{}) || len(receipt.attempt.forms) == 0 {
			t.Fatal("pop refusal did not expose a retry owner", failure)
		}
		original, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
		must(receipt.Cleanup())
		must(receipt.Cleanup())
		if level(native) != 0 || wrapped.pops != 2 || receipt.attempt.scope.pushed || receipt.attempt.post != (ast.ASTRef{}) || len(receipt.attempt.forms) != 0 || receipt.CleanupCause() != nil {
			t.Fatal("pop cleanup did not drain its scope and formula owners", level(native), wrapped.pops, receipt.attempt.scope.pushed, receipt.attempt.post, len(receipt.attempt.forms), receipt.CleanupCause())
		}
		if receipt.OriginalCause() != original || errors.Unwrap(receipt) != exposed || receipt.Error() != message {
			t.Fatal("pop cleanup changed its evidence")
		}
		valid, err = validateSubsumption(m, candidate, bindings, nil, lite, wrapped, nil, nil)
		if err != nil || !valid || level(native) != 0 {
			t.Fatal("validation solver was not reusable", valid, err, level(native))
		}
	})

	t.Run("pop-consumed-before-error", func(t *testing.T) {
		native := newSolver()
		defer func() { must(native.Close()) }()
		refused := errors.New("subsumption validation pop reported late failure")
		wrapped := &subsumptionScopeRefusalSolver{Solver: native, popAfter: map[uint32]error{1: refused}}
		valid, failure := validateSubsumption(m, candidate, bindings, nil, lite, wrapped, nil, nil)
		if valid || !errors.Is(failure, refused) || level(native) != 0 || wrapped.pops != 1 {
			t.Fatal("late pop failure published validation", valid, failure, level(native), wrapped.pops)
		}
		var receipt *subsumptionScopeFailure
		if !errors.As(failure, &receipt) {
			t.Fatal("late pop failure did not expose a retry owner", failure)
		}
		if receipt.attempt.post == (ast.ASTRef{}) || len(receipt.attempt.forms) == 0 {
			t.Fatal("late pop failure released validation formula owners")
		}
		must(receipt.Cleanup())
		must(receipt.Cleanup())
		if level(native) != 0 || wrapped.pops != 1 || receipt.attempt.scope.pushed || receipt.attempt.post != (ast.ASTRef{}) || len(receipt.attempt.forms) != 0 {
			t.Fatal("late pop failure was popped twice or kept owners", level(native), wrapped.pops, receipt.attempt.scope.pushed, receipt.attempt.post, len(receipt.attempt.forms))
		}
	})
}

func TestClauseSubsumptionDrainsNestedScopeBeforeOuterScope(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m := ctx.manager
	rw, err := rewriter.NewTheoryRewriter(m, ctx.params, ctx.globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	coop, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(coop.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: ctx.params, Globals: ctx.globals, Cooperation: coop})
	must(err)
	defer func() { must(lite.Close()) }()
	native, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(native.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	constant := func(text string) ast.ASTHandle {
		name, e := m.Symbols.Intern(text)
		must(e)
		h, e := m.MkConst(name, integer)
		must(e)
		return h
	}
	a := constant("nested-scope-a")
	declaration := func(text string) ast.ASTHandle {
		name, e := m.Symbols.Intern(text)
		must(e)
		d, e := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		return d
	}
	headDecl, bodyDecl := declaration("nested-scope-head"), declaration("nested-scope-body")
	app := func(decl, value ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(decl, []ast.ASTHandle{value})
		must(e)
		return h
	}
	family, err := m.ArithmeticFamilyID()
	must(err)
	compare := func(op int32, left, right ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{left, right})
		must(e)
		return h
	}
	precondition, err := m.MkAnd(compare(2, a, one), compare(3, a, one))
	must(err)
	head := app(headDecl, zero)
	query, err := NewClause(m, rw, head, []ast.ASTHandle{app(bodyDecl, a)}, precondition)
	must(err)
	defer func() { must(query.Close()) }()
	x, err := m.MkVar(0, integer)
	must(err)
	candidate, err := NewClause(m, rw, head, []ast.ASTHandle{app(bodyDecl, x)}, compare(2, x, two))
	must(err)
	candidate.sequence = 73
	defer func() { must(candidate.Close()) }()

	// Preserve a caller-owned scope to prove cleanup restores its saved depth,
	// rather than assuming every solver starts at zero.
	must(native.Push())
	innerRefused := errors.New("inner subsumption pop refused")
	innerRetryRefused := errors.New("inner subsumption retry pop refused")
	outerRefused := errors.New("outer subsumption pop refused")
	wrapped := &subsumptionScopeRefusalSolver{Solver: native, popBefore: map[uint32]error{1: innerRefused, 2: innerRetryRefused, 4: outerRefused}}
	found, sequence, failure := isClauseSubsumed(query, []*Clause{candidate}, lite, wrapped)
	level, err := native.ScopeLevel()
	must(err)
	if found || sequence != 0 || !errors.Is(failure, innerRefused) || !errors.Is(failure, innerRetryRefused) || level != 3 {
		t.Fatal("nested cleanup published a match or lost its frontier", found, sequence, failure, level)
	}
	if wrapped.pops != 2 || len(wrapped.popLevels) != 2 || wrapped.popLevels[0] != 3 || wrapped.popLevels[1] != 3 {
		t.Fatal("inner scope was not drained before the outer scope", wrapped.pops, wrapped.popLevels)
	}
	var receipt *subsumptionScopeFailure
	if !errors.As(failure, &receipt) || receipt.attempt.scope.target != 1 || !receipt.attempt.scope.pushed || receipt.attempt.nested == nil || receipt.attempt.grounded == nil || receipt.attempt.bindings == nil || receipt.attempt.search == nil {
		t.Fatal("outer cleanup did not preserve its exact scope", failure)
	}
	original, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
	if err = receipt.Cleanup(); !errors.Is(err, outerRefused) {
		t.Fatal("outer pop refusal was not reached after draining the inner scope", err)
	}
	level, err = native.ScopeLevel()
	must(err)
	if level != 2 || wrapped.pops != 4 || receipt.attempt.nested != nil || !receipt.attempt.scope.pushed || receipt.attempt.grounded == nil || receipt.attempt.bindings == nil {
		t.Fatal("outer refusal lost its own scope or owners", level, wrapped.pops, receipt.attempt.nested, receipt.attempt.scope.pushed)
	}
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	level, err = native.ScopeLevel()
	must(err)
	if level != 1 || wrapped.pops != 5 || receipt.attempt.scope.pushed || receipt.attempt.grounded != nil || receipt.attempt.bindings != nil || receipt.attempt.search != nil || receipt.CleanupCause() != nil {
		t.Fatal("outer cleanup did not restore the caller scope and owners", level, wrapped.pops, receipt.attempt.scope.pushed, receipt.CleanupCause())
	}
	if receipt.OriginalCause() != original || errors.Unwrap(receipt) != exposed || receipt.Error() != message {
		t.Fatal("outer cleanup changed its evidence")
	}

	found, sequence, err = isClauseSubsumed(query, []*Clause{candidate}, lite, wrapped)
	if err != nil || !found || sequence != 73 {
		t.Fatal("subsumption solver was not reusable", found, sequence, err)
	}
	level, err = native.ScopeLevel()
	must(err)
	if level != 1 {
		t.Fatal("successful retry consumed the caller scope", level)
	}
	must(native.Pop(1))
}

func TestClauseSubsumptionSuppressesMatchOnOuterPopRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m := ctx.manager
	rw, err := rewriter.NewTheoryRewriter(m, ctx.params, ctx.globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	coop, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(coop.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: ctx.params, Globals: ctx.globals, Cooperation: coop})
	must(err)
	defer func() { must(lite.Close()) }()
	native, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(native.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("outer-scope-head")
	must(err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	head, err := m.MkApp(decl, []ast.ASTHandle{zero})
	must(err)
	truth, err := m.MkTrue()
	must(err)
	query, err := NewClause(m, rw, head, nil, truth)
	must(err)
	defer func() { must(query.Close()) }()
	candidate, err := NewClause(m, rw, head, nil, truth)
	must(err)
	candidate.sequence = 19
	defer func() { must(candidate.Close()) }()
	refused := errors.New("outer subsumption pop refused")
	wrapped := &subsumptionScopeRefusalSolver{Solver: native, popBefore: map[uint32]error{1: refused}}
	found, sequence, failure := isClauseSubsumed(query, []*Clause{candidate}, lite, wrapped)
	level, err := native.ScopeLevel()
	must(err)
	if found || sequence != 0 || !errors.Is(failure, refused) || level != 1 || wrapped.pushes != 1 || wrapped.pops != 1 {
		t.Fatal("outer pop refusal published a match", found, sequence, failure, level, wrapped.pushes, wrapped.pops)
	}
	var receipt *subsumptionScopeFailure
	if !errors.As(failure, &receipt) || receipt.attempt.grounded == nil || receipt.attempt.bindings == nil || receipt.attempt.search == nil {
		t.Fatal("outer pop refusal did not expose a retry owner", failure)
	}
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	level, err = native.ScopeLevel()
	must(err)
	if level != 0 || wrapped.pops != 2 || receipt.attempt.grounded != nil || receipt.attempt.bindings != nil || receipt.attempt.search != nil {
		t.Fatal("outer scope cleanup was not exact", level, wrapped.pops, receipt.attempt.grounded, receipt.attempt.bindings)
	}
	found, sequence, err = isClauseSubsumed(query, []*Clause{candidate}, lite, wrapped)
	if err != nil || !found || sequence != 19 || wrapped.pushes != 2 || wrapped.pops != 3 {
		t.Fatal("outer solver was not reusable", found, sequence, err, wrapped.pushes, wrapped.pops)
	}
}
