package qe

// Source: src/qe/qe.cpp, search_tree::display, display_node, reset's
// TRACE and get_leaves' TRACE (AUDIT.md qe_05-F6).

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// displayNode is search_tree::display_node. The source prints the node's
// address after "node "; the Go node's address takes its place.
func (s *exprQETree) displayNode(out io.Writer, indent string) error {
	m := s.manager
	write := func(text string) error { _, err := io.WriteString(out, text); return err }
	expression := func(ref ast.ASTRef) error {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		return writeExpression(m, s.config, out, h)
	}
	if err := write(fmt.Sprintf("%snode %p\n", indent, s)); err != nil {
		return err
	}
	if s.selected != (ast.ASTRef{}) {
		if err := write(indent + " var:  "); err != nil {
			return err
		}
		if err := expression(s.selected); err != nil {
			return err
		}
		if err := write("\n"); err != nil {
			return err
		}
	}
	for _, v := range s.variables {
		if err := write(indent + " free: "); err != nil {
			return err
		}
		if err := expression(v); err != nil {
			return err
		}
		if err := write("\n"); err != nil {
			return err
		}
	}
	if s.formula != (ast.ASTRef{}) {
		h, err := m.RefGet(s.formula)
		if err != nil {
			return err
		}
		if !h.IsNull() {
			if err = write(indent + " fml:  "); err != nil {
				return err
			}
			if err = writeExpression(m, s.config, out, h); err != nil {
				return err
			}
			if err = write("\n"); err != nil {
				return err
			}
		}
	}
	if s.definitions != nil {
		size, err := s.definitions.Size()
		if err != nil {
			return err
		}
		for i := uint32(0); i < size; i++ {
			decl, err := s.definitions.Variable(i)
			if err != nil {
				return err
			}
			definition, err := s.definitions.Definition(i)
			if err != nil {
				return err
			}
			name, err := m.GetASTName(decl)
			if err != nil {
				return err
			}
			text, err := m.Symbols.String(name)
			if err != nil {
				return err
			}
			if err = write(indent + " def:  " + text + " = "); err != nil {
				return err
			}
			if err = writeExpression(m, s.config, out, definition); err != nil {
				return err
			}
			if err = write("\n"); err != nil {
				return err
			}
		}
	}
	branches, err := m.NumeralService().Manager.String(&s.branches)
	if err != nil {
		return err
	}
	return write(indent + " branches: " + branches + "\n")
}

// display is search_tree::display(out, indent): the node, then each child
// one space further in.
func (s *exprQETree) display(out io.Writer, indent string) error {
	if err := s.displayNode(out, indent); err != nil {
		return err
	}
	for _, child := range s.children {
		if err := child.display(out, indent+" "); err != nil {
			return err
		}
	}
	return nil
}

// traceAddChild is add_child's TRACE("qe", display_node(tout);
// st->display_node(tout);); line is the add_child overload's site.
func (s *exprQETree) traceAddChild(child *exprQETree, line int) error {
	return qeTrace(s.manager, s.config, "add_child", "src/qe/qe.cpp", line,
		func(out io.Writer) error { return s.displayNode(out, "") },
		func(out io.Writer) error { return child.displayNode(out, "") })
}

// traceReset emits search_tree::reset's TRACE("qe",tout << "resetting\n";)
// for this node and, as reset deallocates each child (whose destructor
// resets it), for every descendant in pre-order.
func (s *exprQETree) traceReset() error {
	if s == nil || s.manager == nil {
		return nil
	}
	if err := traceExprQETreeReset(s.manager, s.config); err != nil {
		return err
	}
	for _, child := range s.children {
		if err := child.traceReset(); err != nil {
			return err
		}
	}
	return nil
}

// resetFreeVars is search_tree::reset_free_vars: TRACE("qe", tout << m_vars
// << "\n";), then the variables are released back to front.
func (s *exprQETree) resetFreeVars() error {
	m := s.manager
	if err := qeTrace(m, s.config, "reset_free_vars", "src/qe/qe.cpp", 1049, func(out io.Writer) error { return writeExpressionVector(m, s.config, out, s.variables) }, "\n"); err != nil {
		return err
	}
	for len(s.variables) > 0 {
		i := len(s.variables) - 1
		if err := m.DelRef(&s.variables[i]); err != nil {
			return err
		}
		s.variables = s.variables[:i]
	}
	return nil
}

// traceExprQETreeReset is one search_tree::reset TRACE record.
func traceExprQETreeReset(m *ast.ASTManager, config Config) error {
	return qeTrace(m, config, "reset", "src/qe/qe.cpp", 1021, "resetting\n")
}

// abstractVariable is search_tree::abstract_variable: expr_abstract over x,
// then m.mk_exists with x's name and the default weight. The result is
// owned by the caller.
func (s *exprQETree) abstractVariable(x, fml ast.ASTHandle) (result ast.ASTRef, err error) {
	m := s.manager
	if result, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return result, err
	}
	err = abstractor.Abstract(0, []ast.ASTHandle{x}, fml, result)
	if err = errors.Join(err, abstractor.Close()); err != nil {
		return result, err
	}
	body, err := m.RefGet(result)
	if err != nil {
		return result, err
	}
	decl, err := m.GetDecl(x)
	if err != nil {
		return result, err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return result, err
	}
	sort, err := m.GetSort(x)
	if err != nil {
		return result, err
	}
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{name}, Body: body})
	if err != nil {
		return result, err
	}
	return result, m.SetRef(result, q)
}

// displayValidate is search_tree::display_validate: for each node with a
// selected variable and children, an SMT2 benchmark asserting that the
// elimination of the variable differs from the disjunction of the
// children, then the same for each child with a formula.
func (s *exprQETree) displayValidate(out io.Writer) (err error) {
	if len(s.children) == 0 {
		return nil
	}
	m := s.manager
	// expr_ref q lives to the end of the function, past the children.
	var q ast.ASTRef
	defer func() {
		if q != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&q))
		}
	}()
	if s.selected != (ast.ASTRef{}) {
		if err = s.displayValidateNode(out, &q); err != nil {
			return err
		}
	}
	for _, child := range s.children {
		if child.formula == (ast.ASTRef{}) {
			continue
		}
		h, e := m.RefGet(child.formula)
		if e != nil {
			return e
		}
		if h.IsNull() {
			continue
		}
		if e = child.displayValidate(out); e != nil {
			return e
		}
	}
	return nil
}

// displayValidateNode is display_validate's if (x) block. Its locals are
// released at the block's end in the source order: fml, then the entries
// of fmls front to back; q is the caller's.
func (s *exprQETree) displayValidateNode(out io.Writer, q *ast.ASTRef) (err error) {
	m := s.manager
	var fmls []ast.ASTRef
	var fml ast.ASTRef
	defer func() {
		if fml != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&fml))
		}
		for i := range fmls {
			err = errors.Join(err, m.DelRef(&fmls[i]))
		}
	}()
	selected, err := m.RefGet(s.selected)
	if err != nil {
		return err
	}
	formula, err := m.RefGet(s.formula)
	if err != nil {
		return err
	}
	if *q, err = s.abstractVariable(selected, formula); err != nil {
		return err
	}
	var forms []ast.ASTHandle
	for _, child := range s.children {
		if child.formula == (ast.ASTRef{}) {
			continue
		}
		h, e := m.RefGet(child.formula)
		if e != nil {
			return e
		}
		if h.IsNull() {
			continue
		}
		current, e := m.NewRef(h)
		if e != nil {
			return e
		}
		// abstract free variables in children.
		childVars := make([]ast.ASTHandle, 0, len(child.variables)+1)
		for _, v := range child.variables {
			x, e := m.RefGet(v)
			if e != nil {
				return errors.Join(e, m.DelRef(&current))
			}
			childVars = append(childVars, x)
		}
		if child.selected != (ast.ASTRef{}) {
			x, e := m.RefGet(child.selected)
			if e != nil {
				return errors.Join(e, m.DelRef(&current))
			}
			childVars = append(childVars, x)
		}
		var newVars []ast.ASTHandle
		for _, v := range childVars {
			i, e := s.variableIndex(v)
			if e != nil {
				return errors.Join(e, m.DelRef(&current))
			}
			if i >= 0 || containsASTHandle(newVars, v) {
				continue
			}
			body, e := m.RefGet(current)
			if e != nil {
				return errors.Join(e, m.DelRef(&current))
			}
			abstracted, e := s.abstractVariable(v, body)
			if e = errors.Join(e, m.DelRef(&current)); e != nil {
				return e
			}
			current = abstracted
			newVars = append(newVars, v)
		}
		fmls = append(fmls, current)
		value, e := m.RefGet(current)
		if e != nil {
			return e
		}
		forms = append(forms, value)
	}
	boolRw, err := rewriter.NewBoolRewriter(m, s.config.Context, s.config.Globals, nil)
	if err != nil {
		return err
	}
	if fml, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return errors.Join(err, boolRw.Close())
	}
	err = boolRw.MkOr(forms, fml)
	if err = errors.Join(err, boolRw.Close()); err != nil {
		return err
	}
	disjunction, err := m.RefGet(fml)
	if err != nil {
		return err
	}
	quantifier, err := m.RefGet(*q)
	if err != nil {
		return err
	}
	// fml = mk_not(m, m.mk_iff(q, fml)).
	iff, err := m.MkIff(quantifier, disjunction)
	if err != nil {
		return err
	}
	iffRef, err := m.NewRef(iff)
	if err != nil {
		return err
	}
	negated, err := m.MkNotSimplified(iff)
	if err == nil {
		err = m.SetRef(fml, negated)
	}
	if err = errors.Join(err, m.DelRef(&iffRef)); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "; eliminate "); err != nil {
		return err
	}
	if err = writeExpression(m, s.config, out, selected); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "\n(push)\n"); err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(m)
	if err != nil {
		return err
	}
	if err = printer.DisplayBenchmark(out, negated, ast.LegacySMTBenchmarkConfig{}); err != nil {
		return err
	}
	_, err = io.WriteString(out, "(pop)\n\n")
	return err
}

func containsASTHandle(list []ast.ASTHandle, h ast.ASTHandle) bool {
	for _, x := range list {
		if x == h {
			return true
		}
	}
	return false
}
