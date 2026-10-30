package api

// Source: src/api/api_datalog.cpp:347-358.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (f *APIFixedpoint) WriteSMT2(output io.Writer, queries ...ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	if output == nil {
		return io.ErrClosedPipe
	}
	return f.command.WriteFixedpointSMT2WithQueries(output, queries)
}

func (f *APIFixedpoint) SMT2(queries ...ast.ASTHandle) (string, error) {
	if err := f.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	if err := f.command.WriteFixedpointSMT2WithQueries(&output, queries); err != nil {
		return "", err
	}
	return output.String(), nil
}
