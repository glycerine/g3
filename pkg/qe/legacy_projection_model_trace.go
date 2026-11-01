package qe

// Source: src/muz/spacer/spacer_util.cpp mbqi_project_var TRACE decisions.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *LegacyProjection) traceModelTermProjection(variable, term ast.ASTHandle, replaced bool) error {
	tags := p.config.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	line := 1256
	if replaced {
		line = 1247
	}
	err := tags.EmitConditional("mbqi_project", "mbqi_project_var", "src/muz/spacer/spacer_util.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			writer := traceBodyWriter{out}
			prefix, separator := "MBQI: failed to eliminate ", " from "
			if replaced {
				prefix, separator = "MBQI: replacing ", " with "
			}
			_, _ = io.WriteString(writer, prefix)
			if err := writeExpression(p.manager, p.config, writer, variable); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, separator)
			if err := writeExpression(p.manager, p.config, writer, term); err != nil {
				return err
			}
			_, _ = io.WriteString(writer, "\n")
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
