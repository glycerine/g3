package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) ProjectModelFormulaNNF(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, config Config) (bool, error) {
	root, e := p.manager.RefGet(formula)
	if e != nil {
		return false, e
	}
	atoms, e := CollectArithmeticAtomsNNF(p.manager, root)
	if e != nil {
		return false, e
	}
	defer atoms.Close()
	return p.ProjectModelAtoms(variable, source, formula, atoms, config)
}

// ProjectModelFormulaTables projects with the solver context's atom sets,
// given in obj_hashtable iteration order, as arith_plugin::project does.
func (p *ArithmeticPlugin) ProjectModelFormulaTables(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, positive, negative []ast.ASTHandle, config Config) (bool, error) {
	root, e := p.manager.RefGet(formula)
	if e != nil {
		return false, e
	}
	ok, e := p.Prepare(variable, root, positive, negative)
	if e != nil || !ok {
		return false, e
	}
	return p.ProjectModel(variable, source, formula, config)
}
