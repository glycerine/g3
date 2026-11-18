package bv

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	"io"
)

func (t *BVArray2UFTactic) tags() *diagnostic.TraceTags {
	if t.trace != nil {
		return t.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}
func (t *BVArray2UFTactic) formatArray(h ast.ASTHandle) (text string, result error) {
	// mk_ismt2_pp of a func_decl prints its declaration.
	var f ast.ASTHandle
	k, e := t.manager.GetKind(h)
	if e == nil && k == ast.ASTFuncDecl {
		f, e = t.manager.SMT2PPDeclareFun(h)
	} else if e == nil {
		f, e = t.manager.SMT2PPExprTree(h, nil)
	}
	if e != nil {
		return "", e
	}
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if e != nil {
		return "", e
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if e = t.manager.PrintFormatTransient(out, f, nil, nil, nil); e != nil {
		return "", e
	}
	return out.String()
}
func (t *BVArray2UFTactic) tracePair(prefix string, a ast.ASTHandle, middle string, b ast.ASTHandle, line int) {
	tags := t.tags()
	_ = tags.EmitConditional("bvarray2uf_rw", "mk_uf_for_array", "src/tactic/bv/bvarray2uf_rewriter.cpp", line, func() bool { return true }, func(w io.Writer) error {
		x, e := t.formatArray(a)
		if e != nil {
			return e
		}
		y, e := t.formatArray(b)
		if e != nil {
			return e
		}
		_, e = io.WriteString(w, prefix+x+middle+y+"\n")
		return e
	})
}

// traceGoal is TRACE("bvarray2uf", tout << label << std::endl;
// g->display(tout);) in bvarray2uf_tactic::imp::operator().
func (t *BVArray2UFTactic) traceGoal(label string, line int, g interface{ Display(io.Writer) error }) error {
	tags := t.tags()
	err := tags.EmitConditional("bvarray2uf", "operator()", "src/tactic/bv/bvarray2uf_tactic.cpp", line, func() bool { return true }, func(w io.Writer) error {
		if _, e := io.WriteString(w, label+"\n"); e != nil {
			return e
		}
		return g.Display(w)
	})
	if err != nil && !tags.IsStreamFailure(err) {
		return err
	}
	return nil
}

// traceRW emits one bvarray2uf_rewriter.cpp record of tag (bvarray2uf_rw
// or bvarray2uf_rw_q). A part is a string, an ast.ASTHandle printed with
// mk_ismt2_pp, or a bvArrayName printed as func_decl::get_name().
type bvArrayName struct{ decl ast.ASTHandle }

func (t *BVArray2UFTactic) traceRW(tag, function string, line int, parts ...any) {
	tags := t.tags()
	_ = tags.EmitConditional(tag, function, "src/tactic/bv/bvarray2uf_rewriter.cpp", line, func() bool { return true }, func(w io.Writer) error {
		for _, p := range parts {
			var e error
			switch v := p.(type) {
			case string:
				_, e = io.WriteString(w, v)
			case ast.ASTHandle:
				var s string
				if s, e = t.formatArray(v); e == nil {
					_, e = io.WriteString(w, s)
				}
			case bvArrayName:
				var name usymbol.Symbol
				if name, e = t.manager.GetASTName(v.decl); e == nil {
					e = t.manager.Symbols.WriteDisplay(w, name)
				}
			}
			if e != nil {
				return e
			}
		}
		return nil
	})
}
