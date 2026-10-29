package rel_test

import (
	"bytes"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	. "github.com/glycerine/g3/pkg/muz/rel"
	"strings"
	"testing"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func relationASTManager(t *testing.T) *ast.ASTManager {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	must(t, err)
	t.Cleanup(func() { must(t, context.Close()) })
	manager, err := context.Manager()
	must(t, err)
	return manager
}

func TestAAGBuilderExpressionSharingAndSerialization(t *testing.T) {
	m := relationASTManager(t)
	b, e := NewAAGBuilder(m)
	must(t, e)
	defer b.Close()
	sort, e := m.MkBoolSort()
	must(t, e)
	x, e := m.MkVar(0, sort)
	must(t, e)
	y, e := m.MkVar(1, sort)
	must(t, e)
	and, e := m.MkAnd(x, y)
	must(t, e)
	id, e := b.Expression(and)
	must(t, e)
	again, e := b.Expression(and)
	must(t, e)
	if id != again {
		t.Fatal(id, again)
	}
	must(t, b.AddOutput(id))
	var out bytes.Buffer
	must(t, b.Write(&out))
	if !strings.HasPrefix(out.String(), "aag 3 2 0 1 1\n") || !strings.Contains(out.String(), "6 2 4\n") {
		t.Fatal(out.String())
	}
}
