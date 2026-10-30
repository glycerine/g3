package opt

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type objectiveFormulaFixture struct {
	manager  *ast.ASTManager
	numerals *unumeral.RationalService
	symbols  *usymbol.Manager
	budget   *arena.Budget
	refs     []ast.ASTRef
	p, q     ast.ASTHandle
}

func newObjectiveFormulaFixture(t *testing.T) *objectiveFormulaFixture {
	t.Helper()
	f := &objectiveFormulaFixture{budget: arena.NewBudget(1 << 26)}
	var err error
	f.symbols, err = usymbol.NewManager(usymbol.Config{Budget: f.budget})
	if err != nil {
		t.Fatal(err)
	}
	f.numerals, err = unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: f.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: f.budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	f.manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: f.symbols, Numerals: f.numerals, Budget: f.budget})
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := f.manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		out  *ast.ASTHandle
	}{{"codec-p", &f.p}, {"codec-q", &f.q}} {
		name, e := f.symbols.Intern(item.name)
		if e != nil {
			t.Fatal(e)
		}
		*item.out, e = f.manager.MkConst(name, boolean)
		if e != nil {
			t.Fatal(e)
		}
		ref, e := f.manager.NewRef(*item.out)
		if e != nil {
			t.Fatal(e)
		}
		f.refs = append(f.refs, ref)
	}
	t.Cleanup(func() {
		for i := len(f.refs) - 1; i >= 0; i-- {
			if err := f.manager.DelRef(&f.refs[i]); err != nil {
				t.Error(err)
			}
		}
		if err := f.manager.Close(); err != nil {
			t.Error(err)
		}
		if err := f.numerals.Close(); err != nil {
			t.Error(err)
		}
		if err := f.symbols.Close(); err != nil {
			t.Error(err)
		}
		if used := f.budget.Used(); used != 0 {
			t.Errorf("retained budget %d", used)
		}
	})
	return f
}

func TestObjectiveFormulaCodecRoundTripRetainsOrderAndMetadata(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	codec, err := NewObjectiveFormulaCodec(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := codec.Close(); err != nil {
			t.Error(err)
		}
	})
	weight := big.NewRat(7, 2)
	maximum, err := codec.EncodeObjective(FormulaObjective{Kind: FormulaMaximize, Term: f.p})
	if err != nil {
		t.Fatal(err)
	}
	minimum, err := codec.EncodeObjective(FormulaObjective{Kind: FormulaMinimize, Term: f.q})
	if err != nil {
		t.Fatal(err)
	}
	soft, err := codec.EncodeObjective(FormulaObjective{Kind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{f.p}, Weights: []*big.Rat{weight}, ID: "pref"})
	if err != nil {
		t.Fatal(err)
	}
	weight.SetInt64(99)
	decoded, err := codec.Decode([]ast.ASTHandle{f.q, maximum, minimum, soft})
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Hard) != 1 || decoded.Hard[0] != f.q || len(decoded.Objectives) != 3 {
		t.Fatal(decoded)
	}
	if decoded.Objectives[0].Kind != FormulaMaximize || decoded.Objectives[0].Term != f.p || decoded.Objectives[0].Negate {
		t.Fatal(decoded.Objectives[0])
	}
	if decoded.Objectives[1].Kind != FormulaMinimize || decoded.Objectives[1].Term != f.q || !decoded.Objectives[1].Negate {
		t.Fatal(decoded.Objectives[1])
	}
	got := decoded.Objectives[2]
	if got.Kind != FormulaMaxSAT || got.ID != "pref" || len(got.SoftTerms) != 1 || got.SoftTerms[0] != f.p || got.Weights[0].RatString() != "7/2" {
		t.Fatal(got)
	}
}
