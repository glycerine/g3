package model

// Source: src/model/model_implicant.cpp:99-107,121-124,154-157.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (i *ModelImplicant) traceMinimization(tag, function string, line int, prefix string, expressions []ast.ASTHandle, results []ast.ASTRef) error {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional(tag, function, "src/model/model_implicant.cpp", line, func() bool { return true }, func(w io.Writer) error {
		_, _ = io.WriteString(w, prefix)
		print := func(h ast.ASTHandle) (resultErr error) {
			format, err := i.manager.SMT2PPExprTree(h, nil)
			if err != nil {
				return err
			}
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: i.manager.Budget()})
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, out.Close()) }()
			empty := i.services.Context.EmptyValue()
			if err = i.manager.PrintFormat(out, format, i.services.Context, &empty, i.services.Globals); err != nil {
				return err
			}
			body, err := out.String()
			if err != nil {
				return err
			}
			_, _ = io.WriteString(w, body+"\n")
			return nil
		}
		for _, h := range expressions {
			if operandErr = print(h); operandErr != nil {
				return operandErr
			}
		}
		for _, ref := range results {
			h, err := i.manager.RefGet(ref)
			if err != nil {
				operandErr = err
				return err
			}
			if operandErr = print(h); operandErr != nil {
				return operandErr
			}
		}
		return nil
	})
	return operandErr
}
