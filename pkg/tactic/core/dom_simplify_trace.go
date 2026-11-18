package core

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
	"io"
)

func (t *DomSimplifyTactic) tags() *diagnostic.TraceTags {
	if t.traceTags != nil {
		return t.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}
func (t *DomSimplifyTactic) formatDom(h ast.ASTHandle) (text string, result error) {
	f, e := t.manager.SMT2PPExprTree(h, nil)
	if e != nil {
		return "", e
	}
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if e != nil {
		return "", e
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if e = t.manager.PrintFormatTransient(out, f, nil, nil, nil); e != nil {
		return "", e
	}
	return out.String()
}
func (t *DomSimplifyTactic) traceRewrite(old, next ast.ASTHandle, line int) {
	tags := t.tags()
	_ = tags.EmitConditional("simplify", "simplify_goal", "src/tactic/core/dom_simplify_tactic.cpp", line, func() bool { return true }, func(w io.Writer) error {
		a, e := t.formatDom(next)
		if e != nil {
			return e
		}
		b, e := t.formatDom(old)
		if e != nil {
			return e
		}
		_, e = io.WriteString(w, a+" "+b+"\n")
		return e
	})
}
func (t *DomSimplifyTactic) traceDominatorTree() {
	tags := t.tags()
	_ = tags.EmitConditional("simplify", "compile", "src/tactic/core/dom_simplify_tactic.cpp", 139, func() bool { return true }, func(w io.Writer) error {
		var walk func(ast.ASTHandle, int) error
		walk = func(h ast.ASTHandle, n int) error {
			for i := 0; i < n; i++ {
				if _, e := io.WriteString(w, " "); e != nil {
					return e
				}
			}
			s, e := t.formatDom(h)
			if e != nil {
				return e
			}
			// expr_dominators::display: the node, its children (each on
			// the parent's line), and only then the newline.
			if _, e = io.WriteString(w, s); e != nil {
				return e
			}
			for _, child := range t.dominators.tree[h] {
				if child != h {
					if e = walk(child, n+1); e != nil {
						return e
					}
				}
			}
			_, e = io.WriteString(w, "\n")
			return e
		}
		return walk(t.dominators.root, 0)
	})
}
