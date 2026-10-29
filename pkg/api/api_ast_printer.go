package api

// Source: src/api/api_ast.cpp:819-851 and src/api/api_context.cpp print-mode
// configuration. Go keeps mode on a lightweight borrowed printer instead of
// adding mutable formatting state to the shared solver context.

import (
	"bytes"
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type APIASTPrintMode uint8

const (
	APIASTPrintSMTFull APIASTPrintMode = iota
	APIASTPrintLowLevel
	APIASTPrintSMT2
)

type APIASTPrinter struct {
	owner *smtContextState
	mode  APIASTPrintMode
}

func (c *SMTContext) NewASTPrinter(mode APIASTPrintMode) (*APIASTPrinter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if mode > APIASTPrintSMT2 {
		return nil, ast.ErrASTDomain
	}
	return &APIASTPrinter{owner: c.smtContextState, mode: mode}, nil
}

func (p *APIASTPrinter) SetMode(mode APIASTPrintMode) error {
	if p == nil || p.owner == nil || p.owner.closed || p.owner.command == nil {
		return ErrSMTContextOwner
	}
	if mode > APIASTPrintSMT2 {
		return ast.ErrASTDomain
	}
	p.mode = mode
	return nil
}

func (p *APIASTPrinter) String(value ast.ASTHandle) (result string, resultErr error) {
	if p == nil || p.owner == nil || p.owner.closed || p.owner.command == nil {
		return "", ErrSMTContextOwner
	}
	m, err := p.owner.command.Manager()
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	switch p.mode {
	case APIASTPrintSMTFull:
		err = ast.DisplayLegacySMT(&output, m, value, 0, nil)
	case APIASTPrintLowLevel:
		stream, streamErr := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
		if streamErr != nil {
			return "", streamErr
		}
		if streamErr = m.DisplayLowLevel(stream, value, false, false); streamErr == nil {
			result, streamErr = stream.String()
		}
		return result, errors.Join(streamErr, stream.Close())
	case APIASTPrintSMT2:
		printer, printerErr := ast.NewSimpleASTPrinterContext(m, &output, &output)
		if printerErr != nil {
			return "", printerErr
		}
		kind, kindErr := m.GetKind(value)
		if kindErr != nil {
			return "", kindErr
		}
		switch kind {
		case ast.ASTSort:
			err = printer.DisplaySort(&output, value, 0)
		case ast.ASTFuncDecl:
			err = printer.DisplayFuncDecl(&output, value, 0)
		case ast.ASTApp, ast.ASTVar, ast.ASTQuantifier:
			err = printer.DisplayExpr(&output, value, 0)
		default:
			err = ast.ErrASTDomain
		}
	default:
		err = ast.ErrASTDomain
	}
	if err != nil {
		return "", err
	}
	return output.String(), nil
}
