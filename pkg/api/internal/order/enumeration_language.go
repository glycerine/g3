package order

import (
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

// The private language gives each source sort and function its own declaration.
// An enumeration becomes an uninterpreted sort with distinct named values and
// an axiom saying that each element of the sort is one of those values.
type enumerationLanguage struct {
	manager      *ast.ASTManager
	boolean      ast.ASTHandle
	datatype     ast.FamilyID
	base         *enumerationLanguage
	sorts        map[ast.ASTHandle]enumerationSort
	functions    map[ast.ASTHandle]enumerationFunction
	declarations strings.Builder
	axioms       []string // unasserted Boolean terms
	axiomBytes   int
	declCount    int
	enumerations int
}

type enumerationSort struct {
	name   string
	values []string
}

type enumerationFunction struct {
	name   string
	domain []ast.ASTHandle
	result ast.ASTHandle
}

func enumerationName(prefix string, h ast.ASTHandle) string {
	return prefix + strconv.FormatUint(uint64(h.Slot()), 10) + "_" + strconv.FormatUint(uint64(h.Generation()), 10)
}

func (l *enumerationLanguage) declare(line string) error {
	if l.declarations.Len()+len(line) > maxEnumerationBytes || l.declCount >= 8192 {
		return inapplicable
	}
	l.declarations.WriteString(line)
	l.declCount++
	return nil
}

func (l *enumerationLanguage) sort(h ast.ASTHandle) (enumerationSort, error) {
	if h == l.boolean {
		return enumerationSort{name: "Bool"}, nil
	}
	if s, ok := l.sorts[h]; ok {
		return s, nil
	}
	if l.base != nil {
		if s, ok := l.base.sorts[h]; ok {
			return s, nil
		}
	}
	var s enumerationSort
	uninterpreted, err := l.manager.IsUninterpretedSort(h)
	if err != nil {
		return s, err
	}
	if uninterpreted {
		s.name = enumerationName("es", h)
	} else {
		enum, err := l.manager.DatatypeIsEnumSort(h)
		if err != nil {
			return s, err
		}
		if !enum {
			return s, inapplicable
		}
		n, err := l.manager.DatatypeNumConstructors(h)
		if err != nil {
			return s, err
		}
		if n < 2 || n > 16 {
			return s, inapplicable
		}
		s.name = enumerationName("ee", h)
		s.values = make([]string, n)
	}
	if err := l.declare("(declare-sort " + s.name + " 0)\n"); err != nil {
		return s, err
	}
	if len(s.values) != 0 {
		equalities := make([]string, len(s.values))
		for i := range s.values {
			value := s.name + "c" + strconv.Itoa(i)
			if err := l.declare("(declare-fun " + value + " () " + s.name + ")\n"); err != nil {
				return s, err
			}
			s.values[i] = value
			equalities[i] = "(= ea " + value + ")"
		}
		distinct := "(distinct " + strings.Join(s.values, " ") + ")"
		cover := "(forall ((ea " + s.name + ")) (or " + strings.Join(equalities, " ") + "))"
		l.axiomBytes += len(distinct) + len(cover)
		if l.axiomBytes > maxEnumerationBytes {
			return s, inapplicable
		}
		l.axioms = append(l.axioms, distinct, cover)
		l.enumerations++
	}
	if l.sorts == nil {
		l.sorts = make(map[ast.ASTHandle]enumerationSort)
	}
	l.sorts[h] = s
	return s, nil
}

func (l *enumerationLanguage) function(h ast.ASTHandle, arity uint32) (enumerationFunction, error) {
	if f, ok := l.functions[h]; ok {
		return f, nil
	}
	if l.base != nil {
		if f, ok := l.base.functions[h]; ok {
			return f, nil
		}
	}
	var f enumerationFunction
	n, err := l.manager.GetArity(h)
	if err != nil {
		return f, err
	}
	if n != arity || n > 128 {
		return f, inapplicable
	}
	f.domain = make([]ast.ASTHandle, n)
	inputs := make([]string, n)
	for i := range n {
		d, err := l.manager.GetDomain(h, i)
		if err != nil {
			return f, err
		}
		s, err := l.sort(d)
		if err != nil {
			return f, err
		}
		f.domain[i], inputs[i] = d, s.name
	}
	f.result, err = l.manager.GetRange(h)
	if err != nil {
		return f, err
	}
	result, err := l.sort(f.result)
	if err != nil {
		return f, err
	}
	f.name = enumerationName("ef", h)
	if err := l.declare("(declare-fun " + f.name + " (" + strings.Join(inputs, " ") + ") " + result.name + ")\n"); err != nil {
		return f, err
	}
	if l.functions == nil {
		l.functions = make(map[ast.ASTHandle]enumerationFunction)
	}
	l.functions[h] = f
	return f, nil
}
