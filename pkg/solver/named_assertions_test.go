package solver

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type namedCoreFixture struct {
	asserted     []ast.ASTHandle
	checked      []ast.ASTHandle
	pushes, pops uint32
	display      string
}

func (c *namedCoreFixture) AssertFormula(h ast.ASTHandle) error {
	c.asserted = append(c.asserted, h)
	return nil
}
func (c *namedCoreFixture) CheckSatCore(values []ast.ASTHandle) (lbool.LBool, error) {
	c.checked = append(c.checked[:0], values...)
	return lbool.True, nil
}
func (c *namedCoreFixture) PushCore() error        { c.pushes++; return nil }
func (c *namedCoreFixture) PopCore(n uint32) error { c.pops += n; return nil }
func (c *namedCoreFixture) DisplayCore(w io.Writer) error {
	_, err := io.WriteString(w, c.display)
	return err
}

func TestNamedAssertionsTrackedChecksScopesAndTemporaryAssumptions(t *testing.T) {
	m := solverTestManager(t)
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("solver_na2as")
	core := &namedCoreFixture{display: "solver-state\n"}
	n, err := NewNamedAssertionsWithTrace(m, core, tags)
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, n.Close()) })
	boolSort, err := m.MkBoolSort()
	solverMust(t, err)
	formulaName, err := m.Symbols.Intern("na-formula")
	solverMust(t, err)
	formula, err := m.MkConst(formulaName, boolSort)
	solverMust(t, err)
	tagName, err := m.Symbols.Intern("na-tag")
	solverMust(t, err)
	tag, err := m.MkConst(tagName, boolSort)
	solverMust(t, err)
	extraName, err := m.Symbols.Intern("na-extra")
	solverMust(t, err)
	extra, err := m.MkConst(extraName, boolSort)
	solverMust(t, err)
	for _, value := range []ast.ASTHandle{formula, tag, extra} {
		ref, e := m.NewRef(value)
		solverMust(t, e)
		t.Cleanup(func() { solverMust(t, m.DelRef(&ref)) })
	}

	solverMust(t, n.AssertTracked(formula, ast.ASTHandle{}))
	solverMust(t, n.AssertTracked(formula, tag))
	if len(core.asserted) != 2 || core.asserted[0] != formula {
		t.Fatal("tracked assertion forwarding", core.asserted)
	}
	if count, err := n.NumAssumptions(); err != nil || count != 1 {
		t.Fatal("named assumption count", count, err)
	}
	if got, err := n.Assumption(0); err != nil || got != tag {
		t.Fatal("named assumption", got, err)
	}
	status, err := n.CheckSat([]ast.ASTHandle{extra})
	solverMust(t, err)
	if status != lbool.True || len(core.checked) != 2 || core.checked[0] != tag || core.checked[1] != extra {
		t.Fatal("combined assumptions", status, core.checked)
	}
	if count, _ := n.NumAssumptions(); count != 1 {
		t.Fatal("temporary assumption survived", count)
	}

	solverMust(t, n.Push())
	solverMust(t, n.AssertTracked(formula, extra))
	solverMust(t, n.Pop(1))
	if core.pushes != 1 || core.pops != 1 {
		t.Fatal("core scope forwarding", core.pushes, core.pops)
	}
	if count, _ := n.NumAssumptions(); count != 1 {
		t.Fatal("scope did not restore assumptions", count)
	}
	if tags.ShouldTrace("solver_na2as") {
		text := trace.String()
		if !bytes.Contains([]byte(text), []byte("asserting\nna-formula\nna-tag\n")) || !bytes.Contains([]byte(text), []byte("solver-state\n")) {
			t.Fatal("named assertion trace", text)
		}
	} else if trace.Len() != 0 {
		t.Fatal("ordinary build emitted trace", trace.String())
	}
}

func TestNamedAssertionsWithAssumptionsAndInvalidTag(t *testing.T) {
	m := solverTestManager(t)
	core := &namedCoreFixture{}
	n, err := NewNamedAssertions(m, core)
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, n.Close()) })
	truth, err := m.MkTrue()
	solverMust(t, err)
	if err = n.AssertTracked(truth, truth); err == nil {
		t.Fatal("interpreted tag accepted")
	}
	status, consequences, err := n.WithAssumptions([]ast.ASTHandle{truth}, func(values []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
		if len(values) != 1 || values[0] != truth {
			t.Fatal("temporary operation assumptions", values)
		}
		return lbool.False, []ast.ASTHandle{truth}, nil
	})
	solverMust(t, err)
	if status != lbool.False || len(consequences) != 1 {
		t.Fatal("temporary operation result", status, consequences)
	}
	if count, _ := n.NumAssumptions(); count != 0 {
		t.Fatal("operation assumption survived", count)
	}
}

func TestNamedAssertionsTemporaryRetirementFollowsSourceForwardRange(t *testing.T) {
	for _, mode := range []string{"check", "consequences", "close"} {
		t.Run(mode, func(t *testing.T) {
			base := solverTestManager(t)
			var trace bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&trace)
			m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags})
			solverMust(t, err)
			t.Cleanup(func() { solverMust(t, m.Close()) })
			n, err := NewNamedAssertions(m, &namedCoreFixture{})
			solverMust(t, err)
			t.Cleanup(func() { solverMust(t, n.Close()) })
			boolean, err := m.MkBoolSort()
			solverMust(t, err)
			var values []ast.ASTHandle
			var ids []uint32
			for _, name := range []string{"na-retire-first", "na-retire-second", "na-retire-third"} {
				symbol, e := m.Symbols.Intern(name)
				solverMust(t, e)
				h, e := m.MkConst(symbol, boolean)
				solverMust(t, e)
				id, e := m.GetASTID(h)
				solverMust(t, e)
				values = append(values, h)
				ids = append(ids, id)
			}
			tags.Enable("ast")
			trace.Reset()
			switch mode {
			case "check":
				_, err = n.CheckSat(values)
			case "consequences":
				_, _, err = n.WithAssumptions(values, func([]ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) { return lbool.True, nil, nil })
			case "close":
				_, err = n.appendTemporary(values)
				solverMust(t, err)
				err = n.Close()
			}
			solverMust(t, err)
			if len(n.assumptions) != 0 {
				t.Fatal("retired assumptions retained")
			}
			for _, h := range values {
				if m.IsLiveHandle(h) {
					t.Fatal("temporary assumption survived")
				}
			}
			if tags.ShouldTrace("ast") {
				previous := -1
				for _, id := range ids {
					pos := strings.Index(trace.String(), fmt.Sprintf("Deleting object %d ", id))
					if pos <= previous {
						t.Fatal("source forward range retirement order", ids, trace.String())
					}
					previous = pos
				}
			}
		})
	}
}
