package core

// Source: src/tactic/core/pb_preprocess_tactic.cpp TRACE("pb") and
// IF_VERBOSE(3) sites.

import (
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// pbPP renders mk_pp(e, m) (and expr_ref's operator<<).
func (t *PBPreprocessTactic) pbPP(term ast.ASTHandle) (_ string, err error) {
	format, err := t.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return "", err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	empty := t.services.Context.EmptyValue()
	if err = t.manager.PrintFormatTransient(stream, format, t.services.Context, &empty, t.services.Globals); err != nil {
		return "", err
	}
	return stream.String()
}

// pbText concatenates literal strings and mk_pp renderings of handles.
func (t *PBPreprocessTactic) pbText(parts ...any) (string, error) {
	var b strings.Builder
	for _, part := range parts {
		switch v := part.(type) {
		case string:
			b.WriteString(v)
		case ast.ASTHandle:
			text, err := t.pbPP(v)
			if err != nil {
				return "", err
			}
			b.WriteString(text)
		default:
			return "", ast.ErrASTDomain
		}
	}
	return b.String(), nil
}

// pbVerbose is IF_VERBOSE(level, verbose_stream() << ...).
func (t *PBPreprocessTactic) pbVerbose(level uint32, text func() (string, error)) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(level)
	if err != nil || !enabled {
		return err
	}
	body, err := text()
	if err != nil {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return stream.AppendCString(body)
}

// pbTrace is TRACE("pb", tout << ...).
func (t *PBPreprocessTactic) pbTrace(function string, line int, text func() (string, error)) error {
	tags := diagnostic.BorrowProcessTraceTags()
	err := tags.EmitConditional("pb", function, "src/tactic/core/pb_preprocess_tactic.cpp", line, func() bool { return true }, func(w io.Writer) error {
		body, err := text()
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, body)
		return err
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
