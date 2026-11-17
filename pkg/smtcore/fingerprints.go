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
	// byHash is fingerprint_set::m_set, hashed like fingerprint_hash_proc
	// over the data and every argument; buckets hold only collisions.
	byHash       map[uint64][]*SMTFingerprint
	fingerprints []*SMTFingerprint
	scopes       []int
}

func NewSMTFingerprintSet() *SMTFingerprintSet {
	return &SMTFingerprintSet{byHash: make(map[uint64][]*SMTFingerprint)}
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

func (s *SMTFingerprintSet) find(hash uint64, data ast.ASTHandle, args []*SMTEnode) *SMTFingerprint {
	for _, fingerprint := range s.byHash[hash] {
		if fingerprint.data == data && smtFingerprintArgsEqual(fingerprint.args, args) {
			return fingerprint
		}
	}
	return nil
}

func smtFingerprintRoots(args []*SMTEnode) ([]*SMTEnode, error) {
	roots := make([]*SMTEnode, len(args))
	for i, argument := range args {
		if argument == nil || argument.root == nil {
			return nil, ast.ErrASTDomain
		}
		roots[i] = argument.root
	}
	return roots, nil
}

func (s *SMTFingerprintSet) Insert(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (*SMTFingerprint, error) {
	if s == nil || data.IsNull() {
		return nil, ast.ErrASTDomain
	}
	if s.find(smtFingerprintHash(data, args), data, args) != nil {
		return nil, nil
	}
	roots, err := smtFingerprintRoots(args)
	if err != nil {
		return nil, err
	}
	hash := smtFingerprintHash(data, roots)
	if s.find(hash, data, roots) != nil {
		return nil, nil
	}
	fingerprint := &SMTFingerprint{data: data, dataHash: dataHash, args: roots, hash: hash}
	s.byHash[hash] = append(s.byHash[hash], fingerprint)
	s.fingerprints = append(s.fingerprints, fingerprint)
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
	roots, err := smtFingerprintRoots(args)
	if err != nil {
		return false, err
	}
	return s.find(smtFingerprintHash(data, roots), data, roots) != nil, nil
}

func (s *SMTFingerprintSet) SlowContains(data ast.ASTHandle, dataHash uint32, args []*SMTEnode) (bool, error) {
	return s.Contains(data, dataHash, args)
}

func (s *SMTFingerprintSet) Reset() {
	if s == nil {
		return
	}
	s.byHash = make(map[uint64][]*SMTFingerprint)
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
		bucket := s.byHash[fingerprint.hash]
		for j := len(bucket) - 1; j >= 0; j-- {
			if bucket[j] == fingerprint {
				copy(bucket[j:], bucket[j+1:])
				bucket[len(bucket)-1] = nil
				bucket = bucket[:len(bucket)-1]
				break
			}
		}
		// A truncated Go backing array still owns its pointer slots.
		s.fingerprints[i] = nil
		if len(bucket) == 0 {
			delete(s.byHash, fingerprint.hash)
		} else {
			s.byHash[fingerprint.hash] = bucket
		}
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
