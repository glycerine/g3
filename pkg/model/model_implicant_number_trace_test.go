package model

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestModelImplicantNumberAssignmentSourceTrace(t *testing.T) {
	for _, value := range []string{"-7", "100000000000000000000000000000000000001", "3/2", "-7/3"} {
		for _, mode := range []string{"enabled", "disabled", "other-tag", "refused"} {
			t.Run(fmt.Sprintf("%s/%s", value, mode), func(t *testing.T) {
				m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
				var pins []ast.ASTRef
				defer func() {
					for n := range pins {
						arrayTableMust(t, m.DelRef(&pins[n]))
					}
				}()
				own := func(h ast.ASTHandle, err error) ast.ASTHandle {
					t.Helper()
					arrayTableMust(t, err)
					ref, err := m.NewRef(h)
					arrayTableMust(t, err)
					pins = append(pins, ref)
					return h
				}
				var numeral ast.ASTHandle
				if strings.Contains(value, "/") {
					numeral = own(m.ArithmeticRealNumeralText(value))
				} else {
					numeral = own(m.ArithmeticIntegerNumeralText(value))
				}
				sort, err := m.GetSort(numeral)
				arrayTableMust(t, err)
				symbol, err := m.Symbols.Intern("numeric-trace-x")
				arrayTableMust(t, err)
				x := own(m.MkConst(symbol, sort))
				decl, err := m.GetDecl(x)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, numeral))
				yes := own(m.MkTrue())
				inherited := own(m.MkIte(yes, x, x))
				formula := own(m.MkEq(inherited, numeral))
				switch mode {
				case "enabled":
					tags.Enable("pdr_verbose")
				case "other-tag":
					tags.Enable("pdr")
				case "refused":
					tags = diagnostic.NewTraceTagsWithWriter(modelImplicantRefusingTraceWriter{})
					tags.Enable("pdr_verbose")
					implicant.services.Trace = tags
				}
				header := "-------- [pdr_verbose] set_number src/model/model_implicant.h:77 ---------\n"
				footer := "------------------------------------------------\n"
				before := m.Stats()
				arrayTableMust(t, implicant.setupModel(md))
				if mode == "enabled" && tags.ShouldTrace("pdr_verbose") {
					expected := header + "numeric-trace-x " + value + "\n" + footer
					if output.String() != expected {
						t.Fatalf("wrong setup number trace\ngot %q\nwant %q", output.String(), expected)
					}
				} else if output.Len() != 0 {
					t.Fatal("disabled number trace wrote output", output.String())
				}
				output.Reset()
				satisfied, err := implicant.checkModel([]ast.ASTHandle{formula})
				arrayTableMust(t, err)
				if !satisfied {
					t.Fatal("number tracing changed formula truth")
				}
				expected, _ := new(big.Rat).SetString(value)
				for _, term := range []ast.ASTHandle{x, numeral, inherited} {
					if actual := implicant.numbers[term]; actual == nil || actual.Cmp(expected) != 0 {
						t.Fatal("numeric assignment changed value", actual, expected)
					}
				}
				if mode == "enabled" && tags.ShouldTrace("pdr_verbose") {
					records := strings.Split(output.String(), header)
					if len(records) != 3 {
						t.Fatal("wrong literal/inherited number trace count", output.String())
					}
					for _, record := range records[1:] {
						body, _, found := strings.Cut(record, footer)
						if !found || !strings.HasSuffix(body, " "+value+"\n") {
							t.Fatal("wrong exact rational trace value", body, value)
						}
					}
					if !strings.HasPrefix(records[2], "(ite true numeric-trace-x numeric-trace-x) "+value+"\n") {
						t.Fatal("missing inherited numeric expression", records[2])
					}
				} else if output.Len() != 0 {
					t.Fatal("disabled number trace wrote evaluation output", output.String())
				}
				arrayTableMust(t, implicant.reset())
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("numeric trace leaked ownership", after, before)
				}
			})
		}
	}
}
