package cmd_context

// Source: src/cmd_context/cmd_context.cpp find_func_decl overloads.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// FindFunctionDeclaration resolves an SMT-LIB declaration reference. A nil
// domain denotes a bare name; a non-nil domain supplies an exact signature.
// The caller must pin the borrowed result before creating other ASTs.
func (c *Context) FindFunctionDeclaration(name string, indices []uint32, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (declaration ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return declaration, err
	}
	if builtins := c.builtins[name]; len(builtins) != 0 {
		op := builtins[0]
		// Source hack: only a nonzero first kind with a nonempty domain
		// consults domain[0]'s family.
		if op.kind != 0 && len(domain) > 0 {
			family, e := m.GetFamilyID(domain[0])
			if e != nil {
				return declaration, e
			}
			for _, candidate := range builtins {
				if candidate.family == family {
					op = candidate
					break
				}
			}
		}
		parameters := make([]ast.ASTParameter, len(indices))
		defer func() {
			for i := range parameters {
				if parameters[i] != (ast.ASTParameter{}) {
					err = errors.Join(err, m.DelParameter(&parameters[i]))
				}
			}
		}()
		for i, index := range indices {
			parameters[i], err = m.NewParameterUint(index)
			if err != nil {
				return declaration, err
			}
		}
		declaration, err = m.MkFamilyFuncDecl(op.family, op.kind, parameters, domain, rangeSort)
		if domain == nil {
			// The bare form catches ast_exception and reports one message.
			if err != nil || declaration.IsNull() {
				return ast.ASTHandle{}, fmt.Errorf("invalid function declaration reference, must provide signature for builtin symbol %s", name)
			}
			return declaration, nil
		}
		if err == nil && declaration.IsNull() {
			return declaration, fmt.Errorf("invalid function declaration reference, invalid builtin reference %s", name)
		}
		return declaration, err
	}
	for _, macro := range c.macros[name] {
		matches := domain == nil
		if domain != nil && len(macro.domain) == len(domain) {
			matches = true
			for i, sort := range domain {
				matches = matches && macro.domain[i] == sort
			}
		}
		if matches {
			return declaration, fmt.Errorf("invalid function declaration reference, named expressions (aka macros) cannot be referenced %s", name)
		}
	}
	if len(indices) != 0 {
		return declaration, fmt.Errorf("invalid indexed function declaration reference, unknown builtin function %s", name)
	}
	// func_decls::find returns a sole declaration without consulting the
	// signature; only overloaded names are matched by range, arity, domain.
	candidates := c.functions[name]
	if domain == nil {
		if len(candidates) > 1 {
			return declaration, fmt.Errorf("ambiguous function declaration reference, provide full signature to disumbiguate (<symbol> (<sort>*) <sort>) %s", name)
		}
	} else if len(candidates) > 1 {
		for _, candidate := range candidates {
			rangeValue, e := m.GetRange(candidate)
			if e != nil {
				return declaration, e
			}
			if rangeValue != rangeSort {
				continue
			}
			arity, e := m.GetArity(candidate)
			if e != nil {
				return declaration, e
			}
			if int(arity) != len(domain) {
				continue
			}
			matches := true
			for i, sort := range domain {
				actual, e := m.GetDomain(candidate, uint32(i))
				if e != nil {
					return declaration, e
				}
				if actual != sort {
					matches = false
					break
				}
			}
			if matches {
				return candidate, nil
			}
		}
		return declaration, fmt.Errorf("invalid function declaration reference, unknown function %s", name)
	}
	if len(candidates) == 0 {
		return declaration, fmt.Errorf("invalid function declaration reference, unknown function %s", name)
	}
	return candidates[0], nil
}

// IsNullaryFunction reports whether a bare name resolves to one zero-arity
// declaration. Source parse_pattern uses find_func_decl(name) before it parses
// the rest of a unary pattern so it can discard legacy constant patterns.
func (c *Context) IsNullaryFunction(name string) (bool, error) {
	m, err := c.Manager()
	if err != nil {
		return false, err
	}
	if builtins := c.builtins[name]; len(builtins) != 0 {
		declaration, cause := m.MkFamilyFuncDecl(builtins[0].family, builtins[0].kind, nil, nil, ast.ASTHandle{})
		if cause != nil || declaration.IsNull() {
			// C++ find_func_decl catches AST declaration refusal as an ordinary
			// command lookup failure, then parses the name as a nonconstant.
			return false, nil
		}
		arity, cause := m.GetArity(declaration)
		return cause == nil && arity == 0, cause
	}
	if c.containsMacro(name, nil) || len(c.functions[name]) != 1 {
		return false, nil
	}
	arity, err := m.GetArity(c.functions[name][0])
	return arity == 0, err
}
