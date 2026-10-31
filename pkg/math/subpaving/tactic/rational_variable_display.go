package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:32-49,
// concrete inverse expr2var display for MPQ arithmetic variables.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (c *RationalExprConverter) displayMappedVariable(w io.Writer, h ast.ASTHandle, symbols *symbol.Manager) error {
	kind, err := c.ast.GetKind(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, err := c.ast.GetVarIndex(h)
		if err != nil {
			return err
		}
		return fastWriteDisplay(w, "(:var "+strconv.FormatUint(uint64(index), 10)+")")
	}
	if kind != ast.ASTApp {
		return ErrRationalCorePrintNodes
	}
	value, integer, numeral, err := c.ast.ArithmeticNumeralText(h)
	if err != nil {
		return err
	}
	if numeral {
		return fastDisplayNumeral(w, value, integer)
	}
	family, err := c.ast.GetFamilyID(h)
	if err != nil {
		return err
	}
	if family != ast.NullFamilyID {
		return ErrRationalCorePrintNodes
	}
	count, err := c.ast.GetNumArgs(h)
	if err != nil {
		return err
	}
	decl, err := c.ast.GetDecl(h)
	if err != nil {
		return err
	}
	name, err := c.ast.GetASTName(decl)
	if err != nil {
		return err
	}
	if count == 0 {
		return fastDisplaySymbol(w, symbols, name)
	}
	if err := fastWriteDisplay(w, "("); err != nil {
		return err
	}
	if err := fastDisplaySymbol(w, symbols, name); err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		if err := fastWriteDisplay(w, " "); err != nil {
			return err
		}
		arg, err := c.ast.GetArg(h, i)
		if err != nil {
			return err
		}
		if err := c.displayMappedVariable(w, arg, symbols); err != nil {
			return err
		}
	}
	return fastWriteDisplay(w, ")")
}

// VariableDisplay captures mapped leaves after internalization. Generated
// definition variables retain the source k!N fallback spelling.
func (c *RationalExprConverter) VariableDisplay(symbols *symbol.Manager) (subpaving.RationalVariableDisplay, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	if symbols == nil {
		return nil, arena.ErrInvalidSlot
	}
	entries, err := c.mapping.Entries()
	if err != nil {
		return nil, err
	}
	inverse := make(map[uint32]ast.ASTHandle, len(entries))
	pins, err := pinExprMapping(c.ast, entries)
	if err != nil {
		return nil, err
	}
	c.order = append(c.order, pins...)
	for _, entry := range entries {
		inverse[entry.Variable] = entry.Expr
	}
	return func(w io.Writer, variable uint32) error {
		if err := c.check(); err != nil {
			return err
		}
		if w == nil {
			return arena.ErrInvalidSlot
		}
		if h, found := inverse[variable]; found {
			return c.displayMappedVariable(w, h, symbols)
		}
		return fastWriteDisplay(w, "k!"+strconv.FormatUint(uint64(variable), 10))
	}, nil
}
