package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderAUFComparisonGroundBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, domain, bound, want     string
		negative, ge, offset, inverse bool
	}{
		{name: "positive-le", domain: "integer", bound: "10", want: "11"},
		{name: "negative-le", domain: "integer", bound: "10", want: "10", negative: true},
		{name: "positive-ge", domain: "integer", bound: "10", want: "9", ge: true},
		{name: "offset-le", domain: "integer", bound: "10", want: "9", offset: true},
		{name: "inverted-offset-le", domain: "integer", bound: "10", want: "-9", offset: true, inverse: true},
		{name: "negative-inverted-offset-le", domain: "integer", bound: "10", want: "-8", negative: true, offset: true, inverse: true},
		{name: "real-le", domain: "real", bound: "3/2", want: "5/2"},
		{name: "bit-vector-wrap", domain: "bit-vector", bound: "15", want: "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, _, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
			makeNumeral := func(value string) ast.ASTHandle {
				var h ast.ASTHandle
				var err error
				switch test.domain {
				case "integer":
					h, err = m.ArithmeticIntegerNumeralText(value)
				case "real":
					h, err = m.ArithmeticRealNumeralText(value)
				case "bit-vector":
					_, err = m.RegisterBitVectorPlugin()
					astTestMust(t, err)
					v, e := strconv.ParseUint(value, 10, 32)
					astTestMust(t, e)
					h, err = m.BitVectorNumeralUnsigned(uint32(v), 4)
				}
				astTestMust(t, err)
				return h
			}
			bound, want := makeNumeral(test.bound), makeNumeral(test.want)
			sort, err := m.GetSort(bound)
			astTestMust(t, err)
			x, err := m.MkVar(0, sort)
			astTestMust(t, err)
			left := x
			if test.inverse {
				left, err = m.MkArithmeticMul(makeNumeral("-1"), x)
				astTestMust(t, err)
			}
			if test.offset {
				left, err = m.MkArithmeticAdd(left, makeNumeral("2"))
				astTestMust(t, err)
			}
			var body ast.ASTHandle
			if test.domain == "bit-vector" {
				body, err = m.MkBitVectorULE(left, bound)
			} else if test.ge {
				body, err = m.MkArithmeticGE(left, bound)
			} else {
				body, err = m.MkArithmeticLE(left, bound)
			}
			astTestMust(t, err)
			if test.negative {
				body, err = m.MkNot(body)
				astTestMust(t, err)
			}
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "auf-bound-x")}, Body: body,
			})
			astTestMust(t, err)
			graph := newSMTModelFinderAUFGraph(manager.modelFinder)
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
			node := graph.uvars[smtModelFinderAUFKey{q, 0}].root()
			values := aufValueSet(t, manager, node.set.testElements())
			generation, found := values[want]
			if !node.mono || node.signed || len(values) != 1 || !found || generation != 0 {
				t.Fatal("source comparison boundary and projection flags", node.mono, node.signed, node.set.testElements(), want)
			}
		})
	}
}

func TestSMTModelFinderAUFVariableComparisonPolarityAndSignedProjection(t *testing.T) {
	for _, test := range []struct {
		name                                   string
		negative, signed, difference, equality bool
	}{
		{name: "positive-integer"},
		{name: "negative-integer", negative: true},
		{name: "negative-difference", negative: true, difference: true},
		{name: "negative-difference-equality", negative: true, difference: true, equality: true},
		{name: "negative-signed-bit-vector", negative: true, signed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, _, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			if test.signed {
				_, err = m.RegisterBitVectorPlugin()
				astTestMust(t, err)
				zero, err = m.BitVectorNumeralUnsigned(0, 4)
				astTestMust(t, err)
			}
			sort, err := m.GetSort(zero)
			astTestMust(t, err)
			x, err := m.MkVar(0, sort)
			astTestMust(t, err)
			y, err := m.MkVar(1, sort)
			astTestMust(t, err)
			left, right := x, y
			if test.difference {
				minusOne, err := m.ArithmeticIntegerNumeralText("-1")
				astTestMust(t, err)
				negatedY, err := m.MkArithmeticMul(minusOne, y)
				astTestMust(t, err)
				left, err = m.MkArithmeticAdd(x, negatedY)
				astTestMust(t, err)
				right = zero
			}
			var body ast.ASTHandle
			if test.signed {
				body, err = m.MkBitVectorSLE(left, right)
			} else if test.equality {
				body, err = m.MkEq(left, right)
			} else {
				body, err = m.MkArithmeticLE(left, right)
			}
			astTestMust(t, err)
			if test.negative {
				body, err = m.MkNot(body)
				astTestMust(t, err)
			}
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{sort, sort},
				Names: []symbol.Symbol{astTestSymbol(t, m, "auf-pair-y"), astTestSymbol(t, m, "auf-pair-x")}, Body: body,
			})
			astTestMust(t, err)
			isAUF, err := manager.modelFinder.classifyAUF(q)
			astTestMust(t, err)
			if isAUF != (test.negative || test.equality) {
				t.Fatal("AUF classifier ignored comparison polarity", isAUF)
			}
			graph := newSMTModelFinderAUFGraph(manager.modelFinder)
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
			first := graph.uvars[smtModelFinderAUFKey{q, 0}].root()
			second := graph.uvars[smtModelFinderAUFKey{q, 1}].root()
			if (first == second) != test.negative || first.mono != (test.negative && !test.equality) || first.signed != test.signed {
				t.Fatal("source comparison class merge or projection flags", first, second)
			}
		})
	}
}
