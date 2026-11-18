package fpa

// Source: src/ast/fpa/bv2fpa_converter.cpp:474-503.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Display writes the source converter cache form used by the FPA model
// converter's diagnostic display.
func (c *Converter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	printer, err := ast.NewSimpleASTPrinterContext(c.manager, out, out)
	if err != nil {
		return err
	}
	write := func(text string) error {
		_, err := io.WriteString(out, text)
		return err
	}
	name := func(declaration ast.ASTHandle) (string, error) {
		symbol, err := c.manager.GetASTName(declaration)
		if err != nil {
			return "", err
		}
		return c.manager.Symbols.Display(symbol)
	}
	// indent = n.size() + 4, the symbol's own size.
	indentOf := func(declaration ast.ASTHandle) (uint32, error) {
		symbol, err := c.manager.GetASTName(declaration)
		if err != nil {
			return 0, err
		}
		size, err := c.manager.Symbols.Size(symbol)
		return size + 4, err
	}
	expressionEntry := func(declaration, value ast.ASTHandle) error {
		text, err := name(declaration)
		if err != nil {
			return err
		}
		indent, err := indentOf(declaration)
		if err != nil {
			return err
		}
		if err = write("\n  (" + text + " "); err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, value, indent); err != nil {
			return err
		}
		return write(")")
	}
	declarationEntry := func(declaration, value ast.ASTHandle) error {
		text, err := name(declaration)
		if err != nil {
			return err
		}
		if err = write("\n  (" + text + " "); err != nil {
			return err
		}
		if err = printer.DisplayFuncDecl(out, value, 0); err != nil {
			return err
		}
		return write(")")
	}
	for _, entry := range c.constants.Entries() {
		declaration, value := entry.Key, entry.Value[0]
		if err = expressionEntry(declaration, value); err != nil {
			return err
		}
	}
	for _, entry := range c.roundingConstants.Entries() {
		declaration, value := entry.Key, entry.Value[0]
		if err = expressionEntry(declaration, value); err != nil {
			return err
		}
	}
	for _, entry := range c.convertedUFs.Entries() {
		declaration, value := entry.Key, entry.Value[0]
		if err = declarationEntry(declaration, value); err != nil {
			return err
		}
	}
	for _, entry := range c.minMaxSigns.Entries() {
		declaration, values := entry.Key, entry.Value
		text, e := name(declaration)
		if e != nil {
			return e
		}
		indent, e := indentOf(declaration)
		if e != nil {
			return e
		}
		if e = write("\n  (" + text + " "); e != nil {
			return e
		}
		if e = printer.DisplayExpr(out, values[0], indent); e != nil {
			return e
		}
		if e = write("; "); e != nil {
			return e
		}
		if e = printer.DisplayExpr(out, values[1], indent); e != nil {
			return e
		}
		if e = write(")"); e != nil {
			return e
		}
	}
	return nil
}
