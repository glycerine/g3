package sls

// Source: src/tactic/sls/sls_tracker.h:454,647,657,661 and
// src/tactic/sls/sls_engine.cpp:301,563 TRACE sites.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// traceModel is TRACE(tag, tout << header << std::endl; show_model(tout);).
func (t *Tracker) traceModel(function, file string, line int, header string) error {
	tags := t.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("sls") {
		return nil
	}
	var bodyErr error
	err := tags.EmitConditional("sls", function, file, line, func() bool { return true }, func(out io.Writer) error {
		writer := slsTraceWriter{out}
		if _, bodyErr = io.WriteString(writer, header+"\n"); bodyErr != nil {
			return bodyErr
		}
		bodyErr = t.DisplayModel(writer)
		return bodyErr
	})
	if bodyErr != nil {
		return bodyErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// traceRandomMove is the TRACE("sls") of sls_engine::mk_random_move for a
// bit-vector constant; move is the text after "Random move: ".
func (e *Engine) traceRandomMove(declarations []ast.ASTHandle, move string) error {
	tags := e.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("sls") {
		return nil
	}
	name := func(decl ast.ASTHandle) (string, error) {
		symbol, err := e.manager.GetASTName(decl)
		if err != nil {
			return "", err
		}
		return e.manager.Symbols.Display(symbol)
	}
	var bodyErr error
	err := tags.EmitConditional("sls", "mk_random_move", "src/tactic/sls/sls_engine.cpp", 301, func() bool { return true }, func(out io.Writer) error {
		writer := slsTraceWriter{out}
		_, _ = io.WriteString(writer, "Randomization candidates: ")
		for _, decl := range declarations {
			text, err := name(decl)
			if err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(writer, text+", ")
		}
		_, _ = io.WriteString(writer, "\nRandom move: "+move+"Locally randomized model: \n")
		bodyErr = e.tracker.DisplayModel(writer)
		return bodyErr
	})
	if bodyErr != nil {
		return bodyErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// randomMoveText renders the "Random move: " case of mk_random_move.
func (e *Engine) randomMoveText(choice, bit uint32, decl ast.ASTHandle) (string, error) {
	symbol, err := e.manager.GetASTName(decl)
	if err != nil {
		return "", err
	}
	text, err := e.manager.Symbols.Display(symbol)
	if err != nil {
		return "", err
	}
	switch choice {
	case 0:
		return fmt.Sprintf("Flip #%d in %s\n", bit, text), nil
	case 1:
		return "+1 for " + text + "\n", nil
	default:
		return "-1 for " + text + "\n", nil
	}
}

// traceModelConverter is TRACE("sls_model", mc->display(tout);).
func traceModelConverter(m *ast.ASTManager, mc tactic.ModelConverter) error {
	tags := m.BorrowSourceTraceTags()
	if !tags.ShouldTrace("sls_model") {
		return nil
	}
	var bodyErr error
	err := tags.EmitConditional("sls_model", "operator()", "src/tactic/sls/sls_engine.cpp", 563, func() bool { return true }, func(out io.Writer) error {
		bodyErr = mc.Display(slsTraceWriter{out})
		return bodyErr
	})
	if bodyErr != nil {
		return bodyErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
