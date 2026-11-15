package interp

// Source: src/interp/iz3pp.cpp; src/interp/iz3pp.h.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// BadTreeError identifies a malformed tree-interpolation specification.
type BadTreeError struct{ cause Error }

func newBadTreeError() BadTreeError                     { return BadTreeError{cause: NewError("iz3pp_bad_tree")} }
func (e BadTreeError) Error() string                    { return e.cause.Error() }
func (e BadTreeError) Message() string                  { return e.cause.Message() }
func (e BadTreeError) ErrorCode() z3exception.ErrorCode { return e.cause.ErrorCode() }
func (e BadTreeError) HasErrorCode() bool               { return e.cause.HasErrorCode() }

func interpWrite(out io.Writer, text string) error {
	if out == nil {
		return NewError("invalid interpolation output")
	}
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

func printInterpolationTree(m *ast.ASTManager, tree ast.ASTHandle, names map[ast.ASTHandle]string, out io.Writer) error {
	if name, found := names[tree]; found {
		return interpWrite(out, name)
	}
	kind, err := m.GetKind(tree)
	if err != nil || kind != ast.ASTApp {
		return newBadTreeError()
	}
	family, err := m.GetFamilyID(tree)
	if err != nil {
		return err
	}
	op, err := m.GetDeclKind(tree)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() {
		return newBadTreeError()
	}
	switch op {
	case ast.ASTOpAnd:
		if err = interpWrite(out, "(and"); err != nil {
			return err
		}
		count, e := m.GetNumArgs(tree)
		if e != nil {
			return e
		}
		for i := uint32(0); i < count; i++ {
			if err = interpWrite(out, " "); err != nil {
				return err
			}
			arg, e := m.GetArg(tree, i)
			if e != nil {
				return e
			}
			if err = printInterpolationTree(m, arg, names, out); err != nil {
				return err
			}
		}
		return interpWrite(out, ")")
	case ast.ASTOpInterp:
		count, e := m.GetNumArgs(tree)
		if e != nil {
			return e
		}
		if count != 1 {
			return newBadTreeError()
		}
		arg, e := m.GetArg(tree, 0)
		if e != nil {
			return e
		}
		if err = interpWrite(out, "(interp "); err != nil {
			return err
		}
		if err = printInterpolationTree(m, arg, names, out); err != nil {
			return err
		}
		return interpWrite(out, ")")
	default:
		return newBadTreeError()
	}
}

// freeFuncVisitor is iz3pp.cpp's free_func_visitor: every application's
// declaration goes into a func_decl_set, and its sort into an
// obj_hashtable<sort> when the sort has the null family. Both keep the
// source hash-table iteration order.
type freeFuncVisitor struct {
	m     *ast.ASTManager
	funcs *ast.FuncDeclSet
	sorts *ast.ObjMap
}

func (v *freeFuncVisitor) visitApp(n ast.ASTHandle) error {
	decl, err := v.m.GetDecl(n)
	if err != nil {
		return err
	}
	if err = v.funcs.Insert(decl); err != nil {
		return err
	}
	sort, err := v.m.GetSort(n)
	if err != nil {
		return err
	}
	family, err := v.m.GetFamilyID(sort)
	if err != nil {
		return err
	}
	if family == ast.NullFamilyID {
		return v.sorts.Insert(sort, [2]ast.ASTHandle{})
	}
	return nil
}

// forEachExpr is for_each_expr(visitor, visited, e): applications are
// visited after their arguments, left to right; variables and quantifiers
// do nothing in free_func_visitor. visited is shared across calls.
func (v *freeFuncVisitor) forEachExpr(root ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
	if visited[root] {
		return nil
	}
	visited[root] = true
	type frame struct {
		n        ast.ASTHandle
		children []ast.ASTHandle
		next     int
	}
	children := func(n ast.ASTHandle) ([]ast.ASTHandle, ast.ASTKind, error) {
		kind, err := v.m.GetKind(n)
		if err != nil {
			return nil, kind, err
		}
		switch kind {
		case ast.ASTApp:
			count, err := v.m.GetNumArgs(n)
			if err != nil {
				return nil, kind, err
			}
			result := make([]ast.ASTHandle, count)
			for i := range result {
				if result[i], err = v.m.GetArg(n, uint32(i)); err != nil {
					return nil, kind, err
				}
			}
			return result, kind, nil
		case ast.ASTQuantifier:
			// get_child order: body, patterns, no-patterns.
			body, err := v.m.GetQuantifierBody(n)
			if err != nil {
				return nil, kind, err
			}
			result := []ast.ASTHandle{body}
			patterns, err := v.m.GetNumPatterns(n)
			if err != nil {
				return nil, kind, err
			}
			for i := uint32(0); i < patterns; i++ {
				p, err := v.m.GetPattern(n, i)
				if err != nil {
					return nil, kind, err
				}
				result = append(result, p)
			}
			noPatterns, err := v.m.GetNumNoPatterns(n)
			if err != nil {
				return nil, kind, err
			}
			for i := uint32(0); i < noPatterns; i++ {
				p, err := v.m.GetNoPattern(n, i)
				if err != nil {
					return nil, kind, err
				}
				result = append(result, p)
			}
			return result, kind, nil
		case ast.ASTVar:
			return nil, kind, nil
		}
		return nil, kind, ast.ErrASTDomain
	}
	rootChildren, rootKind, err := children(root)
	if err != nil {
		return err
	}
	type entry struct {
		frame
		kind ast.ASTKind
	}
	stack := []entry{{frame{n: root, children: rootChildren}, rootKind}}
	for len(stack) != 0 {
		top := &stack[len(stack)-1]
		if top.next < len(top.children) {
			child := top.children[top.next]
			top.next++
			if visited[child] {
				continue
			}
			visited[child] = true
			grand, kind, err := children(child)
			if err != nil {
				return err
			}
			stack = append(stack, entry{frame{n: child, children: grand}, kind})
			continue
		}
		n, kind := top.n, top.kind
		stack = stack[:len(stack)-1]
		if kind == ast.ASTApp {
			if err = v.visitApp(n); err != nil {
				return err
			}
		}
	}
	return nil
}

// PrettyPrintProblem emits the retained interpolation-specific SMT problem
// (iz3pp with print_low_level set). It borrows the manager and every AST
// handle for the duration of the call.
func PrettyPrintProblem(m *ast.ASTManager, constraints []ast.ASTHandle, tree ast.ASTHandle, out io.Writer) error {
	if m == nil || out == nil {
		return NewError("invalid interpolation pretty-printer")
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return err
	}
	if err := interpWrite(out, "(set-option :produce-interpolants true)\n"); err != nil {
		return err
	}
	visitor := &freeFuncVisitor{m: m, funcs: ast.NewFuncDeclSet(m), sorts: ast.NewObjMap(m)}
	visited := make(map[ast.ASTHandle]bool)
	for _, constraint := range constraints {
		if err := visitor.forEachExpr(constraint, visited); err != nil {
			return err
		}
	}
	// Name all the constraints first: a constraint listed twice prints
	// its last name in both asserts.
	names := make(map[ast.ASTHandle]string, len(constraints))
	for i, constraint := range constraints {
		names[constraint] = fmt.Sprintf("f!%d", i+1)
	}
	printer, err := ast.NewLegacySMTPrinter(m)
	if err != nil {
		return err
	}
	for _, entry := range visitor.sorts.Entries() {
		if err = printer.DisplayAST(out, entry.Key, 0, nil); err != nil {
			return err
		}
	}
	for _, decl := range visitor.funcs.Keys() {
		family, e := m.GetFamilyID(decl)
		if e != nil {
			return e
		}
		if family != ast.NullFamilyID {
			continue
		}
		if err = printer.DisplayAST(out, decl, 0, nil); err != nil {
			return err
		}
		if err = interpWrite(out, "\n"); err != nil {
			return err
		}
	}
	for _, constraint := range constraints {
		if err = interpWrite(out, "(assert (! "); err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, constraint, nil); err != nil {
			return err
		}
		if err = interpWrite(out, " :named "+names[constraint]+"))\n"); err != nil {
			return err
		}
	}
	if err = interpWrite(out, "(check-sat)\n(get-interpolant "); err != nil {
		return err
	}
	if err = printInterpolationTree(m, tree, names, out); err != nil {
		return err
	}
	return interpWrite(out, ")\n")
}
