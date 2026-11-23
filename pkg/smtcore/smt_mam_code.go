package smtcore

// Source: src/smt/mam.cpp label_hasher, the instruction set, code_tree and
// code_tree_manager (lines 60-776).

import (
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/approx"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

// smtMAMTrail undoes pattern-matcher changes in reverse order on pop.
// Adjacent merges often save identical label sets. Keep the destinations in
// order and store one copy of the two sets for each run.
type smtMAMTrail struct {
	nodes     []*SMTEnode
	labelRuns []smtMAMLabelRun
	undo      []smtMAMUndo
	scopes    []smtMAMScope
}

type smtMAMLabelRun struct {
	start              int
	labels, propagated approx.UintSet
}

type smtMAMUndo struct {
	restore   func()
	nodeLimit int
}

type smtMAMScope struct {
	nodeLimit, undoLimit int
}

func (t *smtMAMTrail) push(restore func()) {
	t.undo = append(t.undo, smtMAMUndo{restore, len(t.nodes)})
}

func (t *smtMAMTrail) saveLabels(n *SMTEnode) {
	last := len(t.labelRuns) - 1
	if last < 0 || t.labelRuns[last].labels != n.labels || t.labelRuns[last].propagated != n.propagatedLabels {
		t.labelRuns = append(t.labelRuns, smtMAMLabelRun{len(t.nodes), n.labels, n.propagatedLabels})
	}
	t.nodes = append(t.nodes, n)
}

func (t *smtMAMTrail) pushScope() {
	t.scopes = append(t.scopes, smtMAMScope{len(t.nodes), len(t.undo)})
}

func (t *smtMAMTrail) popScope(n int) {
	if n <= 0 || n > len(t.scopes) {
		return
	}
	level := len(t.scopes) - n
	t.undoTo(t.scopes[level])
	t.scopes = t.scopes[:level]
}

func (t *smtMAMTrail) undoTo(scope smtMAMScope) {
	for len(t.undo) > scope.undoLimit {
		i := len(t.undo) - 1
		entry := t.undo[i]
		t.restoreLabels(entry.nodeLimit)
		entry.restore()
		t.undo[i].restore = nil
		t.undo = t.undo[:i]
	}
	t.restoreLabels(scope.nodeLimit)
}

func (t *smtMAMTrail) restoreLabels(limit int) {
	for len(t.nodes) > limit {
		last := len(t.labelRuns) - 1
		run := t.labelRuns[last]
		start := max(limit, run.start)
		for j := len(t.nodes) - 1; j >= start; j-- {
			node := t.nodes[j]
			node.labels, node.propagatedLabels = run.labels, run.propagated
			t.nodes[j] = nil
		}
		t.nodes = t.nodes[:start]
		if start == run.start {
			t.labelRuns = t.labelRuns[:last]
		}
	}
}

func (t *smtMAMTrail) reset() {
	t.popScope(len(t.scopes))
	t.undoTo(smtMAMScope{})
	*t = smtMAMTrail{}
}

func (t *smtMAMTrail) numScopes() int { return len(t.scopes) }

// smtMAMLabelHasher is label_hasher: lbl_id -> mix(17, 3, lbl_id) & 63.
type smtMAMLabelHasher struct {
	lbl2hash []int8
}

func (h *smtMAMLabelHasher) hash(declID uint32) uint8 {
	if int(declID) >= len(h.lbl2hash) {
		oldLen := len(h.lbl2hash)
		h.lbl2hash = append(h.lbl2hash, make([]int8, int(declID)+1-oldLen)...)
		for i := oldLen; i < len(h.lbl2hash); i++ {
			h.lbl2hash[i] = -1
		}
	}
	if h.lbl2hash[declID] == -1 {
		h.lbl2hash[declID] = int8(uhash.Mix(17, 3, declID) & 63)
	}
	return uint8(h.lbl2hash[declID])
}

func (h *smtMAMLabelHasher) display(out io.Writer) {
	var b strings.Builder
	b.WriteString("lbl-hasher:\n")
	first := true
	for i, v := range h.lbl2hash {
		if v != -1 {
			if !first {
				b.WriteString(", ")
			}
			first = false
			fmt.Fprintf(&b, "%d -> %d", i, v)
		}
	}
	b.WriteString("\n")
	_, _ = io.WriteString(out, b.String())
}

type smtMAMOpcode uint8

const (
	mamINIT1 smtMAMOpcode = iota
	mamINIT2
	mamINIT3
	mamINIT4
	mamINIT5
	mamINIT6
	mamINITN
	mamBIND1
	mamBIND2
	mamBIND3
	mamBIND4
	mamBIND5
	mamBIND6
	mamBINDN
	mamYIELD1
	mamYIELD2
	mamYIELD3
	mamYIELD4
	mamYIELD5
	mamYIELD6
	mamYIELDN
	mamCOMPARE
	mamCHECK
	mamFILTER
	mamCFILTER
	mamPFILTER
	mamCHOOSE
	mamNOOP
	mamCONTINUE
	mamGETENODE
	mamGETCGR1
	mamGETCGR2
	mamGETCGR3
	mamGETCGR4
	mamGETCGR5
	mamGETCGR6
	mamGETCGRN
	mamISCGR
)

// Joint tags of cont::m_joints.
const (
	mamNullTag       = 0
	mamGroundTermTag = 1
	mamVarTag        = 2
	mamNestedVarTag  = 3
)

// smtMAMJoint2 is joint2: (f ... (g ... X ...) ...).
type smtMAMJoint2 struct {
	decl   ast.ASTHandle
	argPos uint32
	reg    uint32
}

// smtMAMJoint is one tagged cont joint.
type smtMAMJoint struct {
	tag    uint8
	enode  *SMTEnode // GROUND_TERM_TAG
	reg    uint32    // VAR_TAG
	joint2 *smtMAMJoint2
}

// smtMAMInstr is the instruction union: m_opcode and m_next are shared, the
// rest belongs to the opcode's struct.
type smtMAMInstr struct {
	op   smtMAMOpcode
	next *smtMAMInstr

	numArgs uint32 // initn, bind, get_cgr, is_cgr, cont
	reg1    uint32 // compare
	reg2    uint32
	reg     uint32 // check, filter
	enode   *SMTEnode
	lblSet  approx.UintSet // filter, pcheck, cont, get_cgr
	oreg    uint32         // get_enode, cont, bind, get_cgr
	ireg    uint32         // bind, is_cgr
	alt     *smtMAMInstr   // choose
	label   ast.ASTHandle  // cont, bind, get_cgr, is_cgr (func_decl)
	joints  []smtMAMJoint  // cont
	iregs   []uint32       // get_cgr, is_cgr
	qa      ast.ASTHandle  // yield
	pat     ast.ASTHandle
	binding []uint32 // yield
}

func (i *smtMAMInstr) isInit() bool { return i.op <= mamINITN }

// smtMAMCodeTree is code_tree.
type smtMAMCodeTree struct {
	rootLbl          ast.ASTHandle
	numArgs          uint32
	filterCandidates bool
	numRegs          uint32
	numChoices       uint32
	root             *smtMAMInstr
	candidates       []*SMTEnode
}

func (t *smtMAMCodeTree) addCandidate(n *SMTEnode) { t.candidates = append(t.candidates, n) }
func (t *smtMAMCodeTree) hasCandidates() bool      { return len(t.candidates) != 0 }
func (t *smtMAMCodeTree) resetCandidates() {
	clear(t.candidates) // the backing array keeps no enodes alive
	t.candidates = t.candidates[:0]
}

// smtMAMCodeTreeManager is code_tree_manager.
type smtMAMCodeTreeManager struct {
	hasher *smtMAMLabelHasher
	trail  *smtMAMTrail
	m      *ast.ASTManager
}

func mamCountedOp(base, last smtMAMOpcode, n uint32) smtMAMOpcode {
	if n <= 6 {
		return base + smtMAMOpcode(n-1)
	}
	return last
}

func (c *smtMAMCodeTreeManager) mkInit(n uint32) *smtMAMInstr {
	return &smtMAMInstr{op: mamCountedOp(mamINIT1, mamINITN, n), numArgs: n}
}

func (c *smtMAMCodeTreeManager) mkCodeTree(lbl ast.ASTHandle, numArgs uint32, filterCandidates bool) *smtMAMCodeTree {
	return &smtMAMCodeTree{rootLbl: lbl, numArgs: numArgs, filterCandidates: filterCandidates, numRegs: numArgs + 1, root: c.mkInit(numArgs)}
}

func (c *smtMAMCodeTreeManager) mkJoint2(f ast.ASTHandle, pos, reg uint32) *smtMAMJoint2 {
	return &smtMAMJoint2{decl: f, argPos: pos, reg: reg}
}

func (c *smtMAMCodeTreeManager) mkCompare(reg1, reg2 uint32) *smtMAMInstr {
	return &smtMAMInstr{op: mamCOMPARE, reg1: reg1, reg2: reg2}
}

func (c *smtMAMCodeTreeManager) mkCheck(reg uint32, n *SMTEnode) *smtMAMInstr {
	return &smtMAMInstr{op: mamCHECK, reg: reg, enode: n}
}

func (c *smtMAMCodeTreeManager) mkFilter(op smtMAMOpcode, reg uint32, s approx.UintSet) *smtMAMInstr {
	return &smtMAMInstr{op: op, reg: reg, lblSet: s}
}

func (c *smtMAMCodeTreeManager) mkGetEnode(reg uint32, n *SMTEnode) *smtMAMInstr {
	return &smtMAMInstr{op: mamGETENODE, oreg: reg, enode: n}
}

func (c *smtMAMCodeTreeManager) mkChoose(alt *smtMAMInstr) *smtMAMInstr {
	return &smtMAMInstr{op: mamCHOOSE, alt: alt}
}

func (c *smtMAMCodeTreeManager) mkNoop() *smtMAMInstr { return &smtMAMInstr{op: mamNOOP} }

func (c *smtMAMCodeTreeManager) mkBind(lbl ast.ASTHandle, numArgs, ireg, oreg uint32) *smtMAMInstr {
	return &smtMAMInstr{op: mamCountedOp(mamBIND1, mamBINDN, numArgs), label: lbl, numArgs: numArgs, ireg: ireg, oreg: oreg}
}

func (c *smtMAMCodeTreeManager) mkGetCgr(lbl ast.ASTHandle, declID uint32, oreg uint32, iregs []uint32) *smtMAMInstr {
	r := &smtMAMInstr{op: mamCountedOp(mamGETCGR1, mamGETCGRN, uint32(len(iregs))), label: lbl, oreg: oreg, numArgs: uint32(len(iregs)), iregs: append([]uint32(nil), iregs...)}
	r.lblSet.Insert(uint32(c.hasher.hash(declID)))
	return r
}

func (c *smtMAMCodeTreeManager) mkIsCgr(lbl ast.ASTHandle, ireg uint32, iregs []uint32) *smtMAMInstr {
	return &smtMAMInstr{op: mamISCGR, label: lbl, ireg: ireg, numArgs: uint32(len(iregs)), iregs: append([]uint32(nil), iregs...)}
}

func (c *smtMAMCodeTreeManager) mkYield(qa, pat ast.ASTHandle, bindings []uint32) *smtMAMInstr {
	return &smtMAMInstr{op: mamCountedOp(mamYIELD1, mamYIELDN, uint32(len(bindings))), qa: qa, pat: pat, binding: append([]uint32(nil), bindings...)}
}

func (c *smtMAMCodeTreeManager) mkCont(lbl ast.ASTHandle, numArgs, oreg uint32, s approx.UintSet, joints []smtMAMJoint) *smtMAMInstr {
	return &smtMAMInstr{op: mamCONTINUE, label: lbl, numArgs: numArgs, oreg: oreg, lblSet: s, joints: append([]smtMAMJoint(nil), joints...)}
}

// setNext is set_next: a value trail on instr->m_next.
func (c *smtMAMCodeTreeManager) setNext(instr, next *smtMAMInstr) {
	old := instr.next
	c.trail.push(func() { instr.next = old })
	instr.next = next
}

func (c *smtMAMCodeTreeManager) saveNumRegs(t *smtMAMCodeTree) {
	old := t.numRegs
	c.trail.push(func() { t.numRegs = old })
}

func (c *smtMAMCodeTreeManager) saveNumChoices(t *smtMAMCodeTree) {
	old := t.numChoices
	c.trail.push(func() { t.numChoices = old })
}

func (c *smtMAMCodeTreeManager) insertNewLblHash(instr *smtMAMInstr, h uint32) {
	old := instr.lblSet
	c.trail.push(func() { instr.lblSet = old })
	instr.lblSet.Insert(h)
}
