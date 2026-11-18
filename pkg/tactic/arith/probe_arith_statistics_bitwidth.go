package arith

// Source: src/tactic/arith/probe_arith.cpp:87-125; src/util/mpq.h:792-793.
import (
	"errors"

	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticStatisticProbe) evalBitWidth(g *tactic.Goal) (_ float64, resultErr error) {
	apps, e := statisticApplications(g)
	if e != nil {
		return 0, e
	}
	m := g.Manager()
	// rational r;
	q := m.NumeralService().Manager
	r := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&r)) }()
	var max, count uint32
	var total uint64
	for _, h := range apps {
		_, yes, e := m.ArithmeticNumeralRational(h, &r)
		if e != nil {
			return 0, e
		}
		if !yes {
			continue
		}
		// mpq bitsize: numerator bits, plus denominator bits when not integral.
		bits, e := q.Bitsize(&r)
		if e != nil {
			return 0, e
		}
		if bits > max {
			max = bits
		}
		total += uint64(bits)
		count++
	}
	return statisticResult(p.kind == statAverageBitWidth, max, total, count), nil
}
