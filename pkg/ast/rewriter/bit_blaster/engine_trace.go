package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster.cpp:39,41,78,80
// (TRACE("xor3"/"xor3_sorted"/"carry"/"carry_sorted")) and
// src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h mk_udiv_urem and
// mk_rotate_left (TRACE("bit_blaster")).

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// writeTraceSMT2 is tout << mk_pp(h, m()).
func writeTraceSMT2(m *ast.ASTManager, w io.Writer, h ast.ASTHandle) (err error) {
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil {
		return uparams.ErrGlobalParamsOwner
	}
	f, err := m.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	p := g.Context.EmptyValue()
	if err = m.PrintFormatTransient(stream, f, g.Context, &p, g); err != nil {
		return err
	}
	s, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}

// traceGateIDs is TRACE(tag, tout << "#" << l1->get_id() << " #" ...) in
// bit_blaster_cfg::mk_xor3 / mk_carry (no trailing newline).
func (c *Config) traceGateIDs(tag, function string, line int, a, b, d ast.ASTHandle) error {
	tags := c.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	err := tags.EmitConditional(tag, function, "src/ast/rewriter/bit_blaster/bit_blaster.cpp", line, func() bool { return true }, func(w io.Writer) error {
		for i, h := range [3]ast.ASTHandle{a, b, d} {
			id, e := c.manager.GetASTID(h)
			if e != nil {
				return e
			}
			prefix := " #"
			if i == 0 {
				prefix = "#"
			}
			if _, e = io.WriteString(w, prefix+strconv.FormatUint(uint64(id), 10)); e != nil {
				return e
			}
		}
		return nil
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (e *Engine) traceBits(w io.Writer, bits []ast.ASTHandle) error {
	for _, h := range bits {
		if err := writeTraceSMT2(e.config.manager, w, h); err != nil {
			return err
		}
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) emitBitBlasterTrace(function string, line int, body func(io.Writer) error) error {
	tags := e.config.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bit_blaster") {
		return nil
	}
	err := tags.EmitConditional("bit_blaster", function, "src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// traceUnsignedDivRem is mk_udiv_urem's closing TRACE("bit_blaster").
func (e *Engine) traceUnsignedDivRem(a, b []ast.ASTHandle, quotient, remainder []ast.ASTRef) error {
	return e.emitBitBlasterTrace("mk_udiv_urem", 486, func(w io.Writer) error {
		q, err := e.handles(quotient)
		if err != nil {
			return err
		}
		r, err := e.handles(remainder)
		if err != nil {
			return err
		}
		for i, row := range [4][]ast.ASTHandle{a, b, q, r} {
			label := [4]string{"a: ", "\nb: ", "\nq: ", "\nr: "}[i]
			if _, err = io.WriteString(w, label); err != nil {
				return err
			}
			if err = e.traceBits(w, row); err != nil {
				return err
			}
		}
		_, err = io.WriteString(w, "\n")
		return err
	})
}

// traceRotateLeft is mk_rotate_left's TRACE("bit_blaster"): the amount is
// printed before the n % sz reduction.
func (e *Engine) traceRotateLeft(bits []ast.ASTHandle, amount uint64) error {
	return e.emitBitBlasterTrace("mk_rotate_left", 838, func(w io.Writer) error {
		if _, err := io.WriteString(w, strconv.FormatUint(uint64(uint32(amount)), 10)+": "+strconv.Itoa(len(bits))+" "); err != nil {
			return err
		}
		if err := e.traceBits(w, bits); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
