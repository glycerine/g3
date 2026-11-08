package model

// Source: src/model/model_pp.cpp; src/model/model_smt2_pp.cpp.

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usmt2 "github.com/glycerine/g3/pkg/util/smt2"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type modelPrinter struct {
	model   *ModelCore
	out     io.Writer
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
}

func (p *modelPrinter) write(text string) error {
	_, err := io.WriteString(p.out, text)
	return err
}

func (p *modelPrinter) indent(n uint32) error {
	return p.write(strings.Repeat(" ", int(n)))
}

func (p *modelPrinter) render(h ast.ASTHandle, names []usymbol.Symbol) (text string, result error) {
	format, err := p.model.manager.SMT2PPExprTree(h, names)
	if err != nil {
		return "", err
	}
	formats, err := p.model.manager.GetFormatManager()
	if err != nil {
		return "", err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.model.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := p.context.EmptyValue()
	if err = p.model.manager.PrintFormat(stream, format, p.context, &empty, p.globals); err != nil {
		return "", err
	}
	return stream.String()
}

// renderSort is source cmd_context::display(out, s, indent): a positive
// indent wraps the sort format in mk_indent before printing.
func (p *modelPrinter) renderSort(h ast.ASTHandle, indent uint32) (text string, result error) {
	format, err := p.model.manager.SMT2PPSort(h)
	if err != nil {
		return "", err
	}
	formats, err := p.model.manager.GetFormatManager()
	if err != nil {
		return "", err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	if indent > 0 {
		if format, err = p.model.manager.FormatIndent(indent, format); err != nil {
			return "", err
		}
		indented, err := formats.NewRef(format)
		if err != nil {
			return "", err
		}
		defer func() { result = errors.Join(result, formats.DelRef(&indented)) }()
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.model.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := p.context.EmptyValue()
	if err = p.model.manager.PrintFormat(stream, format, p.context, &empty, p.globals); err != nil {
		return "", err
	}
	return stream.String()
}

func (p *modelPrinter) rawName(d ast.ASTHandle) (string, error) {
	name, err := p.model.manager.GetASTName(d)
	if err != nil {
		return "", err
	}
	return p.model.manager.Symbols.String(name)
}

func (p *modelPrinter) smt2Name(d ast.ASTHandle) (string, error) {
	name, err := p.model.manager.GetASTName(d)
	if err != nil {
		return "", err
	}
	quoted, err := usmt2.IsSMT2QuotedSymbol(p.model.manager.Symbols, name)
	if err != nil {
		return "", err
	}
	if quoted {
		return usmt2.MkSMT2QuotedSymbol(p.model.manager.Symbols, name, nil)
	}
	return p.model.manager.Symbols.String(name)
}

func newModelPrinter(md *ModelCore, out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*modelPrinter, error) {
	if err := md.open(); err != nil {
		return nil, err
	}
	if out == nil || context == nil || globals == nil || globals.Context != context || context.SymbolsField != md.manager.Symbols {
		return nil, ast.ErrASTHandle
	}
	return &modelPrinter{model: md, out: out, context: context, globals: globals}, nil
}

// WriteDebug implements model_pp's compact debugging syntax in registration
// order. The parameter services are borrowed for the AST pretty printer.
func (md *ModelCore) WriteDebug(out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) error {
	p, err := newModelPrinter(md, out, context, globals)
	if err != nil {
		return err
	}
	if md.Derived != nil {
		for _, sort := range md.Derived.sorts {
			sortText, e := p.renderSort(sort, 0)
			if e != nil {
				return e
			}
			if e = p.write("(define-sort " + sortText); e != nil {
				return e
			}
			u, found := md.Derived.Universes.Get2(sort)
			if !found || u == nil {
				return ast.ErrASTDomain
			}
			for _, value := range u.values {
				text, e := p.render(value, nil)
				if e != nil {
					return e
				}
				if e = p.write(" " + text); e != nil {
					return e
				}
			}
			if e = p.write(")\n"); e != nil {
				return e
			}
		}
	}
	for _, decl := range md.ConstantDeclarations {
		name, e := p.rawName(decl)
		if e != nil {
			return e
		}
		value, e := md.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		text, e := p.render(value, nil)
		if e != nil {
			return e
		}
		if e = p.write("(define " + name + " " + text + ")\n"); e != nil {
			return e
		}
	}
	for _, decl := range md.functionDeclarations {
		name, e := p.rawName(decl)
		if e != nil {
			return e
		}
		fi, e := md.FunctionInterpretation(decl)
		if e != nil || fi == nil {
			if e != nil {
				return e
			}
			return ast.ErrASTPending
		}
		vars, exprNames, e := p.functionNames(fi.arity, 0)
		if e != nil {
			return e
		}
		if e = p.write("(define (" + name); e != nil {
			return e
		}
		for _, variable := range vars {
			if e = p.write(" " + variable); e != nil {
				return e
			}
		}
		if e = p.write(")\n"); e != nil {
			return e
		}
		for _, entry := range fi.entries {
			if e = p.write("  (if "); e != nil {
				return e
			}
			if fi.arity > 1 {
				if e = p.write("(and "); e != nil {
					return e
				}
			}
			for j, variable := range vars {
				arg, x := entry.Argument(uint32(j))
				if x != nil {
					return x
				}
				text, x := p.render(arg, nil)
				if x != nil {
					return x
				}
				if e = p.write("(= " + variable + " " + text + ")"); e != nil {
					return e
				}
				if j+1 < len(vars) {
					if e = p.write(" "); e != nil {
						return e
					}
				}
			}
			if fi.arity > 1 {
				if e = p.write(")"); e != nil {
					return e
				}
			}
			result, x := entry.Result()
			if x != nil {
				return x
			}
			text, x := p.render(result, nil)
			if x != nil {
				return x
			}
			if e = p.write(" " + text + "\n"); e != nil {
				return e
			}
		}
		if len(fi.entries) > 0 {
			if e = p.write("  "); e != nil {
				return e
			}
		}
		if fi.fallback.IsNull() {
			if e = p.write("  #unspecified"); e != nil {
				return e
			}
		} else {
			text, x := p.render(fi.fallback, exprNames)
			if x != nil {
				return x
			}
			if e = p.write("  " + text); e != nil {
				return e
			}
		}
		if e = p.write(strings.Repeat(")", len(fi.entries)) + ")\n"); e != nil {
			return e
		}
	}
	return nil
}

func (p *modelPrinter) functionNames(arity uint32, first uint32) ([]string, []usymbol.Symbol, error) {
	vars := make([]string, arity)
	names := make([]usymbol.Symbol, arity)
	for i := uint32(0); i < arity; i++ {
		vars[i] = "x!" + strconv.FormatUint(uint64(i+first), 10)
		s, err := p.model.manager.Symbols.Intern(vars[i])
		if err != nil {
			return nil, nil, err
		}
		names[arity-i-1] = s
	}
	return vars, names, nil
}

func (md *ModelCore) sortedFunctions() ([]ast.ASTHandle, error) {
	visited := make(map[ast.ASTHandle]bool)
	result := make([]ast.ASTHandle, 0, len(md.functionDeclarations))
	for _, root := range md.functionDeclarations {
		if visited[root] {
			continue
		}
		visited[root] = true
		stack := []ast.ASTHandle{root}
		for len(stack) != 0 {
			current := stack[len(stack)-1]
			fi, err := md.FunctionInterpretation(current)
			if err != nil || fi == nil {
				if err != nil {
					return nil, err
				}
				return nil, ast.ErrASTPending
			}
			added := false
			if !fi.fallback.IsNull() {
				deps := make(ast.FuncDeclSet)
				if err = md.manager.CollectFuncDecls(fi.fallback, deps, false); err != nil {
					return nil, err
				}
				// Native map order is deliberately not observable. Visit model
				// declarations in registration order while preserving the source
				// dependency-before-dependent postorder.
				for _, candidate := range md.functionDeclarations {
					if _, found := deps[candidate]; found && !visited[candidate] {
						visited[candidate] = true
						stack = append(stack, candidate)
						added = true
					}
				}
			}
			if added {
				continue
			}
			stack = stack[:len(stack)-1]
			result = append(result, current)
		}
	}
	return result, nil
}

// WriteSMT2 implements model_smt2_pp, including universe declarations and
// cardinality constraints, constants, and dependency-ordered functions.
func (md *ModelCore) WriteSMT2(out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, indent uint32) error {
	p, err := newModelPrinter(md, out, context, globals)
	if err != nil {
		return err
	}
	if md.Derived != nil {
		for _, sort := range md.Derived.sorts {
			u, found := md.Derived.Universes.Get2(sort)
			if !found || u == nil || len(u.values) == 0 {
				return ast.ErrASTDomain
			}
			sortText, e := p.renderSort(sort, 13)
			if e != nil {
				return e
			}
			names := make([]string, len(u.values))
			for i, value := range u.values {
				decl, x := md.manager.GetDecl(value)
				if x != nil {
					return x
				}
				names[i], x = p.smt2Name(decl)
				if x != nil {
					return x
				}
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; universe for " + sortText + ":\n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";;   " + strings.Join(names, " ") + " \n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; -----------\n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; definitions for universe elements:\n"); e != nil {
				return e
			}
			for _, name := range names {
				if e = p.indent(indent); e != nil {
					return e
				}
				rangeText, e := p.renderSort(sort, indent+uint32(len(name))+16)
				if e != nil {
					return e
				}
				if e = p.write("(declare-fun " + name + " () " + rangeText + ")\n"); e != nil {
					return e
				}
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; cardinality constraint:\n"); e != nil {
				return e
			}
			conds := make([]string, len(names))
			for i, name := range names {
				conds[i] = "(= x " + name + ")"
			}
			condition := conds[0]
			if len(conds) > 1 {
				condition = "(or " + strings.Join(conds, " ") + ")"
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write("(forall ((x " + sortText + ")) " + condition + ")\n"); e != nil {
				return e
			}
			if e = p.indent(indent); e != nil {
				return e
			}
			if e = p.write(";; -----------\n"); e != nil {
				return e
			}
		}
	}
	for _, decl := range md.ConstantDeclarations {
		name, e := p.smt2Name(decl)
		if e != nil {
			return e
		}
		rangeSort, e := md.manager.GetRange(decl)
		if e != nil {
			return e
		}
		sortText, e := p.renderSort(rangeSort, indent+uint32(len(name))+16)
		if e != nil {
			return e
		}
		value, e := md.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		valueText, e := p.render(value, nil)
		if e != nil {
			return e
		}
		if e = p.indent(indent); e != nil {
			return e
		}
		if e = p.write("(define-fun " + name + " () " + sortText + "\n"); e != nil {
			return e
		}
		if e = p.indent(indent + 2); e != nil {
			return e
		}
		if e = p.write(valueText + ")\n"); e != nil {
			return e
		}
	}
	functions, err := md.sortedFunctions()
	if err != nil {
		return err
	}
	for _, decl := range functions {
		if e := p.writeFunctionSMT2(decl, indent); e != nil {
			return e
		}
	}
	return nil
}

// formatRefs holds format_ref locals; release follows reverse declaration.
type formatRefs struct {
	formats *ast.ASTManager
	refs    []ast.ASTRef
}

func (r *formatRefs) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return h, err
	}
	ref, err := r.formats.NewRef(h)
	if err != nil {
		return h, err
	}
	r.refs = append(r.refs, ref)
	return h, nil
}

func (r *formatRefs) release() (err error) {
	for i := len(r.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, r.formats.DelRef(&r.refs[i]))
	}
	r.refs = nil
	return err
}

func (p *modelPrinter) printFormat(format ast.ASTHandle) (text string, result error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: p.model.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := p.context.EmptyValue()
	if err = p.model.manager.PrintFormat(stream, format, p.context, &empty, p.globals); err != nil {
		return "", err
	}
	return stream.String()
}

// writeFunctionSMT2 is one iteration of model_smt2_pp.cpp pp_funs. The
// nested format calls are built in GCC's right-to-left argument order.
func (p *modelPrinter) writeFunctionSMT2(decl ast.ASTHandle, indent uint32) (result error) {
	md := p.model
	m := md.manager
	formats, err := m.GetFormatManager()
	if err != nil {
		return err
	}
	fi, err := md.FunctionInterpretation(decl)
	if err != nil || fi == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTPending
	}
	// format_ref body, f_range, def: destroyed in reverse order.
	var body, rangeRef, def ast.ASTRef
	defer func() {
		result = errors.Join(result, formats.DelRef(&def), formats.DelRef(&rangeRef), formats.DelRef(&body))
	}()
	if body, err = formats.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	if rangeRef, err = formats.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	if def, err = formats.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	partial := fi.fallback.IsNull()
	first := uint32(0)
	if partial {
		first = 1 // "x!" << (j+1)
	}
	vars, exprNames, err := p.functionNames(fi.arity, first)
	if err != nil {
		return err
	}
	var bodyFormat ast.ASTHandle
	if partial {
		bodyFormat, err = m.FormatString("#unspecified")
	} else {
		bodyFormat, err = m.SMT2PPExprTree(fi.fallback, exprNames)
	}
	if err != nil {
		return err
	}
	if err = formats.SetRef(body, bodyFormat); err != nil {
		return err
	}
	varNames := make([]ast.ASTHandle, fi.arity)
	for i := range varNames {
		if varNames[i], err = m.FormatString(vars[i]); err != nil {
			return err
		}
	}
	argDecls := make([]ast.ASTHandle, fi.arity)
	for i := uint32(0); i < fi.arity; i++ {
		domain, err := m.GetDomain(decl, i)
		if err != nil {
			return err
		}
		local := formatRefs{formats: formats}
		domainFormat, err := local.hold(m.SMT2PPSort(domain))
		if err == nil {
			argDecls[i], err = m.FormatSeq5([]ast.ASTHandle{varNames[i], domainFormat})
		}
		if err = errors.Join(err, local.release()); err != nil {
			return err
		}
	}
	domainFormat, err := m.FormatSeq5(argDecls)
	if err != nil {
		return err
	}
	rangeSort, err := m.GetRange(decl)
	if err != nil {
		return err
	}
	rangeFormat, err := m.SMT2PPSort(rangeSort)
	if err != nil {
		return err
	}
	if err = formats.SetRef(rangeRef, rangeFormat); err != nil {
		return err
	}
	if len(fi.entries) > 0 {
		var entries []ast.ASTHandle
		for i, entry := range fi.entries {
			conds := make([]ast.ASTHandle, fi.arity)
			for j := uint32(0); j < fi.arity; j++ {
				arg, err := entry.Argument(j)
				if err != nil {
					return err
				}
				local := formatRefs{formats: formats}
				argFormat, err := local.hold(m.SMT2PPExprTree(arg, nil))
				if err == nil {
					conds[j], err = m.FormatSeq1("=", []ast.ASTHandle{varNames[j], argFormat})
				}
				if err = errors.Join(err, local.release()); err != nil {
					return err
				}
			}
			cond := conds[0]
			if len(conds) > 1 {
				if cond, err = m.FormatSeq1("and", conds); err != nil {
					return err
				}
			}
			value, err := entry.Result()
			if err != nil {
				return err
			}
			local := formatRefs{formats: formats}
			resultFormat, err := local.hold(m.SMT2PPExprTree(value, nil))
			if err == nil && i > 0 {
				var lb ast.ASTHandle
				if lb, err = m.FormatLineBreak(); err == nil {
					entries = append(entries, lb)
				}
			}
			var group ast.ASTHandle
			if err == nil {
				// mk_group(mk_compose(mk_string("(ite "), mk_indent(5, cond),
				//                     mk_indent(TAB_SZ, mk_compose(mk_line_break, f_result))))
				var lb, inner, resultIndent, condIndent, head, whole ast.ASTHandle
				lb, err = m.FormatLineBreak()
				if err == nil {
					inner, err = m.FormatCompose([]ast.ASTHandle{lb, resultFormat})
				}
				if err == nil {
					resultIndent, err = m.FormatIndent(2, inner)
				}
				if err == nil {
					condIndent, err = m.FormatIndent(5, cond)
				}
				if err == nil {
					head, err = m.FormatString("(ite ")
				}
				if err == nil {
					whole, err = m.FormatCompose([]ast.ASTHandle{head, condIndent, resultIndent})
				}
				if err == nil {
					group, err = m.FormatGroup(whole)
				}
			}
			if err = errors.Join(err, local.release()); err != nil {
				return err
			}
			entries = append(entries, group)
		}
		lb, err := m.FormatLineBreak()
		if err != nil {
			return err
		}
		elseCompose, err := m.FormatCompose([]ast.ASTHandle{lb, bodyFormat})
		if err != nil {
			return err
		}
		elseIndent, err := m.FormatIndent(2, elseCompose)
		if err != nil {
			return err
		}
		entries = append(entries, elseIndent)
		for range fi.entries {
			closing, err := m.FormatString(")")
			if err != nil {
				return err
			}
			entries = append(entries, closing)
		}
		if bodyFormat, err = m.FormatCompose(entries); err != nil {
			return err
		}
		if err = formats.SetRef(body, bodyFormat); err != nil {
			return err
		}
	}
	name, err := p.smt2Name(decl)
	if err != nil {
		return err
	}
	// mk_indent(indent, mk_compose(mk_compose("(define-fun ", fname, " ",
	//   mk_compose(f_domain, " ", f_range)), mk_indent(TAB_SZ, mk_compose(lb, body, ")"))))
	closing, err := m.FormatString(")")
	if err != nil {
		return err
	}
	lb, err := m.FormatLineBreak()
	if err != nil {
		return err
	}
	tailCompose, err := m.FormatCompose([]ast.ASTHandle{lb, bodyFormat, closing})
	if err != nil {
		return err
	}
	tail, err := m.FormatIndent(2, tailCompose)
	if err != nil {
		return err
	}
	space, err := m.FormatString(" ")
	if err != nil {
		return err
	}
	signature, err := m.FormatCompose([]ast.ASTHandle{domainFormat, space, rangeFormat})
	if err != nil {
		return err
	}
	space2, err := m.FormatString(" ")
	if err != nil {
		return err
	}
	fname, err := m.FormatString(name)
	if err != nil {
		return err
	}
	opening, err := m.FormatString("(define-fun ")
	if err != nil {
		return err
	}
	header, err := m.FormatCompose([]ast.ASTHandle{opening, fname, space2, signature})
	if err != nil {
		return err
	}
	whole, err := m.FormatCompose([]ast.ASTHandle{header, tail})
	if err != nil {
		return err
	}
	defFormat, err := m.FormatIndent(indent, whole)
	if err != nil {
		return err
	}
	if err = formats.SetRef(def, defFormat); err != nil {
		return err
	}
	text, err := p.printFormat(defFormat)
	if err != nil {
		return err
	}
	if err = p.indent(indent); err != nil {
		return err
	}
	return p.write(text + "\n")
}
