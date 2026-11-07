package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtBooleanModelOrderTheory struct {
	*SMTTheoryArith
	declaration ast.ASTHandle
}

func (t *smtBooleanModelOrderTheory) InitializeModel(g *SMTModelGenerator) error {
	value, err := g.model.Core().ConstantInterpretation(t.declaration)
	if err != nil {
		return err
	}
	if !value.IsNull() {
		return fmt.Errorf("Boolean publication preceded theory initialization")
	}
	return t.SMTTheoryArith.InitializeModel(g)
}

func (t *smtBooleanModelOrderTheory) ModelValue(node *SMTEnode, g *SMTModelGenerator) (SMTModelValueProcedure, error) {
	value, err := g.model.Core().ConstantInterpretation(t.declaration)
	if err != nil {
		return nil, err
	}
	if !g.manager.IsTrue(value) {
		return nil, fmt.Errorf("Boolean assignment unavailable during theory value discovery")
	}
	procedure, err := t.SMTTheoryArith.ModelValue(node, g)
	return &smtBooleanModelOrderProcedure{SMTModelValueProcedure: procedure, declaration: t.declaration}, err
}

type smtBooleanModelOrderProcedure struct {
	SMTModelValueProcedure
	declaration ast.ASTHandle
}

func (p *smtBooleanModelOrderProcedure) MakeValue(g *SMTModelGenerator, values []ast.ASTHandle) (ast.ASTHandle, error) {
	// A theory may write interpretations while constructing its values.
	// Only the later enode-publication pass should overwrite this entry.
	no, err := g.manager.MkFalse()
	if err == nil {
		err = g.model.Core().RegisterConstant(p.declaration, no)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.SMTModelValueProcedure.MakeValue(g, values)
}

func TestSMTModelGeneratorBooleanPublicationPhases(t *testing.T) {
	for _, withEnode := range []bool{false, true} {
		t.Run(fmt.Sprint(withEnode), func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			context, _, state := smtInternalizerFixture(t, m, nil)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			p, err := m.MkConst(astTestSymbol(t, m, "model-order-p"), boolean)
			astTestMust(t, err)
			declaration, err := m.GetDecl(p)
			astTestMust(t, err)
			variable, err := state.MkBoolVar(p)
			astTestMust(t, err)
			astTestMust(t, state.SetAssignment(variable, lbool.True))
			if withEnode {
				_, err = state.MkEnode(p, false, false, true)
				astTestMust(t, err)
			}
			astTestMust(t, context.SetRelevant(p, true))
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			arithmetic, err := NewSMTTheoryArith(family, false)
			astTestMust(t, err)
			theory := &smtBooleanModelOrderTheory{SMTTheoryArith: arithmetic, declaration: declaration}
			astTestMust(t, state.RegisterTheory(theory))
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "model-order-x"), real)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(x, false))
			// internalize_var: an arithmetic term mentioning x attaches it.
			if _, err := arithmetic.internalizeTermCore(x); err != nil {
				t.Fatal(err)
			}
			astTestMust(t, context.SetRelevant(x, true))
			feasible, err := arithmetic.propagateCore()
			astTestMust(t, err)
			if !feasible {
				t.Fatal("free arithmetic fixture infeasible")
			}
			g, err := NewSMTModelGenerator(m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, g.Close()) })
			astTestMust(t, g.SetContext(context))
			astTestMust(t, g.InitializeModel(config))
			astTestMust(t, g.BuildStateValues(state))
			value, err := g.model.Core().ConstantInterpretation(declaration)
			astTestMust(t, err)
			if withEnode && !m.IsTrue(value) || !withEnode && !m.IsFalse(value) {
				t.Fatal("later enode publication did not preserve native phase order", withEnode, value)
			}
		})
	}
}
