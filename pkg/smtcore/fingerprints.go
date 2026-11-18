package smtcore

// Source: src/smt/fingerprints.h and src/smt/fingerprints.cpp.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type SMTFingerprint struct {
	data     ast.ASTHandle
	dataHash uint32
	args     []*SMTEnode
	hash     uint64 // fingerprint_hash_proc of data and args, for removal
	// next chains the fingerprints of one SMTFingerprintSet bucket, the
	// most recently inserted first.
	next *SMTFingerprint
	// inline backs args when it is short (one allocation per fingerprint).
	inline [4]*SMTEnode
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
	// buckets is fingerprint_set::m_set, hashed like fingerprint_hash_proc
	// over the data and every argument: a power-of-two array of chains
	// linked through SMTFingerprint.next. Each chain lists its entries
	// newest first, so PopScopes, which removes in reverse insertion
	// order, always unlinks a chain head. Lookups are exact-match only;
	// nothing iterates the buckets.
	buckets      []*SMTFingerprint
	fingerprints []*SMTFingerprint
	scopes       []int
	// dummy is the argument buffer of fingerprint_set::mk_dummy; the
	// stored fingerprint copies it.
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

func (s *SMTFingerprintSet) bucket(hash uint64) uint64 {
	return (hash ^ hash>>32) & uint64(len(s.buckets)-1)
}

func (s *SMTFingerprintSet) find(hash uint64, data ast.ASTHandle, args []*SMTEnode) *SMTFingerprint {
	if len(s.buckets) == 0 {
		return nil
	}
	for fingerprint := s.buckets[s.bucket(hash)]; fingerprint != nil; fingerprint = fingerprint.next {
		if fingerprint.hash == hash && fingerprint.data == data && smtFingerprintArgsEqual(fingerprint.args, args) {
			return fingerprint
		}
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
	s.buckets = make([]*SMTFingerprint, n)
	for _, fingerprint := range s.fingerprints {
		b := s.bucket(fingerprint.hash)
		fingerprint.next = s.buckets[b]
		s.buckets[b] = fingerprint
	}
}

// dummyRoots fills the mk_dummy buffer with the roots of args.
func (s *SMTFingerprintSet) dummyRoots(args []*SMTEnode) ([]*SMTEnode, error) {
	s.dummy = s.dummy[:0]
	for _, argument := range args {
		if argument == nil || argument.root == nil {
			return nil, ast.ErrASTDomain
		}
		s.dummy = append(s.dummy, argument.root)
	}
	return s.dummy, nil
}

func (s *SMTFingerprintSet) Insert(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (*SMTFingerprint, error) {
	if s == nil || data.IsNull() {
		return nil, ast.ErrASTDomain
	}
	hash := smtFingerprintHash(data, args)
	if s.find(hash, data, args) != nil {
		return nil, nil
	}
	dummy, err := s.dummyRoots(args)
	if err != nil {
		return nil, err
	}
	// When every argument is its own root the second probe repeats the
	// first one exactly.
	if !smtFingerprintArgsEqual(dummy, args) {
		hash = smtFingerprintHash(data, dummy)
		if s.find(hash, data, dummy) != nil {
			return nil, nil
		}
	}
	fingerprint := &SMTFingerprint{data: data, dataHash: dataHash, hash: hash}
	if len(dummy) <= len(fingerprint.inline) {
		fingerprint.args = fingerprint.inline[:len(dummy):len(dummy)]
		copy(fingerprint.args, dummy)
	} else {
		fingerprint.args = append([]*SMTEnode(nil), dummy...)
	}
	s.fingerprints = append(s.fingerprints, fingerprint)
	if len(s.fingerprints) > len(s.buckets) {
		s.grow() // links fingerprint too
	} else {
		b := s.bucket(hash)
		fingerprint.next = s.buckets[b]
		s.buckets[b] = fingerprint
	}
	return fingerprint, nil
}

func (s *SMTFingerprintSet) Contains(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (bool, error) {
	if s == nil || data.IsNull() {
		return false, ast.ErrASTDomain
	}
	_ = dataHash // Source equality does not compare its cached hash.
	if s.find(smtFingerprintHash(data, args), data, args) != nil {
		return true, nil
	}
	dummy, err := s.dummyRoots(args)
	if err != nil {
		return false, err
	}
	return s.find(smtFingerprintHash(data, dummy), data, dummy) != nil, nil
}

func (s *SMTFingerprintSet) SlowContains(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (bool, error) {
	return s.Contains(data, dataHash, args)
}

func (s *SMTFingerprintSet) Reset() {
	if s == nil {
		return
	}
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
	for i := len(s.fingerprints) - 1; i >= oldSize; i-- {
		fingerprint := s.fingerprints[i]
		for link := &s.buckets[s.bucket(fingerprint.hash)]; *link != nil; link = &(*link).next {
			if *link == fingerprint {
				*link = fingerprint.next
				break
			}
		}
		fingerprint.next = nil
		s.fingerprints[i] = nil
	}
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
	for _, fingerprint := range s.fingerprints {
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
