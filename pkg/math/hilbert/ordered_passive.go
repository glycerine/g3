package hilbert

import (
	"math"

	"github.com/glycerine/g3/pkg/util/heap"
)

// orderedPassive is hilbert_basis::passive2
// (src/math/hilbert/hilbert_basis.cpp:488-643). Slot ids v index pas, psos,
// weight and sumAbs; exhausted slots go on a LIFO free list and are reused
// by insert. The slots are ordered by a port of util/heap.h over weight[v],
// so ties pop in the source heap order.
type orderedPassive struct {
	owner       *Basis
	posSos      []*vector
	negSos      []*vector
	posSosSum   []uint64
	negSosSum   []uint64
	sumAbs      []uint64
	psos        []int
	pas         []*vector
	weight      []uint64
	freeList    []int32
	heap        *heap.Int
	initialized bool
}

func newOrderedPassive(owner *Basis, support []*vector) *orderedPassive {
	p := &orderedPassive{owner: owner}
	p.init(support)
	return p
}

// init is passive2::init. Sums are computed lazily on first use so that the
// constructor keeps its error-free signature; the source sums cannot fail.
func (p *orderedPassive) init(support []*vector) {
	for _, value := range support {
		if value.weight > 0 {
			p.posSos = append(p.posSos, value)
		} else {
			p.negSos = append(p.negSos, value)
		}
	}
}

func (p *orderedPassive) prepare() error {
	if p.initialized {
		return nil
	}
	for _, value := range p.posSos {
		sum, err := vectorMagnitude(value)
		if err != nil {
			return err
		}
		p.posSosSum = append(p.posSosSum, sum)
	}
	for _, value := range p.negSos {
		sum, err := vectorMagnitude(value)
		if err != nil {
			return err
		}
		p.negSosSum = append(p.negSosSum, sum)
	}
	h, err := heap.New(10, heap.Config{Less: func(v1, v2 int32) bool { return p.weight[v1] < p.weight[v2] }})
	if err != nil {
		return err
	}
	p.heap = h
	p.initialized = true
	return nil
}

func (p *orderedPassive) insert(value *vector, offset int) error {
	if err := p.prepare(); err != nil {
		return err
	}
	sum, err := vectorMagnitude(value)
	if err != nil {
		return err
	}
	var v int32
	if len(p.freeList) == 0 {
		if len(p.pas) >= math.MaxInt32 {
			return ErrOverflow
		}
		v = int32(len(p.pas))
		p.pas = append(p.pas, value)
		p.psos = append(p.psos, offset)
		p.weight = append(p.weight, 0)
		if err := p.heap.SetBounds(v + 1); err != nil {
			return err
		}
		p.sumAbs = append(p.sumAbs, sum)
	} else {
		v = p.freeList[len(p.freeList)-1]
		p.freeList = p.freeList[:len(p.freeList)-1]
		p.pas[v] = value
		p.psos[v] = offset
		p.weight[v] = 0
		p.sumAbs[v] = sum
	}
	return p.nextResolvable(value.weight > 0, v)
}

func (p *orderedPassive) empty() bool {
	return p.heap == nil || p.heap.Empty()
}

// pop preserves passive2::pop's returned support offset. When resolution
// crosses zero, the new result restarts at the first opposite support.
func (p *orderedPassive) pop() (support, passive *vector, offset int, err error) {
	val, err := p.heap.EraseMin()
	if err != nil {
		return nil, nil, 0, err
	}
	passive = p.pas[val]
	oldWeight := passive.weight
	isPositive := oldWeight > 0
	offset = p.psos[val]
	if isPositive {
		support = p.negSos[offset]
	} else {
		support = p.posSos[offset]
	}
	p.psos[val]++
	if err = p.nextResolvable(isPositive, val); err != nil {
		return nil, nil, 0, err
	}
	newWeight, addErr := checkedAdd(support.weight, oldWeight)
	if addErr != nil {
		return nil, nil, 0, addErr
	}
	if (newWeight > 0) != (oldWeight > 0) {
		offset = 0
	}
	return support, passive, offset, nil
}

func (p *orderedPassive) nextResolvable(isPositive bool, v int32) error {
	pas := p.pas[v]
	soss, sums := p.posSos, p.posSosSum
	if isPositive {
		soss, sums = p.negSos, p.negSosSum
	}
	for p.psos[v] < len(soss) {
		psos := p.psos[v]
		sos := soss[psos]
		if p.owner.canResolve(pas, sos) {
			if math.MaxUint64-p.sumAbs[v] < sums[psos] {
				return ErrOverflow
			}
			p.weight[v] = p.sumAbs[v] + sums[psos]
			return p.heap.Insert(v)
		}
		p.psos[v]++
	}
	// Add pas to the free list; the source marks the slot invalid.
	p.freeList = append(p.freeList, v)
	p.psos[v] = math.MaxInt
	p.pas[v] = nil
	return nil
}
