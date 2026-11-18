package polynomial

// Expression fixture shared by the polynomial tests (kept from the deleted
// Discriminant tests).

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"strconv"
	"strings"
	"testing"
)

func polynomialDiscriminantFixture(t *testing.T) *MultivariatePolynomialManager {
	t.Helper()
	// This source corpus constructs large multivariate resultants; keep its
	// bounded fixture storage separate from the small rational API helper.
	q, _ := rationalManagerWithLimit(t, false, 2<<20)
	m, e := NewMultivariatePolynomialManager(MultivariatePolynomialManagerConfig{Core: MultivariatePolynomialConfig{Integer: q.Integers(), Rationals: q, Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }}})
	serviceMust(t, e)
	t.Cleanup(func() {
		serviceMust(t, m.CloseChecked())
	})
	return m
}

// This fixture-only parser evaluates literal source expressions through the
// existing Add/Sub/Mul/Power APIs. Expected expressions never call Discriminant.
type polynomialDiscriminantExpression struct {
	t         *testing.T
	c         *MultivariatePolynomialCore
	variables [5]Polynomial
	text      string
	position  int
}

func (p *polynomialDiscriminantExpression) own(v Polynomial, e error) Polynomial {
	p.t.Helper()
	serviceMust(p.t, e)
	return polynomialCaller(p.t, p.c, v)
}
func (p *polynomialDiscriminantExpression) take(ch byte) bool {
	if p.position < len(p.text) && p.text[p.position] == ch {
		p.position++
		return true
	}
	return false
}
func (p *polynomialDiscriminantExpression) number() uint32 {
	start := p.position
	for p.position < len(p.text) && p.text[p.position] >= '0' && p.text[p.position] <= '9' {
		p.position++
	}
	if start == p.position {
		p.t.Fatalf("source fixture missing number at %q", p.text[start:])
	}
	n, e := strconv.ParseUint(p.text[start:p.position], 10, 32)
	serviceMust(p.t, e)
	return uint32(n)
}
func (p *polynomialDiscriminantExpression) primary() Polynomial {
	var v Polynomial
	if p.take('(') {
		v = p.sum()
		if !p.take(')') {
			p.t.Fatal("source fixture closing parenthesis", p.text)
		}
	} else if p.position < len(p.text) && p.text[p.position] >= '0' && p.text[p.position] <= '9' {
		i := unumeral.NewInt(int32(p.number()))
		v = p.own(p.c.MkConstantTakeCopy(&i))
	} else {
		if p.position == len(p.text) {
			p.t.Fatal("source fixture missing operand")
		}
		ch := p.text[p.position]
		p.position++
		switch ch {
		case 'a', 'b', 'c', 'd':
			v = p.variables[ch-'a']
		case 'x':
			v = p.variables[4]
		case 'o':
			if !strings.HasPrefix(p.text[p.position:], "ne") {
				p.t.Fatal("source fixture unknown name")
			}
			p.position += 2
			v = p.c.unit
		case 'z':
			if !strings.HasPrefix(p.text[p.position:], "ero") {
				p.t.Fatal("source fixture unknown name")
			}
			p.position += 3
			v = p.c.zero
		default:
			p.t.Fatalf("source fixture unknown operand %c", ch)
		}
	}
	if p.take('^') {
		n := p.number()
		r, e := NewPolynomialRef(p.c)
		serviceMust(p.t, e)
		p.t.Cleanup(func() { serviceMust(p.t, r.Close()) })
		serviceMust(p.t, p.c.Power(v, n, r))
		v = r.root.value
	}
	return v
}
func (p *polynomialDiscriminantExpression) unary() Polynomial {
	if p.take('-') {
		return p.own(p.c.Neg(p.unary()))
	}
	if p.take('+') {
		return p.unary()
	}
	return p.primary()
}
func (p *polynomialDiscriminantExpression) product() Polynomial {
	v := p.unary()
	for p.take('*') {
		v = p.own(p.c.Mul(v, p.unary()))
	}
	return v
}
func (p *polynomialDiscriminantExpression) sum() Polynomial {
	v := p.product()
	for {
		if p.take('+') {
			v = p.own(p.c.Add(v, p.product()))
		} else if p.take('-') {
			v = p.own(p.c.Sub(v, p.product()))
		} else {
			return v
		}
	}
}
func (p *polynomialDiscriminantExpression) evaluate(s string) Polynomial {
	p.text = strings.Join(strings.Fields(s), "")
	p.position = 0
	v := p.sum()
	if p.position != len(p.text) {
		p.t.Fatal("source fixture trailing expression", p.text[p.position:])
	}
	return v
}

// A bounded integer determinant is independent of the production resultant,
// derivative and discriminant implementations; dimension is at most seven.
func polynomialDiscriminantDeterminant(a [7][7]int64, n int) int64 {
	var visit func(int, uint32, int64, bool) int64
	visit = func(row int, used uint32, product int64, negative bool) int64 {
		if row == n {
			if negative {
				return -product
			}
			return product
		}
		var sum int64
		for col := 0; col < n; col++ {
			if used&(1<<col) != 0 || a[row][col] == 0 {
				continue
			}
			odd := false
			for prior := col + 1; prior < n; prior++ {
				if used&(1<<prior) != 0 {
					odd = !odd
				}
			}
			sum += visit(row+1, used|(1<<col), product*a[row][col], negative != odd)
		}
		return sum
	}
	return visit(0, 0, 1, false)
}
