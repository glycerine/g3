package ast

// Source: src/util/sorting_network.h:340-342,491-499,516-518,561,612-614,
// 675-678,732-735,769-771,879-883,1017-1018; psort_nw<card2bv_rewriter>'s
// ctx.pp is mk_ismt2_pp(lit, m) (src/ast/rewriter/pb2bv_rewriter.cpp:386).

import (
	"fmt"
	"io"
)

// psortTraceLiteral is ctx.pp(out, lit): mk_ismt2_pp of the literal.
func (s *PsortASTNetwork) psortTraceLiteral(w io.Writer, h ASTHandle) error {
	m := s.context.manager
	c, p, g, err := m.wellSortedTraceParams(nil, nil, nil)
	if err != nil {
		return err
	}
	text, err := m.wellSortedTraceSMT2(h, c, p, g)
	if err != nil {
		return err
	}
	psortTraceText(w, text)
	return nil
}

// psortTracePP is psort_nw::pp(out, n, lits): each literal and a space.
func (s *PsortASTNetwork) psortTracePP(w io.Writer, n uint32, p psortASTPointer) error {
	for i := uint32(0); i < n; i++ {
		h, err := p.at(i)
		if err != nil {
			return err
		}
		if err = s.psortTraceLiteral(w, h); err != nil {
			return err
		}
		psortTraceText(w, " ")
	}
	return nil
}

// psortTracePPVector is psort_nw::pp(out, lits) for a literal_vector.
func (s *PsortASTNetwork) psortTracePPVector(w io.Writer, v *ASTLiteralVector) error {
	n, err := v.Size()
	if err != nil {
		return err
	}
	p, err := psortVectorPointer(v)
	if err != nil {
		return err
	}
	return s.psortTracePP(w, n, p)
}

// psortTraceAddress streams a literal as the source's raw expr* value.
func (s *PsortASTNetwork) psortTraceAddress(w io.Writer, h ASTHandle) error {
	text, err := s.context.manager.nodeAddressText(h)
	if err != nil {
		return err
	}
	psortTraceText(w, text)
	return nil
}

// nodeAddressText prints a live node's record address, the Go stand-in
// for streaming an ast pointer.
func (m *ASTManager) nodeAddressText(h ASTHandle) (string, error) {
	r, err := m.readNode(h)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%p", m.nodeAt(r.slot)), nil
}

func (s *PsortASTNetwork) traceAtMostOne(full bool, n uint32, p psortASTPointer) {
	s.tracePBEntry(340, "mk_at_most_1", func(w io.Writer) error {
		if full {
			psortTraceText(w, "full ")
		} else {
			psortTraceText(w, "partial ")
		}
		for i := uint32(0); i < n; i++ {
			h, err := p.at(i)
			if err != nil {
				return err
			}
			if err = s.psortTraceAddress(w, h); err != nil {
				return err
			}
			psortTraceText(w, " ")
		}
		psortTraceText(w, "\n")
		return nil
	})
}

func (s *PsortASTNetwork) traceDualize(n uint32, in *ASTLiteralVector, k uint32) {
	s.tracePBEntry(516, "dualize", func(w io.Writer) error {
		psortTraceUint(w, n)
		psortTraceText(w, ": ")
		if err := s.psortTracePPVector(w, in); err != nil {
			return err
		}
		psortTraceText(w, " ~ ")
		psortTraceUint(w, k)
		psortTraceText(w, "\n")
		return nil
	})
}

func (s *PsortASTNetwork) traceAddClause(n uint32, p psortASTPointer) {
	s.tracePBEntry(561, "add_clause", func(w io.Writer) error {
		for i := uint32(0); i < n; i++ {
			h, err := p.at(i)
			if err != nil {
				return err
			}
			if err = s.psortTraceAddress(w, h); err != nil {
				return err
			}
			psortTraceText(w, " ")
		}
		psortTraceText(w, "\n")
		return nil
	})
}

func (s *PsortASTNetwork) traceAddSubset(k, offset, n uint32, lits *ASTLiteralVector) {
	s.tracePBEntry(1017, "add_subset", func(w io.Writer) error {
		psortTraceText(w, "k:")
		psortTraceUint(w, k)
		psortTraceText(w, " offset: ")
		psortTraceUint(w, offset)
		psortTraceText(w, " n: ")
		psortTraceUint(w, n)
		psortTraceText(w, " ")
		if err := s.psortTracePPVector(w, lits); err != nil {
			return err
		}
		psortTraceText(w, "\n")
		return nil
	})
}

// card, sorting, merge, interleave and smerge run their source body and
// then the trailing TRACE that every branch of the body reaches.

func (s *PsortASTNetwork) card(k, n uint32, p psortASTPointer, out *ASTLiteralVector) error {
	if err := s.cardBody(k, n, p, out); err != nil {
		return err
	}
	s.tracePBEntry(612, "card", func(w io.Writer) error {
		psortTraceText(w, "card k: ")
		psortTraceUint(w, k)
		psortTraceText(w, " n: ")
		psortTraceUint(w, n)
		psortTraceText(w, "\nin:")
		if err := s.psortTracePP(w, n, p); err != nil {
			return err
		}
		psortTraceText(w, "\nout:")
		if err := s.psortTracePPVector(w, out); err != nil {
			return err
		}
		psortTraceText(w, "\n")
		return nil
	})
	return nil
}

func (s *PsortASTNetwork) sorting(n uint32, p psortASTPointer, out *ASTLiteralVector) error {
	if err := s.sortingBody(n, p, out); err != nil {
		return err
	}
	s.tracePBEntry(769, "sorting", func(w io.Writer) error {
		psortTraceText(w, "sorting: ")
		psortTraceUint(w, n)
		psortTraceText(w, "\nin:")
		if err := s.psortTracePP(w, n, p); err != nil {
			return err
		}
		psortTraceText(w, "\nout:")
		if err := s.psortTracePPVector(w, out); err != nil {
			return err
		}
		psortTraceText(w, "\n")
		return nil
	})
	return nil
}

func (s *PsortASTNetwork) merge(a uint32, ap psortASTPointer, b uint32, bp psortASTPointer, out *ASTLiteralVector) error {
	if err := s.mergeBody(a, ap, b, bp, out); err != nil {
		return err
	}
	s.tracePBEntry(675, "merge", func(w io.Writer) error {
		return s.traceMergeRecord(w, "merge a: ", a, " b: ", b, "\n", ap, bp, out)
	})
	return nil
}

func (s *PsortASTNetwork) mergeSmall(c, a uint32, ap psortASTPointer, b uint32, bp psortASTPointer, out *ASTLiteralVector) error {
	if err := s.mergeSmallBody(c, a, ap, b, bp, out); err != nil {
		return err
	}
	s.tracePBEntry(879, "smerge", func(w io.Writer) error {
		psortTraceText(w, "smerge: c:")
		psortTraceUint(w, c)
		return s.traceMergeRecord(w, " a:", a, " b:", b, "\n", ap, bp, out)
	})
	return nil
}

// traceMergeRecord writes the header tail and the "a:", "b:" and "out:"
// lines shared by the merge and smerge completion records.
func (s *PsortASTNetwork) traceMergeRecord(w io.Writer, aLabel string, a uint32, bLabel string, b uint32, end string, ap, bp psortASTPointer, out *ASTLiteralVector) error {
	psortTraceText(w, aLabel)
	psortTraceUint(w, a)
	psortTraceText(w, bLabel)
	psortTraceUint(w, b)
	psortTraceText(w, end)
	psortTraceText(w, "a:")
	if err := s.psortTracePP(w, a, ap); err != nil {
		return err
	}
	psortTraceText(w, "\nb:")
	if err := s.psortTracePP(w, b, bp); err != nil {
		return err
	}
	psortTraceText(w, "\nout:")
	if err := s.psortTracePPVector(w, out); err != nil {
		return err
	}
	psortTraceText(w, "\n")
	return nil
}

func (s *PsortASTNetwork) interleave(a, b, out *ASTLiteralVector) error {
	if err := s.interleaveBody(a, b, out); err != nil {
		return err
	}
	s.tracePBEntry(732, "interleave", func(w io.Writer) error {
		az, err := a.Size()
		if err != nil {
			return err
		}
		bz, err := b.Size()
		if err != nil {
			return err
		}
		psortTraceText(w, "interleave: ")
		psortTraceUint(w, az)
		psortTraceText(w, " ")
		psortTraceUint(w, bz)
		psortTraceText(w, "\na: ")
		if err = s.psortTracePPVector(w, a); err != nil {
			return err
		}
		psortTraceText(w, "\nb: ")
		if err = s.psortTracePPVector(w, b); err != nil {
			return err
		}
		psortTraceText(w, "\nout: ")
		if err = s.psortTracePPVector(w, out); err != nil {
			return err
		}
		psortTraceText(w, "\n")
		return nil
	})
	return nil
}
