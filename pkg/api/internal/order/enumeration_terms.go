package order

import (
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

type enumerationWriter struct {
	dependencies *predicateDependencies
	changed      bool
	language     *enumerationLanguage
	scope        []enumerationBinder
	visits       int
	binders      int
}

type enumerationBinder struct {
	sort        ast.ASTHandle
	replacement string
}

const maxEnumerationBytes = 3_000_000

func (w *enumerationWriter) write(out *strings.Builder, h ast.ASTHandle, depth int) (ast.ASTHandle, error) {
	w.visits++
	m := w.language.manager
	if w.visits > 180_000 || depth > 256 || out.Len() > maxEnumerationBytes ||
		w.visits%128 == 0 && m.Limit.CancelFlag() {
		return ast.ASTHandle{}, inapplicable
	}
	hd, err := m.NodeHeader(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch hd.Kind {
	case ast.ASTVar:
		index, err := m.GetVarIndex(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if uint64(index) >= uint64(len(w.scope)) {
			return ast.ASTHandle{}, inapplicable
		}
		position := len(w.scope) - int(index) - 1
		s, err := m.GetSort(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if s != w.scope[position].sort {
			return ast.ASTHandle{}, inapplicable
		}
		if value := w.scope[position].replacement; value != "" {
			out.WriteString(value)
		} else {
			out.WriteByte('x')
			out.WriteString(strconv.Itoa(position))
		}
		return s, nil
	case ast.ASTQuantifier:
		return w.quantifier(out, h, depth)
	case ast.ASTApp:
		return w.application(out, h, hd, depth)
	default:
		return ast.ASTHandle{}, inapplicable
	}
}

func (w *enumerationWriter) quantifier(out *strings.Builder, h ast.ASTHandle, depth int) (ast.ASTHandle, error) {
	m := w.language.manager
	patterns, err := m.GetNumPatterns(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	excluded, err := m.GetNumNoPatterns(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n, err := m.GetNumDeclarations(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if patterns != 0 || excluded != 0 || n == 0 || n > 128 {
		return ast.ASTHandle{}, inapplicable
	}
	universal, err := m.IsForall(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	body, err := m.GetQuantifierBody(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	cut := w.dependencyBindings(body, n, universal)
	if w.dependencies != nil {
		if w.dependencies.err != nil {
			return ast.ASTHandle{}, w.dependencies.err
		}
		if w.dependencies.stopped {
			return ast.ASTHandle{}, inapplicable
		}
	}
	if len(cut) != 0 {
		w.changed = true
	}
	// Every substitution depends on a binder retained in this quantifier.
	if universal {
		out.WriteString("(forall (")
	} else {
		out.WriteString("(exists (")
	}
	base := len(w.scope)
	defer func() { w.scope = w.scope[:base] }()
	for i := range n {
		sort, err := m.GetDeclarationSort(h, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		s, err := w.language.sort(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pos := len(w.scope)
		if cut[pos] == "" {
			out.WriteString("(x")
			out.WriteString(strconv.Itoa(pos))
			out.WriteByte(' ')
			out.WriteString(s.name)
			out.WriteByte(')')
		}
		w.scope = append(w.scope, enumerationBinder{sort: sort, replacement: cut[pos]})
	}
	w.binders += int(n)
	out.WriteString(") ")
	s, err := w.write(out, body, depth+1)
	if err == nil && s != w.language.boolean {
		err = inapplicable
	}
	out.WriteByte(')')
	return s, err
}

func (w *enumerationWriter) application(out *strings.Builder, h ast.ASTHandle, hd ast.ASTNodeHeader, depth int) (ast.ASTHandle, error) {
	l := w.language
	if hd.Family == l.datatype {
		if hd.DeclKind != ast.ASTDatatypeConstructor || hd.NumArgs != 0 {
			return ast.ASTHandle{}, inapplicable
		}
		result, err := l.manager.GetSort(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		s, err := l.sort(result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		index, ok, err := l.manager.DatatypeConstructorIndex(hd.Decl)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !ok || uint64(index) >= uint64(len(s.values)) {
			return ast.ASTHandle{}, inapplicable
		}
		out.WriteString(s.values[index])
		return result, nil
	}
	if hd.Family == ast.NullFamilyID {
		f, err := l.function(hd.Decl, hd.NumArgs)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if uint32(len(f.domain)) != hd.NumArgs {
			return ast.ASTHandle{}, inapplicable
		}
		if hd.NumArgs == 0 {
			out.WriteString(f.name)
			return f.result, nil
		}
		out.WriteByte('(')
		out.WriteString(f.name)
		for i, expected := range f.domain {
			got, err := w.argument(out, h, uint32(i), depth)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if got != expected {
				return ast.ASTHandle{}, inapplicable
			}
		}
		out.WriteByte(')')
		return f.result, nil
	}
	if hd.Family != l.manager.BasicFamilyID() {
		return ast.ASTHandle{}, inapplicable
	}
	return w.basic(out, h, hd, depth)
}

func (w *enumerationWriter) argument(out *strings.Builder, h ast.ASTHandle, index uint32, depth int) (ast.ASTHandle, error) {
	a, err := w.language.manager.GetArg(h, index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	out.WriteByte(' ')
	return w.write(out, a, depth+1)
}

func (w *enumerationWriter) basic(out *strings.Builder, h ast.ASTHandle, hd ast.ASTNodeHeader, depth int) (ast.ASTHandle, error) {
	n := hd.NumArgs
	op := ""
	switch hd.DeclKind {
	case ast.ASTOpTrue, ast.ASTOpFalse:
		if n == 0 {
			if hd.DeclKind == ast.ASTOpTrue {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
			return w.language.boolean, nil
		}
	case ast.ASTOpAnd:
		if n == 0 {
			out.WriteString("true")
			return w.language.boolean, nil
		}
		op = "and"
	case ast.ASTOpOr:
		if n == 0 {
			out.WriteString("false")
			return w.language.boolean, nil
		}
		op = "or"
	case ast.ASTOpImplies:
		if n == 2 {
			op = "=>"
		}
	case ast.ASTOpNot:
		if n == 1 {
			op = "not"
		}
	case ast.ASTOpXor:
		if n == 2 {
			op = "xor"
		}
	case ast.ASTOpEq, ast.ASTOpIff:
		if n == 2 {
			op = "="
		}
	case ast.ASTOpDistinct:
		if n >= 2 {
			op = "distinct"
		}
	case ast.ASTOpIte:
		if n == 3 {
			op = "ite"
		}
	}
	if op == "" {
		return ast.ASTHandle{}, inapplicable
	}
	out.WriteByte('(')
	out.WriteString(op)
	boolean := w.language.boolean
	result, expected := boolean, boolean
	for i := range n {
		s, err := w.argument(out, h, i, depth)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		switch hd.DeclKind {
		case ast.ASTOpEq, ast.ASTOpDistinct:
			if i == 0 {
				expected = s
			}
		case ast.ASTOpIte:
			if i == 1 {
				expected, result = s, s
			}
		}
		if s != expected {
			return ast.ASTHandle{}, inapplicable
		}
	}
	out.WriteByte(')')
	return result, nil
}

func (w *enumerationWriter) assertions(base []ast.ASTHandle) (string, error) {
	var forms strings.Builder
	for _, h := range base {
		forms.WriteString("(assert ")
		s, err := w.write(&forms, h, 0)
		if err != nil {
			return "", err
		}
		if s != w.language.boolean {
			return "", inapplicable
		}
		forms.WriteString(")\n")
	}
	return forms.String(), nil
}
