package fp

// Source: src/muz/tab/tab_context.cpp tab::display_certificate and
// src/muz/base/dl_context.cpp context::display_certificate.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/tab"
)

// tab::imp::display_certificate prints mk_pp(get_answer()): the SMT2
// pretty printer, as for the other proof certificates.
func (e *PropositionalEngine) writeTabCertificate(out io.Writer, declaration ast.ASTHandle, writeExpression func(io.Writer, ast.ASTHandle) error) (found bool, resultErr error) {
	if err := e.requireResultSnapshot(EngineTab); err != nil {
		return false, err
	}
	answer, err := e.Answer(declaration)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&answer)) }()
	if writeExpression == nil {
		return true, tab.DisplayCertificate(e.manager, out, answer)
	}
	proof, err := e.manager.RefGet(answer)
	if err != nil {
		return false, err
	}
	if err = writeExpression(out, proof); err != nil {
		return true, err
	}
	_, err = fmt.Fprintln(out)
	return true, err
}
