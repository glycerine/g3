package bv

// Source: src/tactic/bv/elim_small_bv_tactic.cpp rw_cfg TRACE sites
// (elim_small_bv, elim_small_bv_app, elim_small_bv_pre).

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (c *elimSmallBVConfig) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := diagnostic.BorrowProcessTraceTags()
	err := tags.EmitConditional(tag, function, "src/tactic/bv/elim_small_bv_tactic.cpp", line, func() bool { return true }, body)
	if err != nil && tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// writePP is tout << mk_ismt2_pp(h, m) (mk_pp is the same printer) for an
// expression or a sort.
func (c *elimSmallBVConfig) writePP(w io.Writer, h ast.ASTHandle) (result error) {
	m := c.owner.manager
	k, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if k == ast.ASTSort {
		text, err := m.SMT2PPSortString(h)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, text)
		return err
	}
	f, err := m.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = m.PrintFormatTransient(out, f, nil, nil, nil); err != nil {
		return err
	}
	text, err := out.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}

// writeParts writes strings and expressions in order; an ast.ASTHandle
// part is printed with mk_ismt2_pp.
func (c *elimSmallBVConfig) writeParts(w io.Writer, parts ...any) error {
	for _, p := range parts {
		var err error
		switch v := p.(type) {
		case string:
			_, err = io.WriteString(w, v)
		case ast.ASTHandle:
			err = c.writePP(w, v)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *elimSmallBVConfig) trace(function string, line int, parts ...any) error {
	return c.emit("elim_small_bv", function, line, func(w io.Writer) error {
		return c.writeParts(w, parts...)
	})
}

// traceSubstitution is the "substitution:" TRACE of replace_var, after
// the reverse: position k holds the binding of (VAR size-1-k).
func (c *elimSmallBVConfig) traceSubstitution(args []ast.ASTHandle) error {
	return c.emit("elim_small_bv", "replace_var", 94, func(w io.Writer) error {
		if _, err := io.WriteString(w, "substitution: \n"); err != nil {
			return err
		}
		for k, se := range args {
			if _, err := io.WriteString(w, strconv.Itoa(k)+" = "); err != nil {
				return err
			}
			var err error
			if se.IsNull() {
				_, err = io.WriteString(w, "0")
			} else {
				err = c.writePP(w, se)
			}
			if err != nil {
				return err
			}
			if _, err = io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}

// traceNewBodies is the "new bodies:" TRACE of reduce_quantifier.
func (c *elimSmallBVConfig) traceNewBodies(bodies []ast.ASTHandle) error {
	return c.emit("elim_small_bv", "reduce_quantifier", 177, func(w io.Writer) error {
		if _, err := io.WriteString(w, "new bodies: \n"); err != nil {
			return err
		}
		for _, b := range bodies {
			if err := c.writeParts(w, b, "\n"); err != nil {
				return err
			}
		}
		return nil
	})
}

// traceEliminating is the "eliminating" TRACE of reduce_quantifier.
func (c *elimSmallBVConfig) traceEliminating(q ast.ASTHandle, i uint32, sort, body ast.ASTHandle) error {
	m := c.owner.manager
	return c.emit("elim_small_bv", "reduce_quantifier", 149, func(w io.Writer) error {
		name, err := m.GetDeclarationName(q, i)
		if err != nil {
			return err
		}
		if err = c.writeParts(w, "eliminating "); err != nil {
			return err
		}
		if err = m.Symbols.WriteDisplay(w, name); err != nil {
			return err
		}
		return c.writeParts(w, "; sort = ", sort, "; body = ", body, "\n")
	})
}

// ReduceApp is rw_cfg::reduce_app: TRACE("elim_small_bv_app") builds
// expr_ref tmp(m.mk_app(f, num, args)) for the trace and returns BR_FAILED.
func (c *elimSmallBVConfig) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.owner.manager
	err := c.emit("elim_small_bv_app", "reduce_app", 115, func(w io.Writer) (result error) {
		list := make([]ast.ASTHandle, args.Count())
		for i := range list {
			a, err := args.At(uint32(i))
			if err != nil {
				return err
			}
			list[i] = a
		}
		app, err := m.MkApp(d, list)
		if err != nil {
			return err
		}
		tmp, err := m.NewRef(app)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, m.DelRef(&tmp)) }()
		return c.writeParts(w, "reduce ", app, "\n")
	})
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return c.DefaultConfiguration.ReduceApp(d, args, out, proof)
}
