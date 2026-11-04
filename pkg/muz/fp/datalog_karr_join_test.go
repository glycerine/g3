package fp

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestKarrJoinPlanSourceColumns(t *testing.T) {
	m, _ := newPDRLifecycleServices(t)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	c, err := m.ArithmeticIntegerNumeralText("9")
	must(err)
	app := func(name string, args ...ast.ASTHandle) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		must(e)
		sorts := make([]ast.ASTHandle, len(args))
		for i := range sorts {
			sorts[i] = integer
		}
		d, e := m.MkFuncDecl(s, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		h, e := m.MkApp(d, args)
		must(e)
		return h
	}
	a := &karrDatalogActivation{manager: m}
	p, err := a.compileKarrJoin(app("join-head", x, x, z), app("join-left", x, x, c, y), app("join-right", x, z, z))
	must(err)
	if !reflect.DeepEqual(p.leftColumns, []uint32{0, 1}) || !reflect.DeepEqual(p.rightColumns, []uint32{0, 0}) || !reflect.DeepEqual(p.removed, []uint32{0, 3, 5}) || !reflect.DeepEqual(p.expressions, []ast.ASTHandle{x, c, x, z}) || !reflect.DeepEqual(p.constants, []karrJoinConstant{{1, true}}) || len(p.identical) != 0 {
		t.Fatalf("plan: %+v", p)
	}
	p, err = a.compileKarrJoin(app("join-head2", z, z), app("join-left2", x), app("join-right2", z, z))
	must(err)
	if !reflect.DeepEqual(p.removed, []uint32{0}) || !reflect.DeepEqual(p.identical, [][]uint32{{0, 1}}) {
		t.Fatalf("local duplicate: %+v", p)
	}
}

func TestKarrJoinPlanApplyOwnershipAndEmpty(t *testing.T) {
	left := rel.NewKarrFull(1, nil)
	right := rel.NewKarrFull(1, nil)
	row := []big.Rat{*big.NewRat(1, 1)}
	fragmentMust(t, left.AddConstraint(row, big.NewRat(-2, 1), true))
	fragmentMust(t, right.AddConstraint(row, big.NewRat(-3, 1), true))
	p := karrJoinPlan{leftColumns: []uint32{0}, rightColumns: []uint32{0}, constants: []karrJoinConstant{{1, true}}}
	got, err := p.apply(left, right)
	fragmentMust(t, err)
	matrix, err := got.Inequalities()
	fragmentMust(t, err)
	if len(matrix.A) != 4 || matrix.A[2][0].Cmp(big.NewRat(1, 1)) != 0 || matrix.A[2][1].Cmp(big.NewRat(-1, 1)) != 0 || matrix.B[3].Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("ordered join and filter", matrix)
	}
	for _, operand := range []*rel.KarrRelation{left, right} {
		matrix, err := operand.Inequalities()
		fragmentMust(t, err)
		if len(matrix.A) != 1 {
			t.Fatal("operand changed", matrix)
		}
	}
	empty := rel.NewKarrEmpty(1, func() bool { t.Fatal("empty join evaluated checkpoint"); return false })
	got, err = (karrJoinPlan{removed: []uint32{0}, constants: []karrJoinConstant{{0, true}}}).apply(empty, right)
	fragmentMust(t, err)
	if !got.Empty() || got.Dimension() != 1 {
		t.Fatal("empty projection", got)
	}
	_, err = (karrJoinPlan{leftColumns: []uint32{2}, rightColumns: []uint32{0}}).apply(left, right)
	if err != rel.ErrKarrColumn {
		t.Fatal("invalid column", err)
	}
}

func TestKarrJoinPlanCancellationDuringProjection(t *testing.T) {
	calls := 0
	left := rel.NewKarrFull(1, func() bool { calls++; return false })
	fragmentMust(t, left.AddConstraint([]big.Rat{*big.NewRat(1, 1)}, big.NewRat(-2, 1), true))
	right := rel.NewKarrFull(1, nil)
	// Hilbert reports cancellation as Undef, not an error. The activation must
	// check its resource limit before publishing the candidate.
	_, err := (karrJoinPlan{removed: []uint32{0}}).apply(left, right)
	fragmentMust(t, err)
	if calls == 0 {
		t.Fatal("projection did not consult cancellation checkpoint")
	}
	matrix, err := left.Inequalities()
	fragmentMust(t, err)
	if len(matrix.A) != 1 || matrix.B[0].Cmp(big.NewRat(-2, 1)) != 0 {
		t.Fatal("interrupted join changed input", matrix)
	}
}
