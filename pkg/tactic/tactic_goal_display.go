package tactic

// Source: src/tactic/goal.cpp:402-410.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func goalDisplayWrite(out io.Writer, value string) error {
	n, err := io.WriteString(out, value)
	if err != nil {
		return err
	}
	if n != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

// displaySMT2Formula is ast_smt2_pp(out, h, env, p, indent): a positive
// indent wraps the format in mk_indent, which indents continuation lines.
func (g *Goal) displaySMT2Formula(h ast.ASTHandle, indent uint32) (body string, result error) {
	global := uparams.BorrowProcessGlobalParamsService()
	if global == nil || global.Context == nil {
		return "", uparams.ErrGlobalParamsOwner
	}
	format, err := g.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return "", err
	}
	if indent > 0 {
		if format, err = g.manager.FormatIndent(indent, format); err != nil {
			return "", err
		}
	}
	formats, err := g.manager.GetFormatManager()
	if err != nil {
		return "", err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: g.manager.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := global.Context.EmptyValue()
	if err = g.manager.PrintFormat(stream, format, global.Context, &empty, global); err != nil {
		return "", err
	}
	return stream.String()
}

// Display writes the source default goal printer's formula-only SMT2 form.
// The process parameter owner is borrowed only when a formula is printed.
func (g *Goal) Display(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	if err := goalDisplayWrite(out, "(goal"); err != nil {
		return err
	}
	for _, form := range g.formSlice() {
		if err := goalDisplayWrite(out, "\n  "); err != nil {
			return err
		}
		body, err := g.displaySMT2Formula(form, 2)
		if err != nil {
			return err
		}
		if err = goalDisplayWrite(out, body); err != nil {
			return err
		}
	}
	return goalDisplayWrite(out, ")\n")
}

func goalPrecisionText(value GoalPrecision) (string, error) {
	switch value {
	case GoalPrecise:
		return "precise", nil
	case GoalUnder:
		return "under", nil
	case GoalOver:
		return "over", nil
	case GoalUnderOver:
		return "under-over", nil
	default:
		return "", ast.ErrASTDomain
	}
}

// DisplayWithAttributes is the source ast_printer display profile.
func (g *Goal) DisplayWithAttributes(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	if err := goalDisplayWrite(out, "(goal"); err != nil {
		return err
	}
	for _, form := range g.formSlice() {
		body, err := g.displaySMT2Formula(form, 2)
		if err != nil {
			return err
		}
		if err = goalDisplayWrite(out, "\n  "+body); err != nil {
			return err
		}
	}
	precision, err := goalPrecisionText(g.precision)
	if err != nil {
		return err
	}
	return goalDisplayWrite(out, "\n  :precision "+precision+" :depth "+strconv.FormatUint(uint64(g.depth), 10)+")\n")
}

func (g *Goal) dependencyIsUninterpretedConstant(h ast.ASTHandle) (bool, error) {
	kind, err := g.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := g.manager.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	declaration, err := g.manager.GetDecl(h)
	if err != nil {
		return false, err
	}
	family, err := g.manager.GetFamilyID(declaration)
	return family == ast.NullFamilyID, err
}

func (g *Goal) DisplayWithDependencies(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	if err := goalDisplayWrite(out, "(goal"); err != nil {
		return err
	}
	dependencies := g.dependencySlice()
	for i, form := range g.formSlice() {
		if err := goalDisplayWrite(out, "\n  |-"); err != nil {
			return err
		}
		dependency := ast.ASTDependency{}
		if g.coreEnabled {
			dependency = dependencies[i]
		}
		leaves, err := g.manager.DependencyLinearize(dependency, nil)
		if err != nil {
			return err
		}
		for _, leaf := range leaves {
			constant, err := g.dependencyIsUninterpretedConstant(leaf)
			if err != nil {
				return err
			}
			if constant {
				body, err := g.displaySMT2Formula(leaf, 0)
				if err != nil {
					return err
				}
				if err = goalDisplayWrite(out, " "+body); err != nil {
					return err
				}
			} else {
				id, err := g.manager.GetASTID(leaf)
				if err != nil {
					return err
				}
				if err = goalDisplayWrite(out, " #"+strconv.FormatUint(uint64(id), 10)); err != nil {
					return err
				}
			}
		}
		body, err := g.displaySMT2Formula(form, 2)
		if err != nil {
			return err
		}
		if err = goalDisplayWrite(out, "\n  "+body); err != nil {
			return err
		}
	}
	precision, err := goalPrecisionText(g.precision)
	if err != nil {
		return err
	}
	return goalDisplayWrite(out, "\n  :precision "+precision+" :depth "+strconv.FormatUint(uint64(g.depth), 10)+")\n")
}

func (g *Goal) DisplayAsAnd(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	conjunction, err := g.manager.MkFamilyApp(g.manager.BasicFamilyID(), ast.ASTOpAnd, nil, g.formSlice())
	if err != nil {
		return err
	}
	body, err := g.displaySMT2Formula(conjunction, 0)
	if err != nil {
		return err
	}
	return goalDisplayWrite(out, body+"\n")
}

func (g *Goal) DisplayLowLevel(out io.Writer) (result error) {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	for _, form := range g.formSlice() {
		stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: g.manager.Budget()})
		if err != nil {
			return err
		}
		if err = g.manager.DisplayLowLevel(stream, form, true, true); err != nil {
			return errors.Join(err, stream.Close())
		}
		body, err := stream.String()
		if err != nil {
			return errors.Join(err, stream.Close())
		}
		if err = stream.Close(); err != nil {
			return err
		}
		if err = goalDisplayWrite(out, body+"\n"); err != nil {
			return err
		}
	}
	return nil
}
