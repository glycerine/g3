package smtcore

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

// The watches do the propagation. This directory preserves insertion order for
// Clauses and the two watches to remove during Close. Most solvers fit both
// literal indices in four bytes together. A solver that outgrows them widens
// its own directory once, keeping the full SMTLiteral range.
type smtBinaryClauseDirectory struct {
	short  [][2]uint16
	wide   [][2]SMTLiteral
	flags  []byte
	proofs []smtBinaryClauseProof
}

type smtBinaryClauseProof struct {
	index int
	proof ast.ASTHandle
}

const (
	smtBinaryRoot byte = 1 << iota
	smtBinaryLemma
	smtBinaryFirstWatchRemoved
	smtBinaryBothWatchesRemoved
)

func (d *smtBinaryClauseDirectory) count() int {
	if d.wide != nil {
		return len(d.wide)
	}
	return len(d.short)
}

func (d *smtBinaryClauseDirectory) pair(index int) [2]SMTLiteral {
	if d.wide != nil {
		return d.wide[index]
	}
	pair := d.short[index]
	return [2]SMTLiteral{SMTLiteral(pair[0]), SMTLiteral(pair[1])}
}

func (d *smtBinaryClauseDirectory) add(first, second SMTLiteral, root, lemma bool, proof ast.ASTHandle) {
	index := d.count()
	if d.wide == nil && uint32(first) <= math.MaxUint16 && uint32(second) <= math.MaxUint16 {
		d.short = append(d.short, [2]uint16{uint16(first), uint16(second)})
	} else {
		if d.wide == nil {
			d.wide = make([][2]SMTLiteral, index, index+1)
			for i, p := range d.short {
				d.wide[i] = [2]SMTLiteral{SMTLiteral(p[0]), SMTLiteral(p[1])}
			}
			d.short = nil
		}
		d.wide = append(d.wide, [2]SMTLiteral{first, second})
	}
	var flags byte
	if root {
		flags |= smtBinaryRoot
	}
	if lemma {
		flags |= smtBinaryLemma
	}
	if flags != 0 {
		d.setFlags(index, flags)
	}
	if !proof.IsNull() {
		d.proofs = append(d.proofs, smtBinaryClauseProof{index, proof})
	}
}

func (d *smtBinaryClauseDirectory) flagsAt(index int) byte {
	if index < len(d.flags) {
		return d.flags[index]
	}
	return 0
}

func (d *smtBinaryClauseDirectory) setFlags(index int, flags byte) {
	if index >= len(d.flags) {
		d.flags = append(d.flags, make([]byte, index+1-len(d.flags))...)
	}
	d.flags[index] = flags
}

func (d *smtBinaryClauseDirectory) appendSnapshots(out []SMTInternalizedClause) []SMTInternalizedClause {
	proofs := d.proofs
	for index := 0; index < d.count(); index++ {
		p := d.pair(index)
		flags := d.flagsAt(index)
		record := SMTInternalizedClause{
			Literals: []SMTLiteral{p[0], p[1]},
			Root:     flags&smtBinaryRoot != 0, Lemma: flags&smtBinaryLemma != 0,
			Binary: flags&smtBinaryBothWatchesRemoved == 0,
		}
		if len(proofs) != 0 && proofs[0].index == index {
			record.Proof = proofs[0].proof
			proofs = proofs[1:]
		}
		out = append(out, record)
	}
	return out
}

func (s *SMTInternalizerState) checkBinaryWatches() error {
	d := &s.binaryClauses
	for index := 0; index < d.count(); index++ {
		p := d.pair(index)
		if d.flagsAt(index)&(smtBinaryFirstWatchRemoved|smtBinaryBothWatchesRemoved) != 0 ||
			s.watches[p[0].Negated().Index()].FindLiteral(p[1]) < 0 ||
			s.watches[p[1].Negated().Index()].FindLiteral(p[0]) < 0 {
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (s *SMTInternalizerState) detachBinaryClauses() error {
	d := &s.binaryClauses
	for index := d.count() - 1; index >= 0; index-- {
		flags := d.flagsAt(index)
		if flags&smtBinaryBothWatchesRemoved != 0 {
			continue
		}
		p := d.pair(index)
		if flags&smtBinaryFirstWatchRemoved == 0 {
			if !s.watches[p[0].Negated().Index()].RemoveLiteral(p[1]) {
				return ast.ErrASTDomain
			}
		}
		if !s.watches[p[1].Negated().Index()].RemoveLiteral(p[0]) {
			// A second Close resumes exactly at the failed watch.
			d.setFlags(index, flags|smtBinaryFirstWatchRemoved)
			return ast.ErrASTDomain
		}
		d.setFlags(index, flags|smtBinaryBothWatchesRemoved)
	}
	return nil
}
