package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISolverFactoryDefaultStrategicSolver(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", func(c *SMTContext) {
		astTestMust(t, c.SetBoolOption("model", true))
	})
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-factory-default-p", boolean))
	astTestMust(t, s.Assert(p))
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.True {
		t.Fatal("status", status, checkErr)
	}
	md, err := s.Model()
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(p)
	astTestMust(t, err)
	if value, valueErr := md.ConstantInterpretation(declaration); valueErr != nil || value.IsNull() {
		t.Fatal("model interpretation", value, valueErr)
	}
	astTestMust(t, md.Close())
	astTestMust(t, s.Close())
}

// Z3_mk_solver and Z3_solver_set_params do not allocate solver-private ASTs
// until first use. Other users of the same manager observe that allocation order.
func TestAPINewSolverPreservesSharedManagerIDsUntilUse(t *testing.T) {
	firstID := func(withSolver bool) uint32 {
		c, err := NewSMTContext()
		astTestMust(t, err)
		defer func() { astTestMust(t, c.Close()) }()
		if withSolver {
			s, err := c.NewSolver()
			astTestMust(t, err)
			defer func() { astTestMust(t, s.Close()) }()
			p, err := c.NewParameterSet()
			astTestMust(t, err)
			astTestMust(t, p.SetBool("macro_finder", true))
			astTestMust(t, s.SetParameters(p))
			astTestMust(t, p.Close())
		}
		sort, err := c.BitVectorSort(3)
		astTestMust(t, err)
		x, err := c.Constant("shared-manager-x", sort)
		astTestMust(t, err)
		m, err := c.Manager()
		astTestMust(t, err)
		id, err := m.GetASTID(x)
		astTestMust(t, err)
		return id
	}
	if want, got := firstID(false), firstID(true); got != want {
		t.Fatalf("creating an unused solver changed the next expression ID from %d to %d", want, got)
	}
}

func TestAPINewSolverRetainsParametersUntilFirstUse(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	p, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, p.SetUInt("random_seed", 17))
	astTestMust(t, s.SetParameters(p))
	astTestMust(t, p.SetUInt("random_seed", 23))
	astTestMust(t, p.Close())

	descriptions, err := s.ParameterDescriptions()
	astTestMust(t, err)
	astTestMust(t, descriptions.Close())
	if s.solver != nil {
		t.Fatal("querying parameter descriptions kept a backend alive")
	}

	astTestMust(t, s.Push())
	current, err := s.solver.Params()
	astTestMust(t, err)
	name := "random_seed"
	seed, err := s.explicitContext.GetUIntName(current, &name, 0)
	astTestMust(t, err)
	if seed != 17 {
		t.Fatalf("first use applied seed %d, want 17", seed)
	}
}

func TestAPINewSolverValidatesPendingParametersOnFirstUse(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSolver()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	p, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, p.SetUInt("not_a_solver_parameter", 1))
	astTestMust(t, s.SetParameters(p))
	astTestMust(t, p.Close())
	if err := s.Push(); err == nil {
		t.Fatal("first use accepted an unknown solver parameter")
	}
}
