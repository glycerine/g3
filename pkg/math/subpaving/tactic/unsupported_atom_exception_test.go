package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md math_subpaving_04-F3: subpaving_tactic::operator() turns every
// z3_exception into tactic_exception, so each numeral engine reports
// "unsupported atom" as a tactic exception, as the MPQ engine already did.
func TestSubpavingEnginesUnsupportedAtomIsTacticException(t *testing.T) {
	type core interface {
		Apply(*goaltactic.Goal) (*goaltactic.Goal, error)
		Close() error
	}
	builders := []struct {
		name string
		mk   func(*ast.ASTManager, *params.ParamsContext, *params.ParamsRef) (core, error)
	}{
		{"hwf", func(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (core, error) {
			return NewHardwareCoreTacticOwned(m, pc, p)
		}},
		{"mpfx", func(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (core, error) {
			return NewFixedCoreTacticOwned(m, pc, p)
		}},
		{"mpff", func(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (core, error) {
			return NewFastCoreTacticOwned(m, pc, p)
		}},
		{"mpf", func(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (core, error) {
			return NewArbitraryCoreTacticOwned(m, pc, p)
		}},
	}
	for _, tc := range builders {
		t.Run(tc.name, func(t *testing.T) {
			b := arena.NewBudget(1 << 27)
			integers := numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}
			sy, err := symbol.NewManager(symbol.Config{Budget: b})
			if err != nil {
				t.Fatal(err)
			}
			q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: integers})
			if err != nil {
				t.Fatal(err)
			}
			m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.RegisterArithmeticPlugin(); err != nil {
				t.Fatal(err)
			}
			pc, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: b})
			if err != nil {
				t.Fatal(err)
			}
			p, err := pc.NewValue()
			if err != nil {
				t.Fatal(err)
			}
			numeralName := "numeral"
			engine, err := sy.Intern(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if err := pc.SetSymbolName(&p, &numeralName, engine); err != nil {
				t.Fatal(err)
			}
			c, err := tc.mk(m, pc, &p)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := c.Close(); err != nil {
					t.Error(err)
				}
			})
			g, err := goaltactic.NewGoal(m, false, false, false)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := g.Close(); err != nil {
					t.Error(err)
				}
			})
			boolSort, err := m.MkBoolSort()
			if err != nil {
				t.Fatal(err)
			}
			name, err := sy.Intern("p")
			if err != nil {
				t.Fatal(err)
			}
			atom, err := m.MkConst(name, boolSort)
			if err != nil {
				t.Fatal(err)
			}
			if err := g.Assert(atom); err != nil {
				t.Fatal(err)
			}
			result, err := c.Apply(g)
			var tacticError goaltactic.TacticException
			if result != nil || !errors.As(err, &tacticError) || tacticError.Message() != "unsupported atom" {
				t.Fatalf("got %v, %v; want tactic exception \"unsupported atom\"", result, err)
			}
		})
	}
}
