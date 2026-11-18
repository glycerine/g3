package util

import (
	"fmt"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

type PatternValidator struct {
	manager      *ast.ASTManager
	basic, label ast.FamilyID
	warnings     io.Writer
}

func NewPatternValidator(manager *ast.ASTManager, warnings io.Writer) (*PatternValidator, error) {
	if manager == nil {
		return nil, fmt.Errorf("parser: nil AST manager")
	}
	name, err := manager.Symbols.Intern("label")
	if err != nil {
		return nil, err
	}
	label, err := manager.FamilyIDForSymbol(name)
	if err != nil {
		return nil, err
	}
	if warnings == nil {
		warnings = io.Discard
	}
	return &PatternValidator{manager: manager, basic: manager.BasicFamilyID(), label: label, warnings: warnings}, nil
}

func (v *PatternValidator) warn(line, position uint32, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(v.warnings, "(%d,%d): "+format+"\n", append([]interface{}{line, position}, args...)...)
}

func (v *PatternValidator) ValidateNew(numNewBindings uint32, expression ast.ASTHandle, line, position uint32) (bool, error) {
	return v.Validate(math.MaxUint32, numNewBindings, expression, line, position)
}

func (v *PatternValidator) Validate(numBindings, numNewBindings uint32, expression ast.ASTHandle, line, position uint32) (bool, error) {
	kind, err := v.manager.GetKind(expression)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		v.warn(line, position, "invalid pattern: variable.")
		return false, nil
	}
	found := make(map[uint32]bool)
	foundAny := false
	valid := true
	// pattern_validation_functor's procs.
	procVar := func(current ast.ASTHandle) error {
		index, readErr := v.manager.GetVarIndex(current)
		if readErr != nil {
			return readErr
		}
		if index >= numBindings {
			v.warn(line, position, "free variables cannot be used in patterns.")
			valid = false
		} else if index < numNewBindings {
			foundAny = true
			found[index] = true
		}
		return nil
	}
	procApp := func(current ast.ASTHandle) error {
		family, readErr := v.manager.GetFamilyID(current)
		if readErr != nil {
			return readErr
		}
		declKind, readErr := v.manager.GetDeclKind(current)
		if readErr != nil {
			return readErr
		}
		if family == v.label || family == v.basic && declKind != ast.ASTOpEq && declKind != ast.ASTOpTrue && declKind != ast.ASTOpFalse {
			decl, declErr := v.manager.GetDecl(current)
			if declErr != nil {
				return declErr
			}
			name, nameErr := v.manager.GetASTName(decl)
			if nameErr != nil {
				return nameErr
			}
			text, textErr := v.manager.Symbols.String(name)
			if textErr != nil {
				return textErr
			}
			v.warn(line, position, "'%s' cannot be used in patterns.", text)
			valid = false
		}
		return nil
	}
	if err = v.forEachExpr(expression, procVar, procApp, func() { valid = false }); err != nil {
		return false, err
	}
	if !valid {
		return false, nil
	}
	if !foundAny {
		v.warn(line, position, "pattern does not contain any variable.")
		return false, nil
	}
	if uint32(len(found)) != numNewBindings {
		v.warn(line, position, "pattern does not contain all quantified variables.")
		return false, nil
	}
	return true, nil
}

// forEachExpr is for_each_expr(f, n): a post-order walk that visits each
// shared node once (for_each_expr_core marks nodes whose ref count is above
// one, which are exactly the nodes a tree walk could meet twice) and enters
// quantifiers (body, patterns, no-patterns).
func (v *PatternValidator) forEachExpr(root ast.ASTHandle, procVar, procApp func(ast.ASTHandle) error, procQuantifier func()) error {
	type frame struct {
		node ast.ASTHandle
		next uint32
	}
	visited := map[ast.ASTHandle]bool{root: true}
	children := func(h ast.ASTHandle, kind ast.ASTKind) (uint32, func(uint32) (ast.ASTHandle, error), error) {
		if kind == ast.ASTApp {
			n, e := v.manager.GetNumArgs(h)
			return n, func(i uint32) (ast.ASTHandle, error) { return v.manager.GetArg(h, i) }, e
		}
		patterns, e := v.manager.GetNumPatterns(h)
		if e != nil {
			return 0, nil, e
		}
		noPatterns, e := v.manager.GetNumNoPatterns(h)
		if e != nil {
			return 0, nil, e
		}
		return 1 + patterns + noPatterns, func(i uint32) (ast.ASTHandle, error) {
			if i == 0 {
				return v.manager.GetQuantifierBody(h)
			}
			if i <= patterns {
				return v.manager.GetPattern(h, i-1)
			}
			return v.manager.GetNoPattern(h, i-1-patterns)
		}, nil
	}
	stack := []frame{{node: root}}
	for len(stack) != 0 {
		top := &stack[len(stack)-1]
		kind, err := v.manager.GetKind(top.node)
		if err != nil {
			return err
		}
		if kind == ast.ASTVar {
			stack = stack[:len(stack)-1]
			if err = procVar(top.node); err != nil {
				return err
			}
			continue
		}
		count, child, err := children(top.node, kind)
		if err != nil {
			return err
		}
		pushed := false
		for top.next < count {
			arg, e := child(top.next)
			if e != nil {
				return e
			}
			top.next++
			if visited[arg] {
				continue
			}
			visited[arg] = true
			argKind, e := v.manager.GetKind(arg)
			if e != nil {
				return e
			}
			if argKind == ast.ASTVar {
				if e = procVar(arg); e != nil {
					return e
				}
				continue
			}
			if argKind == ast.ASTApp {
				n, e := v.manager.GetNumArgs(arg)
				if e != nil {
					return e
				}
				if n == 0 {
					if e = procApp(arg); e != nil {
						return e
					}
					continue
				}
			}
			stack = append(stack, frame{node: arg})
			pushed = true
			break
		}
		if pushed {
			continue
		}
		node := top.node
		stack = stack[:len(stack)-1]
		if kind == ast.ASTApp {
			if err = procApp(node); err != nil {
				return err
			}
		} else {
			procQuantifier()
		}
	}
	return nil
}
