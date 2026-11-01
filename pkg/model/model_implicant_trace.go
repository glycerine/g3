package model

// Source: src/model/model_implicant.cpp:480,506,515,519,526,529.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (i *ModelImplicant) traceArrayExtraction(line int, prefix string, terms ...ast.ASTHandle) error {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("pdr", "extract_array_func_interp", "src/model/model_implicant.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			_, _ = io.WriteString(out, prefix)
			for _, term := range terms {
				if err := i.evaluator.writeSMT2(out, term); err != nil {
					operandErr = err
					return err
				}
				_, _ = io.WriteString(out, "\n")
			}
			return nil
		})
	return operandErr
}
