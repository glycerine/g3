package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:68-72; src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *CachedVarSubstitution) traceCacheHit(bindings []ast.ASTHandle) error {
	tags := c.manager.BorrowMemoryTraceTags()
	if !tags.ShouldTrace("bindings") || !tags.ShouldTrace("coming_from_quant") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	err := tags.EmitConditional("bindings", "operator()", "src/ast/rewriter/cached_var_subst.cpp", 68,
		func() bool { return tags.ShouldTrace("coming_from_quant") }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "(cache)\n"); err != nil {
				return err
			}
			for i, binding := range bindings {
				if binding.IsNull() {
					continue
				}
				if _, err := io.WriteString(w, strconv.Itoa(i)+": "); err != nil {
					return err
				}
				if err := writeBVTraceSMT2(c.manager, w, binding, globals); err != nil {
					return err
				}
				if _, err := io.WriteString(w, ";\n"); err != nil {
					return err
				}
			}
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
