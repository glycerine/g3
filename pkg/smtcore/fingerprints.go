package smtcore

// Source: src/smt/fingerprints.h and src/smt/fingerprints.cpp.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type SMTFingerprint struct {
	data     ast.ASTHandle
	args     []*SMTEnode
	dataHash uint32
	hash     uint32
}

// Allocate common binding widths together with their fingerprint so a saved
// result owns its arguments independently of the table and lookup scratch.
func newSMTFingerprint(args []*SMTEnode) *SMTFingerprint {
	var fingerprint *SMTFingerprint
	switch len(args) {
	case 0:
		return new(SMTFingerprint)
	case 1, 2:
		row := new(struct {
			SMTFingerprint
			bindings [2]*SMTEnode
		})
		fingerprint = &row.SMTFingerprint
		fingerprint.args = row.bindings[:len(args):len(args)]
	case 3, 4:
		row := new(struct {
			SMTFingerprint
			bindings [4]*SMTEnode
		})
		fingerprint = &row.SMTFingerprint
		fingerprint.args = row.bindings[:len(args):len(args)]
	case 5, 6:
		row := new(struct {
			SMTFingerprint
			bindings [6]*SMTEnode
		})
		fingerprint = &row.SMTFingerprint
		fingerprint.args = row.bindings[:len(args):len(args)]
	case 7, 8:
		row := new(struct {
			SMTFingerprint
			bindings [8]*SMTEnode
		})
		fingerprint = &row.SMTFingerprint
		fingerprint.args = row.bindings[:len(args):len(args)]
	default:
		fingerprint = &SMTFingerprint{args: make([]*SMTEnode, len(args))}
	}
	copy(fingerprint.args, args)
	return fingerprint
}

// A returned tuple can outlive the set. Keep hash chains in the table so
// retaining one result never retains other tuples from its bucket.
type smtFingerprintEntry struct {
	*SMTFingerprint
	previous int
}

func (f *SMTFingerprint) Data() ast.ASTHandle { return f.data }
func (f *SMTFingerprint) DataHash() uint32    { return f.dataHash }
func (f *SMTFingerprint) NumArgs() uint32     { return uint32(len(f.args)) }
func (f *SMTFingerprint) Args() []*SMTEnode   { return append([]*SMTEnode(nil), f.args...) }
func (f *SMTFingerprint) Arg(index uint32) (*SMTEnode, error) {
	if f == nil || index >= uint32(len(f.args)) {
		return nil, ast.ErrASTDomain
	}
	return f.args[index], nil
}

// SMTFingerprintSet uses concrete quantifier handles for the only retained
// instantiation of fingerprint::m_data. Entries preserve insertion order.
type SMTFingerprintSet struct {
	// A bucket's head and its links are one-based positions in insertion order.
	// Zero denotes an empty bucket or the end of a chain.
	buckets      []int
	fingerprints []smtFingerprintEntry
	scopes       []int
	// dummy holds roots for fingerprint_set::mk_dummy. Its length covers every
	// slot touched since the last pop, including the tail of an earlier, wider lookup.
	dummy []*SMTEnode
}

func NewSMTFingerprintSet() *SMTFingerprintSet {
	return &SMTFingerprintSet{}
}

func (s *SMTFingerprintSet) Size() uint32 {
	if s == nil {
		return 0
	}
	return uint32(len(s.fingerprints))
}

func smtFingerprintArgsEqual(left, right []*SMTEnode) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// smtFingerprintHash is fingerprint_hash_proc: the data and each
// argument's enode (identified by its owner) feed the hash.
func smtFingerprintHash(data ast.ASTHandle, args []*SMTEnode) uint64 {
	h := smtHandleMix(0xcbf29ce484222325^uint64(len(args)), data)
	for _, a := range args {
		if a == nil {
			h = smtHandleMix(h, ast.ASTHandle{})
			continue
		}
		h = smtHandleMix(h, a.owner)
	}
	return h
}

// The folded hash selects a bucket. A match also requires the same data
// handle and identical enode pointers.
func smtFingerprintBucketHash(data ast.ASTHandle, args []*SMTEnode) uint32 {
	h := smtFingerprintHash(data, args)
	return uint32(h ^ (h >> 32))
}

func (s *SMTFingerprintSet) bucket(hash uint32) int {
	return int(uint(hash) & uint(len(s.buckets)-1))
}

func (s *SMTFingerprintSet) find(hash uint32, data ast.ASTHandle, args []*SMTEnode) *SMTFingerprint {
	if len(s.buckets) == 0 {
		return nil
	}
	for pos := s.buckets[s.bucket(hash)]; pos != 0; {
		entry := s.fingerprints[pos-1]
		if f := entry.SMTFingerprint; f.hash == hash && f.data == data && smtFingerprintArgsEqual(f.args, args) {
			return f
		}
		pos = entry.previous
	}
	return nil
}

// grow doubles the bucket array, relinking the entries in insertion order
// so every chain stays newest first.
func (s *SMTFingerprintSet) grow() {
	n := 2 * len(s.buckets)
	if n < 64 {
		n = 64
	}
	s.buckets = make([]int, n)
	for i := range s.fingerprints {
		entry := &s.fingerprints[i]
		b := s.bucket(entry.hash)
		entry.previous = s.buckets[b]
		s.buckets[b] = i + 1
	}
}

// roots borrows args when all enodes are their own roots. Otherwise it
// fills the set's scratch slice and reports that the tuple changed.
func (s *SMTFingerprintSet) roots(args []*SMTEnode) ([]*SMTEnode, bool, error) {
	for i, a := range args {
		if a == nil || a.root == nil {
			return nil, false, ast.ErrASTDomain
		}
		if a.root == a {
			continue
		}
		if len(args) > len(s.dummy) {
			s.dummy = append(s.dummy, make([]*SMTEnode, len(args)-len(s.dummy))...)
		}
		roots := s.dummy[:len(args)]
		copy(roots, args[:i])
		roots[i] = a.root
		for j := i + 1; j < len(args); j++ {
			b := args[j]
			if b == nil || b.root == nil {
				return nil, false, ast.ErrASTDomain
			}
			roots[j] = b.root
		}
		return roots, true, nil
	}
	return args, false, nil
}

func (s *SMTFingerprintSet) Insert(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (*SMTFingerprint, error) {
	if s == nil || data.IsNull() {
		return nil, ast.ErrASTDomain
	}
	hash := smtFingerprintBucketHash(data, args)
	if s.find(hash, data, args) != nil {
		return nil, nil
	}
	roots, changed, err := s.roots(args)
	if err != nil {
		return nil, err
	}
	if changed {
		hash = smtFingerprintBucketHash(data, roots)
		if s.find(hash, data, roots) != nil {
			return nil, nil
		}
	}
	fingerprint := newSMTFingerprint(roots)
	fingerprint.data, fingerprint.dataHash, fingerprint.hash = data, dataHash, hash
	s.fingerprints = append(s.fingerprints, smtFingerprintEntry{SMTFingerprint: fingerprint})
	n := len(s.fingerprints)
	if n > len(s.buckets) {
		s.grow()
	} else {
		b := s.bucket(hash)
		s.fingerprints[n-1].previous = s.buckets[b]
		s.buckets[b] = n
	}
	return fingerprint, nil
}

func (s *SMTFingerprintSet) Contains(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (bool, error) {
	if s == nil || data.IsNull() {
		return false, ast.ErrASTDomain
	}
	_ = dataHash // Source equality does not compare its cached hash.
	if s.find(smtFingerprintBucketHash(data, args), data, args) != nil {
		return true, nil
	}
	roots, changed, err := s.roots(args)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	return s.find(smtFingerprintBucketHash(data, roots), data, roots) != nil, nil
}

func (s *SMTFingerprintSet) SlowContains(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (bool, error) {
	return s.Contains(data, dataHash, args)
}

func (s *SMTFingerprintSet) Reset() {
	if s == nil {
		return
	}
	s.dummy = nil
	s.buckets = nil
	s.fingerprints = nil
}

func (s *SMTFingerprintSet) PushScope() error {
	if s == nil {
		return ast.ErrASTHandle
	}
	s.scopes = append(s.scopes, len(s.fingerprints))
	return nil
}

func (s *SMTFingerprintSet) PopScopes(count uint32) error {
	if s == nil || count == 0 || count > uint32(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	newLevel := len(s.scopes) - int(count)
	oldSize := s.scopes[newLevel]
	// Both insertion and grow link newer entries ahead of older ones.
	// Walking this suffix backwards always removes the current bucket head.
	for i := len(s.fingerprints) - 1; i >= oldSize; i-- {
		entry := s.fingerprints[i]
		s.buckets[s.bucket(entry.hash)] = entry.previous
	}
	clear(s.fingerprints[oldSize:])
	clear(s.dummy)
	s.dummy = s.dummy[:0]
	s.fingerprints = s.fingerprints[:oldSize]
	s.scopes = s.scopes[:newLevel]
	return nil
}

func (s *SMTFingerprintSet) Display(manager *ast.ASTManager, out io.Writer) error {
	if s == nil || manager == nil || out == nil {
		return ast.ErrASTDomain
	}
	if _, err := io.WriteString(out, "fingerprints:\n"); err != nil {
		return err
	}
	for _, entry := range s.fingerprints {
		fingerprint := entry.SMTFingerprint
		dataID, err := manager.GetASTID(fingerprint.data)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "#%d #%d", dataID, fingerprint.dataHash); err != nil {
			return err
		}
		for _, argument := range fingerprint.args {
			ownerID, err := argument.OwnerID()
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, " #%d", ownerID); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
