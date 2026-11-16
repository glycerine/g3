package smt2

// Source: src/parsers/smt2/smt2parser.cpp parse_define_fun_rec,
// parse_define_funs_rec and parse_rec_fun_decl/body.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type recursiveDeclaration struct {
	name        string
	names       []usymbol.Symbol
	domain      []ast.ASTHandle
	rangeSort   ast.ASTHandle
	declaration ast.ASTHandle
	// bindings is parse_rec_fun_decl's expr_ref_vector of the variables
	// parse_sorted_vars made; declRef is its func_decl_ref f.
	bindings []ast.ASTRef
	declRef  ast.ASTRef
}

// release ends the parser locals of one declaration: f, then bindings
// (front to back).
func (d *recursiveDeclaration) release(m *ast.ASTManager) (result error) {
	if d.declRef != (ast.ASTRef{}) {
		result = m.DelRef(&d.declRef)
	}
	for i := range d.bindings {
		result = errors.Join(result, m.DelRef(&d.bindings[i]))
	}
	d.bindings = nil
	return result
}

func (p *commandParser) recursiveHeader() (d recursiveDeclaration, err error) {
	if _, err = p.astManager(); err != nil {
		return d, err
	}
	if d.name, err = p.symbol(); err != nil {
		return d, err
	}
	if d.names, d.domain, err = p.sortedVariables(); err != nil {
		return d, err
	}
	// parse_sorted_vars makes var(num-1-i) for the i-th variable and keeps
	// it on the expression stack; parse_rec_fun_decl moves them to bindings.
	for i := range d.names {
		v, e := p.manager.MkVar(uint32(len(d.names)-1-i), d.domain[i])
		if e == nil {
			var ref ast.ASTRef
			if ref, e = p.manager.NewRef(v); e == nil {
				d.bindings = append(d.bindings, ref)
			}
		}
		if e != nil {
			return d, errors.Join(e, d.release(p.manager))
		}
	}
	if d.rangeSort, err = p.sortIn("Invalid recursive function definition"); err != nil {
		return d, errors.Join(err, d.release(p.manager))
	}
	// f = m().mk_func_decl(id, num_vars, sorts, range)
	symbol, err := p.manager.Symbols.Intern(d.name)
	if err == nil {
		var f ast.ASTHandle
		if f, err = p.manager.MkFuncDecl(symbol, d.domain, d.rangeSort, ast.ASTFuncDeclInfo{}); err == nil {
			d.declRef, err = p.manager.NewRef(f)
		}
	}
	if err != nil {
		return d, errors.Join(err, d.release(p.manager))
	}
	return d, nil
}

func (p *commandParser) recursiveBody(d recursiveDeclaration) (result error) {
	oldBindings, oldExprs := p.bindings, len(p.locals.refs)
	var err error
	p.bindings, err = p.env.bindSortedVariables(p.manager, d.names, d.domain, p.bindings, &p.locals)
	if err != nil {
		return err
	}
	defer func() {
		p.bindings = oldBindings
		result = errors.Join(result, p.env.endScopeWithOwningCleanup(), p.locals.shrink(p.manager, oldExprs))
	}()
	body, _, err := p.expression()
	if err != nil {
		return err
	}
	return p.context.DefineRecursiveFunction(d.declaration, d.names, body)
}

func (p *commandParser) defineRecursiveFunctions(mutual bool) (result error) {
	var declarations []recursiveDeclaration
	defer func() {
		// The command's locals end: define-fun-rec's f and binding;
		// define-funs-rec's bindings vectors, then decls.
		if !mutual {
			for i := range declarations {
				result = errors.Join(result, declarations[i].release(p.manager))
			}
			return
		}
		for i := range declarations {
			for j := range declarations[i].bindings {
				result = errors.Join(result, p.manager.DelRef(&declarations[i].bindings[j]))
			}
			declarations[i].bindings = nil
		}
		for i := range declarations {
			result = errors.Join(result, declarations[i].release(p.manager))
		}
	}()
	if mutual {
		if err := p.expect(LeftParenToken); err != nil {
			return err
		}
		for p.token != RightParenToken {
			if err := p.expect(LeftParenToken); err != nil {
				return err
			}
			d, err := p.recursiveHeader()
			if err != nil {
				return err
			}
			declarations = append(declarations, d)
			if err := p.expect(RightParenToken); err != nil {
				return err
			}
		}
		if err := p.expect(RightParenToken); err != nil {
			return err
		}
	} else {
		d, err := p.recursiveHeader()
		if err != nil {
			return err
		}
		declarations = append(declarations, d)
	}
	// Publish every declaration before parsing any body, allowing forward and
	// mutual references. Declarations remain installed if a later body fails.
	for i := range declarations {
		d := &declarations[i]
		var err error
		d.declaration, err = p.context.DeclareFunction(d.name, d.domain, d.rangeSort)
		if err != nil {
			return err
		}
	}
	if mutual {
		if err := p.expect(LeftParenToken); err != nil {
			return err
		}
	}
	for _, d := range declarations {
		if mutual && p.token == RightParenToken {
			return fmt.Errorf("the number of declarations does not match number of supplied definitions")
		}
		if err := p.recursiveBody(d); err != nil {
			return err
		}
	}
	if mutual {
		if err := p.expect(RightParenToken); err != nil {
			return err
		}
	}
	return p.requireEnd()
}
