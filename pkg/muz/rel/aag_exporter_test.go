package rel_test

import (
	"bytes"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	. "github.com/glycerine/g3/pkg/muz/rel"
	"strings"
	"testing"
)

type exporterFixture struct {
	manager *ast.ASTManager
	rules   *base.RuleManager
	set     *base.RuleSet
	decl    map[string]ast.ASTHandle
}

func newExporterFixture(t *testing.T, names ...string) *exporterFixture {
	m := relationASTManager(t)
	rm, e := base.NewRuleManager(m)
	must(t, e)
	t.Cleanup(func() { must(t, rm.Close()) })
	set, e := base.NewRuleSet(rm)
	must(t, e)
	t.Cleanup(func() { must(t, set.Close()) })
	b, e := m.MkBoolSort()
	must(t, e)
	f := &exporterFixture{m, rm, set, map[string]ast.ASTHandle{}}
	for _, name := range names {
		s, e := m.Symbols.Intern("aag-" + name)
		must(t, e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		must(t, e)
		must(t, rm.RegisterPredicate(d))
		f.decl[name] = d
	}
	return f
}
func (f *exporterFixture) add(t *testing.T, head string, tails ...string) {
	b, e := f.manager.MkBoolSort()
	must(t, e)
	v, e := f.manager.MkVar(0, b)
	must(t, e)
	h, e := f.manager.MkApp(f.decl[head], []ast.ASTHandle{v})
	must(t, e)
	body := make([]ast.ASTHandle, len(tails))
	for i, n := range tails {
		body[i], e = f.manager.MkApp(f.decl[n], []ast.ASTHandle{v})
		must(t, e)
	}
	r, e := f.rules.NewRule(h, body, nil, head)
	must(t, e)
	must(t, f.set.Add(r))
	must(t, r.Close())
}
func TestAAGExporterLowersLinearRuleFactsAndOutput(t *testing.T) {
	f := newExporterFixture(t, "P", "Q")
	f.add(t, "P", "Q")
	must(t, f.set.SetOutputPredicate(f.decl["P"]))
	truth, e := f.manager.MkTrue()
	must(t, e)
	x, e := NewAAGExporter(f.manager, f.set, []AAGFact{{Declaration: f.decl["Q"], Values: []ast.ASTHandle{truth}}})
	must(t, e)
	defer x.Close()
	var out bytes.Buffer
	must(t, x.Write(&out))
	text := out.String()
	if !strings.HasPrefix(text, "aag ") || !strings.Contains(text, " 1 ") {
		t.Fatal(text)
	}
}
