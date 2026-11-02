package duality

// Source: duality_dl_interface.cpp, StatusRefutation display_certificate.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"io"
)

// WriteCertificate appends the source V2 counterexample model, excluding the
// localized quantified variables displayed in the derivation substitutions.
func (c *Counterexample) WriteCertificate(out io.Writer, writeExpr func(io.Writer, ast.ASTHandle) error, factory solver.Factory, params *uparams.ParamsContext, globals *uparams.GlobalParamsService, background ...*Expr) (result error) {
	locals := make(map[ast.ASTHandle]bool)
	if err := c.writeDerivation(out, writeExpr, factory, locals, background); err != nil {
		return err
	}
	g := c.tree
	if g.dualModel == nil {
		return ErrWrapperOwner
	}
	filtered, err := model.NewModelCore(g.context.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, filtered.Close()) }()
	for _, decl := range g.dualModel.ConstantDeclarations {
		if locals[decl] {
			continue
		}
		value, err := g.dualModel.ConstantInterpretation(decl)
		if err != nil {
			return err
		}
		if err := filtered.RegisterConstant(decl, value); err != nil {
			return err
		}
	}
	for i := uint32(0); i < g.dualModel.NumFunctionDeclarations(); i++ {
		decl, err := g.dualModel.Function(i)
		if err != nil {
			return err
		}
		if locals[decl] {
			continue
		}
		value, err := g.dualModel.FunctionInterpretation(decl)
		if err != nil {
			return err
		}
		if err := filtered.CopyFunctionInterpretation(decl, value); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(out, "(model \n\""); err != nil {
		return err
	}
	if err := filtered.WriteV2(out, params, globals, false); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\")\n")
	return err
}
