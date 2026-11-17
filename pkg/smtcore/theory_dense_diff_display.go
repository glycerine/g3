package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:663-706.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Display retains theory_dense_diff_logic::display and display_atom.
func (t *SMTDenseDiffTheory) Display(out io.Writer) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err := smtWriteTheoryText(out, "Theory dense difference logic:\n"); err != nil {
		return err
	}
	if err := t.DisplayVariableEnodes(out); err != nil {
		return err
	}
	var rows [][]int
	if t.matrix != nil {
		rows = t.matrix.rows
	}
	for source, row := range rows {
		for target, edge := range row {
			if edge <= 0 {
				continue
			}
			sourceID, err := t.variables[source].OwnerID()
			if err != nil {
				return err
			}
			targetID, err := t.variables[target].OwnerID()
			if err != nil {
				return err
			}
			if err = smtWriteTheoryText(out, fmt.Sprintf("#%-5d -- %s : id%-5d --> #%d\n", sourceID, t.ext.distText(source, target, 10), edge, targetID)); err != nil {
				return err
			}
		}
	}
	if err := smtWriteTheoryText(out, "atoms:\n"); err != nil {
		return err
	}
	for index := range t.atoms {
		text, err := t.displayAtom(index)
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, text); err != nil {
			return err
		}
	}
	return nil
}

// displayAtom is theory_dense_diff_logic::display_atom.
func (t *SMTDenseDiffTheory) displayAtom(index int) (string, error) {
	atom := &t.atoms[index]
	targetID, err := t.variables[atom.target].OwnerID()
	if err != nil {
		return "", err
	}
	sourceID, err := t.variables[atom.source].OwnerID()
	if err != nil {
		return "", err
	}
	assignment, err := t.state.Assignment(NewSMTLiteral(atom.variable, false))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("#%-5d - #%-5d <= %s        assignment: %s\n", targetID, sourceID, t.ext.atomText(index, 10), assignment), nil
}

// displayAtomText is display_atom for a trace operand.
func (t *SMTDenseDiffTheory) displayAtomText(index int) string {
	text, _ := t.displayAtom(index)
	return text
}
