package tactic

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md math_subpaving_04-F2: subpaving_tactic::imp::process calls
// display_constraints(std::cout) with the default use_star = false, so the
// MPFX and MPFF print_nodes text shows "k = x y", not "k = x*y".
func TestSubpavingPrintNodesDefinitionsWithoutStar(t *testing.T) {
	type core interface {
		ApplyWithOutput(*goaltactic.Goal, io.Writer) (*goaltactic.Goal, error)
		Close() error
	}
	builders := []struct {
		name string
		mk   func(*ast.ASTManager, *params.ParamsContext, *params.ParamsRef) (core, error)
	}{
		{"mpfx", func(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (core, error) {
			return NewFixedCoreTacticOwned(m, pc, p)
		}},
		{"mpff", func(m *ast.ASTManager, pc *params.ParamsContext, p *params.ParamsRef) (core, error) {
			return NewFastCoreTacticOwned(m, pc, p)
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
			family, err := m.RegisterArithmeticPlugin()
			if err != nil {
				t.Fatal(err)
			}
			sort, err := m.MkFamilySort(family, 0, nil)
			if err != nil {
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
			engine, err := sy.Intern(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			engineKey, depthKey, printKey := "numeral", "max_depth", "print_nodes"
			if err := pc.SetSymbolName(&p, &engineKey, engine); err != nil {
				t.Fatal(err)
			}
			if err := pc.SetUIntName(&p, &depthKey, 0); err != nil {
				t.Fatal(err)
			}
			if err := pc.SetBoolName(&p, &printKey, true); err != nil {
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
			var vars []ast.ASTHandle
			for _, s := range []string{"x", "y"} {
				name, err := sy.Intern(s)
				if err != nil {
					t.Fatal(err)
				}
				v, err := m.MkConst(name, sort)
				if err != nil {
					t.Fatal(err)
				}
				vars = append(vars, v)
			}
			value, err := q.NewValue()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = q.Del(&value) })
			if err := q.SetString(&value, "1"); err != nil {
				t.Fatal(err)
			}
			one, err := m.ArithmeticNumeral(&value, false)
			if err != nil {
				t.Fatal(err)
			}
			product, err := m.MkFamilyApp(family, 9, nil, vars) // OP_MUL
			if err != nil {
				t.Fatal(err)
			}
			ineq, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{product, one}) // OP_LE
			if err != nil {
				t.Fatal(err)
			}
			if err := g.Assert(ineq); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if _, err := c.ApplyWithOutput(g, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), " = x y\n") || strings.Contains(out.String(), "x*y") {
				t.Fatalf("print_nodes definitions must not use '*':\n%s", out.String())
			}
		})
	}
}
