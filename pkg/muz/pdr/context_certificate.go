package pdr

// Source: context::display_certificate in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// DisplayCertificate writes the source diagnostic certificate for the latest
// result. boogie selects the source print_boogie_certificate presentation for
// reachable proof traces; queryDeclaration must identify the latest query.
func (c *Context) DisplayCertificate(out io.Writer, queryDeclaration ast.ASTHandle, boogie bool) (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if c.queryDecl != queryDeclaration {
		return ast.ErrASTDomain
	}
	switch c.lastResult {
	case lbool.Undef:
		_, err := io.WriteString(out, "unknown")
		return err
	case lbool.False:
		certificate, err := c.finiteFrameModel(c.inductive, queryDeclaration)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, certificate.Close()) }()
		config := c.owner.contexts.config
		return certificate.Core().WriteSMT2(out, config.Params, config.Globals, 0)
	case lbool.True:
		if boogie {
			proofsEnabled, err := c.manager.ProofsEnabled()
			if err != nil {
				return err
			}
			if !proofsEnabled {
				if err = c.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
					return err
				}
				defer func() { result = errors.Join(result, c.manager.ToggleProofMode(ast.ASTProofDisabled)) }()
			}
			proof, err := c.Proof(queryDeclaration)
			if err != nil {
				return fmt.Errorf("pdr: construct Boogie certificate proof: %w", err)
			}
			defer func() { result = errors.Join(result, c.manager.DelRef(&proof)) }()
			handle, err := c.manager.RefGet(proof)
			if err != nil {
				return err
			}
			config := c.owner.contexts.config
			certificate, err := base.NewBoogieProof(c.manager, base.BoogieProofConfig{Context: config.Params, Globals: config.Globals, Diagnostic: io.Discard})
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, certificate.Close()) }()
			if err = certificate.SetProof(handle); err != nil {
				return fmt.Errorf("pdr: normalize Boogie certificate proof: %w", err)
			}
			if err = certificate.SetModel(nil); err != nil {
				return err
			}
			if err = certificate.PP(out); err != nil {
				return fmt.Errorf("pdr: write Boogie certificate proof: %w", err)
			}
			return nil
		}
		answer, err := c.Answer(queryDeclaration)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, c.manager.DelRef(&answer)) }()
		handle, err := c.manager.RefGet(answer)
		if err != nil {
			return err
		}
		printer, err := ast.NewSimpleASTPrinterContext(c.manager, out, nil)
		if err != nil {
			return err
		}
		return printer.DisplayExpr(out, handle, 0)
	default:
		return ast.ErrASTDomain
	}
}
