package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:32-49;
// concrete inverse expr2var display for MPFX arithmetic variables.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/smt2"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func fixedWriteDisplay(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}

func fixedDisplayNumeral(w io.Writer, value string, integer bool) error {
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = value[1:]
		if err := fixedWriteDisplay(w, "(- "); err != nil {
			return err
		}
	}
	if numerator, denominator, fraction := strings.Cut(value, "/"); fraction {
		if err := fixedWriteDisplay(w, "(/ "+numerator+".0 "+denominator+".0)"); err != nil {
			return err
		}
	} else if integer {
		if err := fixedWriteDisplay(w, value); err != nil {
			return err
		}
	} else if err := fixedWriteDisplay(w, value+".0"); err != nil {
		return err
	}
	if negative {
		return fixedWriteDisplay(w, ")")
	}
	return nil
}

func fixedDisplaySymbol(w io.Writer, symbols *symbol.Manager, name symbol.Symbol) error {
	quoted, err := smt2.IsSMT2QuotedSymbol(symbols, name)
	if err != nil {
		return err
	}
	var spelling string
	if quoted {
		spelling, err = smt2.MkSMT2QuotedSymbol(symbols, name, nil)
	} else {
		spelling, err = symbols.Display(name)
	}
	if err != nil {
		return err
	}
	return fixedWriteDisplay(w, spelling)
}

// displayMappedVariable prints the concrete expr2var values reached by the
// MPFX converter: free variables and uninterpreted arithmetic applications
// whose arguments are themselves free variables, numerals or applications.
func (c *FixedExprConverter) displayMappedVariable(w io.Writer, h ast.ASTHandle, symbols *symbol.Manager) error {
	kind, err := c.ast.GetKind(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, err := c.ast.GetVarIndex(h)
		if err != nil {
			return err
		}
		return fixedWriteDisplay(w, "(:var "+strconv.FormatUint(uint64(index), 10)+")")
	}
	if kind != ast.ASTApp {
		return ErrFixedCorePrintNodes
	}
	value, integer, numeral, err := c.ast.ArithmeticNumeralText(h)
	if err != nil {
		return err
	}
	if numeral {
		return fixedDisplayNumeral(w, value, integer)
	}
	family, err := c.ast.GetFamilyID(h)
	if err != nil {
		return err
	}
	if family != ast.NullFamilyID {
		return ErrFixedCorePrintNodes
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
		return fixedDisplaySymbol(w, symbols, name)
	}
	if err := fixedWriteDisplay(w, "("); err != nil {
		return err
	}
	if err := fixedDisplaySymbol(w, symbols, name); err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		if err := fixedWriteDisplay(w, " "); err != nil {
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
	return fixedWriteDisplay(w, ")")
}

// VariableDisplay captures the source inverse mapping at the point immediately
// after goal internalization. The converter retains mapped AST references until
// it closes, and generated definition variables fall back to k!N.
func (c *FixedExprConverter) VariableDisplay(symbols *symbol.Manager) (subpaving.FixedVariableDisplay, error) {
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
		return fixedWriteDisplay(w, "k!"+strconv.FormatUint(uint64(variable), 10))
	}, nil
}
