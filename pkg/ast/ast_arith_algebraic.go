package ast

// Source: src/ast/arith_decl_plugin.cpp:20-72,99-112.

import (
	"errors"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
	"github.com/glycerine/g3/pkg/util/smallint"
)

// astAlgebraicNumbersWrapper is the concrete lazy m_aw owner. Values are
// indexed by the source id_gen; a recycled slot contains canonical zero.
type astAlgebraicNumbersWrapper struct {
	owner       *astArithmeticPluginOwner
	rationals   *unumeral.RationalManager
	manager     *polynomial.AlgebraicManager
	cooperation *cooperate.Cooperation
	primes      *primegen.Generator
	values      []polynomial.Algebraic
	// The signed numerator chooses the root. Denominators default to one,
	// so integer radicands alone allocate no denominator slots.
	squareRoots        []int32
	squareDenominators []int32
	freeIDs            []uint32
	nextID             uint32
	closed             bool
}

func (p *astArithmeticPluginOwner) algebraicWrapper() (*astAlgebraicNumbersWrapper, error) {
	if p == nil || p.self != p || p.owner == nil || p.owner.arithmeticPlugin != p || !p.published {
		return nil, ErrASTHandle
	}
	if p.algebraic != nil {
		if p.algebraic.closed {
			return nil, arena.ErrClosed
		}
		return p.algebraic, nil
	}
	m := p.owner
	g := p.owner.globals
	params := p.owner.params
	if g == nil {
		g = uparams.BorrowProcessGlobalParamsService()
		if g != nil {
			params = g.Context
		}
	}
	if g == nil {
		return nil, uparams.ErrGlobalParamsOwner
	}
	if params == nil || g.Context != params {
		return nil, uparams.ErrParamsOwner
	}
	parameters, e := g.SourceGetRef()
	if e != nil {
		return nil, e
	}
	w := &astAlgebraicNumbersWrapper{owner: p}
	w.rationals, e = m.numerals.NewPrivateRationalManager()
	if e != nil {
		return nil, e
	}
	fail := func(cause error) (*astAlgebraicNumbersWrapper, error) {
		_ = w.close()
		return nil, cause
	}
	w.cooperation, e = cooperate.NewCooperation(m.BudgetField)
	if e != nil {
		return fail(e)
	}
	w.primes, e = primegen.NewGenerator(m.BudgetField)
	if e != nil {
		return fail(e)
	}
	w.manager, e = polynomial.NewAlgebraicManager(polynomial.AlgebraicManagerConfig{
		Rationals: m.numerals, Numerals: w.rationals, Limit: &m.Limit,
		Cooperation: w.cooperation, Params: parameters, Globals: g,
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: w.primes},
	})
	if e != nil {
		return fail(e)
	}
	p.algebraic = w
	return w, nil
}

func (w *astAlgebraicNumbersWrapper) mkID(source *polynomial.AlgebraicRootCore, value *polynomial.Algebraic) (uint32, error) {
	if w == nil || w.closed || w.manager == nil || source == nil || value == nil {
		return 0, ErrASTHandle
	}
	id, err := w.newID()
	if err != nil {
		return 0, err
	}
	if err = w.manager.CopyFromCore(&w.values[id], source, value); err != nil {
		return 0, errors.Join(err, w.releaseID(id))
	}
	return id, nil
}

func (w *astAlgebraicNumbersWrapper) newID() (uint32, error) {
	var id uint32
	if n := len(w.freeIDs); n != 0 {
		id = w.freeIDs[n-1]
		w.freeIDs = w.freeIDs[:n-1]
	} else {
		id = w.nextID
		if id == math.MaxUint32 || uint64(id)+1 > uint64(^uint(0)>>1) {
			return 0, arena.ErrBounds
		}
		w.nextID++
	}
	for len(w.values) <= int(id) {
		w.values = append(w.values, polynomial.Algebraic{})
	}
	return id, nil
}

func (w *astAlgebraicNumbersWrapper) value(id uint32) (*polynomial.Algebraic, error) {
	if w == nil || w.closed || uint64(id) >= uint64(len(w.values)) {
		return nil, ErrASTHandle
	}
	v := &w.values[id]
	if n := w.signedRadicand(id); n != 0 && *v == (polynomial.Algebraic{}) {
		var exact polynomial.Algebraic
		denominator := w.squareDenominator(id)
		radicand, lower := squareRootInterval(n, denominator)
		if err := w.manager.SetIsolatedSquareRootFraction(&exact, radicand, denominator, lower); err != nil {
			return nil, errors.Join(err, w.manager.Del(&exact))
		}
		if err := w.manager.Swap(v, &exact); err != nil {
			return nil, errors.Join(err, w.manager.Del(&exact))
		}
		return v, nil
	}
	zero, e := w.manager.IsZero(v)
	if e != nil || zero {
		if e != nil {
			return nil, e
		}
		return nil, ErrASTHandle
	}
	return v, nil
}

func (w *astAlgebraicNumbersWrapper) signedRadicand(id uint32) int32 {
	if uint64(id) < uint64(len(w.squareRoots)) {
		return w.squareRoots[id]
	}
	return 0
}

func (w *astAlgebraicNumbersWrapper) squareDenominator(id uint32) int32 {
	if uint64(id) < uint64(len(w.squareDenominators)) && w.squareDenominators[id] != 0 {
		return w.squareDenominators[id]
	}
	return 1
}

func (w *astAlgebraicNumbersWrapper) recycle(id uint32) error {
	if w == nil || w.closed || uint64(id) >= uint64(len(w.values)) {
		return ErrASTHandle
	}
	if w.signedRadicand(id) == 0 {
		if _, err := w.value(id); err != nil {
			return err
		}
	}
	return w.releaseID(id)
}

// A reservation belongs to the wrapper until a declaration takes ownership.
// A failed construction may still have the canonical zero in the slot.
func (w *astAlgebraicNumbersWrapper) releaseID(id uint32) error {
	if err := w.manager.Del(&w.values[id]); err != nil {
		return err
	}
	if uint64(id) < uint64(len(w.squareRoots)) {
		w.squareRoots[id] = 0
	}
	if uint64(id) < uint64(len(w.squareDenominators)) {
		w.squareDenominators[id] = 0
	}
	w.freeIDs = append(w.freeIDs, id)
	return nil
}

func (w *astAlgebraicNumbersWrapper) close() error {
	if w == nil || w.closed {
		return nil
	}
	for i := range w.values {
		if w.manager != nil {
			if e := w.manager.Del(&w.values[i]); e != nil {
				return e
			}
		}
	}
	w.values = nil
	w.squareRoots = nil
	w.squareDenominators = nil
	w.freeIDs = nil
	if w.manager != nil {
		if e := w.manager.Close(); e != nil {
			return e
		}
		w.manager = nil
	}
	if w.primes != nil {
		if e := w.primes.Close(); e != nil {
			return e
		}
		w.primes = nil
	}
	if w.cooperation != nil {
		if e := w.cooperation.Close(); e != nil {
			return e
		}
		w.cooperation = nil
	}
	if w.rationals != nil {
		if e := w.rationals.CloseChecked(); e != nil {
			return e
		}
		w.rationals = nil
	}
	w.closed = true
	return nil
}

// ArithmeticAlgebraicManager returns the arithmetic plugin's lazy algebraic
// owner. The returned manager is borrowed and remains valid until AST Close.
func (m *ASTManager) ArithmeticAlgebraicManager() (*polynomial.AlgebraicManager, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	if m.arithmeticPlugin == nil {
		return nil, ErrASTHandle
	}
	w, e := m.arithmeticPlugin.algebraicWrapper()
	if e != nil {
		return nil, e
	}
	return w.manager, nil
}

// ArithmeticBinomialPower returns the exact rational kth power of an
// irrational numeral when it is immediate from the defining polynomial.
func (m *ASTManager) ArithmeticBinomialPower(h ASTHandle, k uint32) (numerator, denominator int64, ok bool, err error) {
	if err := m.lock(); err != nil {
		return 0, 0, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil || m.arithmeticPlugin == nil || !m.unboundedAlgebraicReads() {
		return 0, 0, false, err
	}
	id, ok, err := m.arithmeticAlgebraicID(m.arithmeticPlugin.family, r)
	if err != nil || !ok {
		return 0, 0, false, err
	}
	w := m.arithmeticPlugin.algebraic
	if n := w.signedRadicand(id); n != 0 {
		if k != 2 {
			return 0, 0, false, nil
		}
		if n < 0 {
			n = -n
		}
		return int64(n), int64(w.squareDenominator(id)), true, nil
	}
	v, err := w.value(id)
	if err != nil {
		return 0, 0, false, err
	}
	return w.manager.BinomialPowerFraction(v, k)
}

// ArithmeticAlgebraicNumeralValue recognizes an irrational arithmetic
// numeral and copies its value into out. The caller owns the copy and must
// release it with the borrowed ArithmeticAlgebraicManager.
func (m *ASTManager) ArithmeticAlgebraicNumeralValue(h ASTHandle, out *polynomial.Algebraic) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if out == nil || m.arithmeticPlugin == nil {
		return false, ErrASTHandle
	}
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	value, matched, e := m.arithmeticAlgebraicValue(m.arithmeticPlugin.family, r)
	if e != nil || !matched {
		return false, e
	}
	if e = m.arithmeticPlugin.algebraic.manager.Copy(out, value); e != nil {
		return false, e
	}
	return true, nil
}

// ArithmeticAlgebraicNumeral implements the irrational branch of
// arith_decl_plugin::mk_numeral(anum,false).
func (m *ASTManager) ArithmeticAlgebraicNumeral(value *polynomial.Algebraic) (ASTHandle, error) {
	return m.ArithmeticAlgebraicNumeralFrom(nil, value)
}

// ArithmeticAlgebraicNumeralFrom is ArithmeticAlgebraicNumeral for a value
// owned by another algebraic manager, such as an nlsat solver's own m_am:
// the plugin's m_amanager.set copies it. A nil source selects the plugin's
// manager.
func (m *ASTManager) ArithmeticAlgebraicNumeralFrom(source *polynomial.AlgebraicRootCore, value *polynomial.Algebraic) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	p := m.arithmeticPlugin
	if p == nil || value == nil {
		return ASTHandle{}, ErrASTHandle
	}
	w, e := p.algebraicWrapper()
	if e != nil {
		return ASTHandle{}, e
	}
	if source == nil {
		source = w.manager.AlgebraicRootCore
	}
	rational, e := source.IsRational(value)
	if e != nil {
		return ASTHandle{}, e
	}
	if rational {
		return ASTHandle{}, ErrASTDomain
	}
	id, e := w.mkID(source, value)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.arithmeticAlgebraicApp(w, id)
}

// ArithmeticSquareRootNumeral returns the exact signed real square root of n.
// Nonsquares can be stored in the arithmetic plugin as a signed radicand.
func (m *ASTManager) ArithmeticSquareRootNumeral(n int32, negative bool) (ASTHandle, error) {
	if n < 0 {
		return ASTHandle{}, ErrASTDomain
	}
	lo := int64(floorSquareRoot(n))
	if lo*lo == int64(n) {
		if negative {
			lo = -lo
		}
		return m.ArithmeticNumeralInt64(lo, false)
	}
	return m.arithmeticIrrationalSquareRoot(n, 1, negative)
}

// ArithmeticSquareRootFraction returns the exact signed square root of
// numerator/denominator. Rational roots use ordinary real numerals.
func (m *ASTManager) ArithmeticSquareRootFraction(numerator, denominator int32, negative bool) (ASTHandle, error) {
	if numerator < 0 || denominator <= 0 {
		return ASTHandle{}, ErrASTDomain
	}
	if numerator == 0 || denominator == 1 {
		return m.ArithmeticSquareRootNumeral(numerator, negative)
	}
	gcd := smallint.GCD(smallint.New(numerator), smallint.New(denominator)).Int32()
	numerator, denominator = numerator/gcd, denominator/gcd
	if denominator == 1 {
		return m.ArithmeticSquareRootNumeral(numerator, negative)
	}
	p, q := floorSquareRoot(numerator), floorSquareRoot(denominator)
	if p*p == numerator && q*q == denominator {
		if negative {
			p = -p
		}
		return m.ArithmeticRealNumeralText(strconv.FormatInt(int64(p), 10) + "/" + strconv.FormatInt(int64(q), 10))
	}
	return m.arithmeticIrrationalSquareRoot(numerator, denominator, negative)
}

func (m *ASTManager) arithmeticIrrationalSquareRoot(n, denominator int32, negative bool) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	if m.arithmeticPlugin == nil {
		return ASTHandle{}, ErrASTHandle
	}
	w, err := m.arithmeticPlugin.algebraicWrapper()
	if err != nil {
		return ASTHandle{}, err
	}
	id, err := w.newID()
	if err != nil {
		return ASTHandle{}, err
	}
	if m.unboundedAlgebraicReads() {
		for len(w.squareRoots) <= int(id) {
			w.squareRoots = append(w.squareRoots, 0)
		}
		if negative {
			n = -n
		}
		w.squareRoots[id] = n
		if denominator > 1 {
			for len(w.squareDenominators) <= int(id) {
				w.squareDenominators = append(w.squareDenominators, 0)
			}
			w.squareDenominators[id] = denominator
		}
	} else {
		lo := floorSquareRoot(n / denominator)
		if negative {
			lo = -lo - 1
		}
		if err = w.manager.SetIsolatedSquareRootFraction(&w.values[id], n, denominator, lo); err != nil {
			return ASTHandle{}, errors.Join(err, w.releaseID(id))
		}
	}
	return m.arithmeticAlgebraicApp(w, id)
}

func (m *ASTManager) unboundedAlgebraicReads() bool {
	return !ASTSourceDebug && !diagnostic.TraceCompiled() && m.TraceStream == nil &&
		!m.BudgetField.InjectionArmed() && m.Limit.Limit == 0 && !m.Limit.CancelFlag()
}

func floorSquareRoot(n int32) int32 {
	lo := int64(math.Sqrt(float64(n)))
	for lo*lo > int64(n) {
		lo--
	}
	for (lo+1)*(lo+1) <= int64(n) {
		lo++
	}
	return int32(lo)
}

func squareRootInterval(signed, denominator int32) (n, lower int32) {
	if signed < 0 {
		return -signed, -floorSquareRoot(-signed/denominator) - 1
	}
	return signed, floorSquareRoot(signed / denominator)
}

func (m *ASTManager) arithmeticAlgebraicApp(w *astAlgebraicNumbersWrapper, id uint32) (ASTHandle, error) {
	p := w.owner
	parameter := astParameterRecord{Kind: ASTParameterExternal, external: id}
	if e := m.beginFormatInfo(false, p.family, 1, &parameter); e != nil {
		return ASTHandle{}, errors.Join(e, w.releaseID(id))
	}
	decl, e := m.mkFuncDecl(p.names[2], nil, m.external(p.fields[0]), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	if e != nil {
		return ASTHandle{}, errors.Join(e, w.releaseID(id))
	}
	app, e := m.mkApp(decl, nil)
	return m.external(app), e
}

// TranslateArithmeticExternalParameter implements the arithmetic plugin's
// cross-manager translation callback for an attached external parameter.
func (m *ASTManager) TranslateArithmeticExternalParameter(source ASTParameter, target *ASTManager) (ASTParameter, error) {
	if target == nil || target == m {
		return ASTParameter{}, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	if e := target.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer target.mu.Unlock()
	return m.translateArithmeticExternalParameter(source, target)
}

func (m *ASTManager) translateArithmeticExternalParameter(source ASTParameter, target *ASTManager) (ASTParameter, error) {
	if target == nil || target.Self != target || target == m {
		return ASTParameter{}, ErrASTHandle
	}
	sr, e := m.parameterOfKind(source, ASTParameterExternal)
	if e != nil {
		return ASTParameter{}, e
	}
	sw, e := m.arithmeticPlugin.algebraicWrapper()
	if e != nil {
		return ASTParameter{}, e
	}
	v, e := sw.value(sr.external)
	if e != nil {
		return ASTParameter{}, e
	}
	tw, e := target.arithmeticPlugin.algebraicWrapper()
	if e != nil {
		return ASTParameter{}, e
	}
	id, e := tw.mkID(sw.manager.AlgebraicRootCore, v)
	if e != nil {
		return ASTParameter{}, e
	}
	p, e := target.allocParameter()
	if e != nil {
		_ = tw.recycle(id)
		return ASTParameter{}, e
	}
	r := target.parameterAt(p.slot)
	r.Kind, r.external = ASTParameterExternal, id
	return p, nil
}
