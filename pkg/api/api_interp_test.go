package api

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type interpolationFixture struct {
	t *testing.T
	c *InterpolationContext
	m *ast.ASTManager
	w *interp.Manager
}

func newInterpolationFixture(t *testing.T) *interpolationFixture {
	t.Helper()
	c, err := NewInterpolationContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
		if used := c.budget.Used(); used != 0 {
			t.Errorf("context retained %d bytes", used)
		}
	})
	return &interpolationFixture{t, c, c.manager, c.terms}
}

func (f *interpolationFixture) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	ref, err := f.m.NewRef(h)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() {
		if err := f.m.DelRef(&ref); err != nil {
			f.t.Error(err)
		}
	})
	return h
}

func (f *interpolationFixture) variable(name string, sort ast.ASTHandle) ast.ASTHandle {
	return f.pin(f.w.MakeVariable(name, sort))
}

func (f *interpolationFixture) make(op interp.Operator, args ...ast.ASTHandle) ast.ASTHandle {
	return f.pin(f.w.Make(op, args...))
}

func (f *interpolationFixture) verify(pattern ast.ASTHandle, results []ast.ASTHandle) {
	f.t.Helper()
	constraints, err := interp.CollectConjuncts(f.w, pattern)
	if err != nil {
		f.t.Fatal(err)
	}
	s, err := f.c.factory.New(f.m, nil, true, false, false, "")
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			f.t.Error(err)
		}
	}()
	var diagnostics bytes.Buffer
	valid, err := interp.CheckTreeInterpolants(f.w, s, &diagnostics, constraints, pattern, results)
	if err != nil || !valid {
		for _, h := range results {
			_ = f.w.PrintExpr(&diagnostics, h)
		}
		f.t.Fatalf("invalid interpolant: %v: %s", err, diagnostics.String())
	}
	if len(results) == 1 && len(constraints) == 2 {
		// The binary contract also excludes private functions. Source tree
		// checking treats positive-arity functions as globally scoped.
		valid, err = interp.CheckInterpolants(f.w, s, &diagnostics, constraints, nil, results, nil)
		if err != nil || !valid {
			f.t.Fatalf("invalid binary vocabulary: %v: %s", err, diagnostics.String())
		}
		valid, message, err := f.c.CheckInterpolants(constraints, nil, results, nil)
		if err != nil || !valid || message != "" {
			f.t.Fatalf("public interpolation check: %v valid=%v: %s", err, valid, message)
		}
	}
}

func TestInterpolationContextBinary(t *testing.T) {
	for _, name := range []string{"boolean", "integer", "local-integer", "integer-cut", "euf", "private-euf", "array", "array-extensionality", "quantifier", "local-false"} {
		t.Run(name, func(t *testing.T) {
			f := newInterpolationFixture(t)
			boolean := f.pin(f.w.BoolType())
			integer := f.pin(f.w.IntType())
			p := f.variable("p", boolean)
			x := f.variable("x", integer)
			y := f.variable("y", integer)
			zero := f.pin(f.w.MakeIntText("0"))
			two := f.pin(f.w.MakeIntText("2"))
			a, b := p, f.make(interp.OperatorNot, p)
			switch name {
			case "integer":
				a = f.make(interp.OperatorLT, x, zero)
				b = f.make(interp.OperatorGT, x, two)
			case "local-integer":
				a = f.make(interp.OperatorAnd, f.make(interp.OperatorEqual, y, f.make(interp.OperatorPlus, x, two)), f.make(interp.OperatorLE, y, zero))
				b = f.make(interp.OperatorGE, x, zero)
			case "integer-cut":
				a = f.make(interp.OperatorEqual, x, f.make(interp.OperatorTimes, two, y))
				one := f.pin(f.w.MakeIntText("1"))
				b = f.make(interp.OperatorEqual, x, one)
			case "euf":
				u := f.pin(f.w.Function("f", []ast.ASTHandle{integer}, integer))
				fx := f.pin(f.m.MkApp(u, []ast.ASTHandle{x}))
				fy := f.pin(f.m.MkApp(u, []ast.ASTHandle{y}))
				a = f.make(interp.OperatorEqual, x, y)
				b = f.make(interp.OperatorNot, f.make(interp.OperatorEqual, fx, fy))
			case "private-euf":
				u := f.pin(f.w.Function("private-f", []ast.ASTHandle{integer}, integer))
				z := f.variable("private-z", integer)
				fz := f.pin(f.m.MkApp(u, []ast.ASTHandle{z}))
				a = f.make(interp.OperatorAnd, f.make(interp.OperatorEqual, x, fz), f.make(interp.OperatorEqual, y, fz))
				b = f.make(interp.OperatorNot, f.make(interp.OperatorEqual, x, y))
			case "array":
				array := f.pin(f.w.ArrayType(integer, integer))
				ar := f.variable("ar", array)
				stored := f.make(interp.OperatorStore, ar, x, y)
				selected := f.make(interp.OperatorSelect, stored, x)
				a = f.make(interp.OperatorEqual, y, zero)
				b = f.make(interp.OperatorNot, f.make(interp.OperatorEqual, selected, zero))
			case "array-extensionality":
				array := f.pin(f.w.ArrayType(integer, integer))
				ar := f.variable("ar", array)
				br := f.variable("br", array)
				left := f.make(interp.OperatorSelect, ar, x)
				right := f.make(interp.OperatorSelect, br, x)
				a = f.pin(f.w.MakeQuantifier(interp.OperatorForall, []ast.ASTHandle{x}, f.make(interp.OperatorEqual, left, right)))
				b = f.make(interp.OperatorNot, f.make(interp.OperatorEqual, ar, br))
			case "quantifier":
				pred := f.pin(f.w.Function("P", []ast.ASTHandle{integer}, boolean))
				px := f.pin(f.m.MkApp(pred, []ast.ASTHandle{x}))
				py := f.pin(f.m.MkApp(pred, []ast.ASTHandle{y}))
				a = f.pin(f.w.MakeQuantifier(interp.OperatorForall, []ast.ASTHandle{x}, px))
				b = f.make(interp.OperatorNot, py)
			case "local-false":
				a = f.make(interp.OperatorAnd, p, f.make(interp.OperatorNot, p))
				b = f.make(interp.OperatorGT, x, zero)
			}
			marked := f.pin(f.c.MkInterpolant(a))
			pattern := f.make(interp.OperatorAnd, marked, b)
			for attempt := 0; attempt < 2; attempt++ {
				results, err := f.c.ComputeInterpolant(pattern)
				if err != nil {
					t.Fatal(err)
				}
				if len(results) != 1 {
					t.Fatalf("got %d interpolants", len(results))
				}
				f.verify(pattern, results)
			}
		})
	}
}

func TestInterpolationContextTree(t *testing.T) {
	f := newInterpolationFixture(t)
	boolean := f.pin(f.w.BoolType())
	p, q := f.variable("p", boolean), f.variable("q", boolean)
	first := f.pin(f.c.MkInterpolant(p))
	second := f.pin(f.c.MkInterpolant(q))
	root := f.make(interp.OperatorNot, f.make(interp.OperatorAnd, p, q))
	branching := f.make(interp.OperatorAnd, first, second, root)
	nested := f.make(interp.OperatorAnd, f.pin(f.c.MkInterpolant(f.make(interp.OperatorAnd, first, q))), root)
	for _, pattern := range []ast.ASTHandle{branching, nested} {
		results, err := f.c.ComputeInterpolant(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Fatalf("got %d tree interpolants", len(results))
		}
		f.verify(pattern, results)
		if pattern == branching {
			valid, message, x := f.c.CheckInterpolants(
				[]ast.ASTHandle{p, q, root},
				[]int{2, 2, interp.ScopeTop},
				results,
				nil,
			)
			if x != nil || !valid || message != "" {
				t.Fatalf("public tree interpolation check: %v valid=%v: %s", x, valid, message)
			}
		}
	}
}

func TestInterpolationContextCompleteResultAndProofInput(t *testing.T) {
	f := newInterpolationFixture(t)
	boolean := f.pin(f.w.BoolType())
	p := f.variable("result-p", boolean)
	marked := f.pin(f.c.MkInterpolant(p))

	satResult, err := f.c.ComputeInterpolation(marked)
	if err != nil {
		t.Fatal(err)
	}
	if satResult.Status != lbool.True || len(satResult.Interpolants) != 0 || satResult.Model == nil {
		t.Fatal("complete SAT interpolation result", satResult.Status, len(satResult.Interpolants), satResult.Model)
	}
	declaration, err := f.m.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	value, err := satResult.Model.ConstantInterpretation(declaration)
	if err != nil || !f.m.IsTrue(value) {
		t.Fatal("SAT interpolation model", value, err)
	}
	if err = satResult.Model.DecRef(); err != nil {
		t.Fatal(err)
	}

	pattern := f.make(interp.OperatorAnd, marked, f.make(interp.OperatorNot, p))
	s, err := f.c.factory.New(f.m, nil, true, true, true, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err = interp.AssertConjuncts(f.w, s, pattern); err != nil {
		t.Fatal(err)
	}
	status, err := s.CheckSat(nil)
	if err != nil || status != lbool.False {
		t.Fatal("proof-producing interpolation solve", status, err)
	}
	proof, err := s.Proof()
	if err != nil {
		t.Fatal(err)
	}
	results, err := f.c.InterpolantsFromProof(proof, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatal("proof interpolant count", len(results))
	}
	f.verify(pattern, results)
}

type unknownInterpolationSolver struct{ solver.Solver }

func (s unknownInterpolationSolver) CheckSat([]ast.ASTHandle) (lbool.LBool, error) {
	return lbool.Undef, nil
}

type failingCloseInterpolationSolver struct {
	solver.Solver
	failure error
}

func (s failingCloseInterpolationSolver) Close() error {
	return errors.Join(s.Solver.Close(), s.failure)
}

func TestInterpolationContextTeardownFailureReleasesResults(t *testing.T) {
	f := newInterpolationFixture(t)
	boolean := f.pin(f.w.BoolType())
	p := f.variable("p", boolean)
	marked := f.pin(f.c.MkInterpolant(p))
	pattern := f.make(interp.OperatorAnd, marked, f.make(interp.OperatorNot, p))
	factory := f.c.factory
	failure := errors.New("solver teardown failed")
	f.c.factory = solver.FactoryFunc(func(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (solver.Solver, error) {
		s, err := factory.New(m, p, proofs, models, cores, logic)
		return failingCloseInterpolationSolver{s, failure}, err
	})
	before := len(f.c.retained)
	results, err := f.c.ComputeInterpolant(pattern)
	f.c.factory = factory
	if !errors.Is(err, failure) || results != nil || len(f.c.retained) != before {
		t.Fatal("failed computation published results", results, err)
	}
	results, err = f.c.ComputeInterpolant(pattern)
	if err != nil {
		t.Fatal(err)
	}
	f.verify(pattern, results)

	f.c.factory = solver.FactoryFunc(func(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (solver.Solver, error) {
		s, err := factory.New(m, p, proofs, models, cores, logic)
		return failingCloseInterpolationSolver{s, failure}, err
	})
	complete, err := f.c.ComputeInterpolation(marked)
	f.c.factory = factory
	if !errors.Is(err, failure) || complete.Status != lbool.Undef || complete.Model != nil || complete.Interpolants != nil {
		t.Fatal("failed SAT computation published result owners", complete, err)
	}
}

func TestInterpolationContextErrorsAndIsolation(t *testing.T) {
	f := newInterpolationFixture(t)
	g := newInterpolationFixture(t)
	integer := f.pin(f.w.IntType())
	x := f.variable("x", integer)
	if _, err := f.c.MkInterpolant(x); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("integer marker", err)
	}
	if _, err := f.c.ComputeInterpolant(x); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("integer pattern", err)
	}
	if _, _, err := f.c.CheckInterpolants([]ast.ASTHandle{x}, []int{0, 1}, nil, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid public interpolation parent vector", err)
	}
	boolean := f.pin(f.w.BoolType())
	p := f.variable("p", boolean)
	if _, err := g.c.MkInterpolant(p); err == nil {
		t.Fatal("accepted foreign AST")
	}
	pattern := f.pin(f.c.MkInterpolant(p))
	if _, err := f.c.ComputeInterpolant(pattern); !errors.Is(err, ErrInterpolationSAT) {
		t.Fatal("SAT", err)
	}
	factory := f.c.factory
	f.c.factory = solver.FactoryFunc(func(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (solver.Solver, error) {
		s, err := factory.New(m, p, proofs, models, cores, logic)
		return unknownInterpolationSolver{s}, err
	})
	_, err := f.c.ComputeInterpolant(pattern)
	f.c.factory = factory
	if !errors.Is(err, ErrInterpolationUnknown) {
		t.Fatal("UNKNOWN", err)
	}
	c, err := NewInterpolationContext()
	if err != nil {
		t.Fatal(err)
	}
	copyContext := *c
	if err = copyContext.Close(); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = c.ComputeInterpolant(p); err == nil {
		t.Fatal("accepted closed context")
	}
	if c.budget.Used() != 0 {
		t.Fatal("copied context did not release its shared state")
	}
	var zero InterpolationContext
	if err := zero.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := zero.Manager(); err == nil {
		t.Fatal("accepted uninitialized context")
	}
}
