package model

// Source: src/model/model_implicant.cpp:136,164; src/util/debug.h SASSERT.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (i *ModelImplicant) assertKnownBoolean(expression ast.ASTHandle, line int32) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil {
		return err
	}
	if enabled && !i.isTrue(expression) && !i.isFalse(expression) {
		return diagnostic.DebugAssertionFailure(debug, "src/model/model_implicant.cpp", line, "is_true(e) || is_false(e)")
	}
	return nil
}
