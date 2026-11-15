package pdr

// Source: pred_transformer::init_rule, pdr_context.cpp:579-584,603-607.

import (
	"bytes"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (b *predicateRuleBuilder) nonGroundTailError(tail ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(b.transformer.manager, nil, nil)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	if err = printer.DisplayExpr(&body, tail, 0); err != nil {
		return err
	}
	return pdrDefaultException("PDR cannot solve non-ground tails: "+body.String(), true)
}

// pdrDefaultException is the source
//
//	IF_VERBOSE(0, verbose_stream() << msg << "\n";);
//	throw default_exception(msg);
//
// pair; verbose selects whether the site has the IF_VERBOSE(0) echo.
func pdrDefaultException(message string, verbose bool) error {
	if verbose {
		enabled, err := diagnostic.ProcessVerboseEnabled(0)
		if err != nil {
			return err
		}
		if enabled {
			stream, x := diagnostic.BorrowProcessVerboseStream()
			if x != nil {
				return x
			}
			// Source ostream errors are recorded on the stream and do not replace the exception.
			_ = stream.AppendCString(message + "\n")
		}
	}
	return z3exception.NewDefaultException(message)
}
