package smtcore

// Source: src/smt/theory_bv.cpp:492-505,1603-1661.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// fixedLiteralValue is get_fixed_value(v, result): result, a rational of
// the AST manager's numeral manager owned by the caller, is reset and then
// accumulates m_bb.power(i) for each true bit.
func (t *SMTBitVectorTheory) fixedLiteralValue(variable SMTTheoryVar, result *numeral.Rational) (bool, error) {
	if variable < 0 || int(variable) >= len(t.bitLiterals) {
		return false, ast.ErrASTDomain
	}
	numerals := t.state.manager.NumeralService()
	q := numerals.Manager
	if err := q.Reset(result); err != nil {
		return false, err
	}
	for index, bit := range t.bitLiterals[variable] {
		assigned, err := t.state.Assignment(bit)
		if err != nil {
			return false, err
		}
		if assigned == lbool.Undef {
			return false, nil
		}
		if assigned == lbool.True {
			power, err := numerals.PowerOfTwo(uint32(index))
			if err != nil {
				return false, err
			}
			err = q.Add(result, &power.Raw, result)
			if e := numerals.Del(&power); e != nil && err == nil {
				err = e
			}
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func (t *SMTBitVectorTheory) DisplayVariable(out io.Writer, variable SMTTheoryVar) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	node, err := t.Enode(variable)
	if err != nil {
		return err
	}
	root, err := t.privateRoot(variable)
	if err != nil {
		return err
	}
	rootNode, err := t.Enode(root)
	if err != nil {
		return err
	}
	id, err := node.OwnerID()
	if err != nil {
		return err
	}
	rootID, err := rootNode.OwnerID()
	if err != nil {
		return err
	}
	if err = smtWriteTheoryText(out, fmt.Sprintf("v%-4d #%-4d -> #%-4d, bits:", variable, id, rootID)); err != nil {
		return err
	}
	for _, literal := range t.bitLiterals[variable] {
		if err = smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err = literal.DisplayCompact(smtContextDisplayWriter{out}, s.manager, s.boolExpr); err != nil {
			return err
		}
	}
	q := s.manager.NumeralService().Manager
	value := smtQZero()
	fixed, err := t.fixedLiteralValue(variable, &value)
	if err == nil && fixed {
		var text string
		if text, err = q.String(&value); err == nil {
			err = smtWriteTheoryText(out, ", value: "+text)
		}
	}
	if e := smtQDel(q, &value); e != nil && err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	return smtWriteTheoryText(out, "\n")
}

func (t *SMTBitVectorTheory) Display(out io.Writer) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if t.NumVariables() == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "Theory bv:\n"); err != nil {
		return err
	}
	for v := range t.variables {
		if err := t.DisplayVariable(out, SMTTheoryVar(v)); err != nil {
			return err
		}
	}
	return t.displayBitAtoms(out)
}
