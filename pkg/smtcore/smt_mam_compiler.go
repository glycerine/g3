package smtcore

// Source: src/smt/mam.cpp compiler (lines 784-1807): patterns to code trees.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/bitset"
)

type smtMAMCheckMark uint8

const (
	mamNotChecked smtMAMCheckMark = iota
	mamCheckSet
	mamCheckSingleton
)

const (
	mamFindBestChildThreshold = 64
	mamSimpleSeqThreshold     = 4
)

// smtMAMCompiler is mam.cpp's compiler.
type smtMAMCompiler struct {
	state              *SMTInternalizerState
	registry           *SMTQuantifierRegistry
	ct                 *smtMAMCodeTreeManager
	hasher             *smtMAMLabelHasher
	useFilters         bool
	registers          []ast.ASTHandle
	todo               []uint32
	aux                []uint32
	vars               []int
	qa                 ast.ASTHandle
	mp                 ast.ASTHandle
	tree               *smtMAMCodeTree
	numChoices         uint32
	isTmpTree          bool
	mpAlreadyProcessed []bool
	matchedExprs       map[ast.ASTHandle]uint32
	mark               []smtMAMCheckMark
	toReset            []uint32
	compatible         []*smtMAMInstr
	incompatible       []*smtMAMInstr
	seq                []*smtMAMInstr
	err                error // first failure; patterns are small, so the
	// recursive helpers record it and unwind
}

func newSMTMAMCompiler(state *SMTInternalizerState, registry *SMTQuantifierRegistry, ct *smtMAMCodeTreeManager, hasher *smtMAMLabelHasher, useFilters bool) *smtMAMCompiler {
	return &smtMAMCompiler{state: state, registry: registry, ct: ct, hasher: hasher, useFilters: useFilters, matchedExprs: make(map[ast.ASTHandle]uint32)}
}

func (c *smtMAMCompiler) fail(err error) {
	if err != nil && c.err == nil {
		c.err = err
	}
}

// ---- AST helpers (errors are recorded in c.err) ----

func (c *smtMAMCompiler) isVar(e ast.ASTHandle) bool {
	k, err := c.state.manager.GetKind(e)
	c.fail(err)
	return err == nil && k == ast.ASTVar
}

func (c *smtMAMCompiler) isApp(e ast.ASTHandle) bool {
	k, err := c.state.manager.GetKind(e)
	c.fail(err)
	return err == nil && k == ast.ASTApp
}

func (c *smtMAMCompiler) isQuantifier(e ast.ASTHandle) bool {
	k, err := c.state.manager.GetKind(e)
	c.fail(err)
	return err == nil && k == ast.ASTQuantifier
}

func (c *smtMAMCompiler) isGround(e ast.ASTHandle) bool {
	g, err := c.state.manager.IsGround(e)
	c.fail(err)
	return err == nil && g
}

func (c *smtMAMCompiler) varIdx(e ast.ASTHandle) uint32 {
	i, err := c.state.manager.GetVarIndex(e)
	c.fail(err)
	return i
}

func (c *smtMAMCompiler) decl(e ast.ASTHandle) ast.ASTHandle {
	d, err := c.state.manager.GetDecl(e)
	c.fail(err)
	return d
}

func (c *smtMAMCompiler) declID(d ast.ASTHandle) uint32 {
	id, err := c.state.manager.GetASTID(d)
	c.fail(err)
	return id
}

func (c *smtMAMCompiler) numArgs(e ast.ASTHandle) uint32 {
	n, err := c.state.manager.GetNumArgs(e)
	c.fail(err)
	return n
}

func (c *smtMAMCompiler) arg(e ast.ASTHandle, i uint32) ast.ASTHandle {
	a, err := c.state.manager.GetArg(e, i)
	c.fail(err)
	return a
}

func (c *smtMAMCompiler) lblHash(d ast.ASTHandle) uint32 {
	return uint32(c.hasher.hash(c.declID(d)))
}

// mkEnode is mam.cpp's mk_enode(ctx, qa, n): internalize n at qa's
// generation.
func (c *smtMAMCompiler) mkEnode(n ast.ASTHandle) *SMTEnode {
	if c.err != nil {
		return nil
	}
	return smtMAMMkEnode(c.state, c.registry, c.qa, n, &c.err)
}

func smtMAMMkEnode(s *SMTInternalizerState, registry *SMTQuantifierRegistry, qa, n ast.ASTHandle, errp *error) *SMTEnode {
	generation, err := registry.Generation(qa)
	if err != nil {
		*errp = err
		return nil
	}
	previous := s.generation
	s.generation = generation
	err = s.Internalize(n, false)
	s.generation = previous
	if err != nil {
		*errp = err
		return nil
	}
	e := s.context.enodes.get(n)
	if e == nil {
		*errp = ast.ErrASTDomain
	}
	return e
}

// ---- register bookkeeping ----

func (c *smtMAMCompiler) setRegister(reg uint32, p ast.ASTHandle) {
	for uint32(len(c.registers)) <= reg {
		c.registers = append(c.registers, ast.ASTHandle{})
	}
	c.registers[reg] = p
}

func (c *smtMAMCompiler) register(reg uint32) ast.ASTHandle {
	if int(reg) < len(c.registers) {
		return c.registers[reg]
	}
	return ast.ASTHandle{}
}

func (c *smtMAMCompiler) getCheckMark(reg uint32) smtMAMCheckMark {
	if int(reg) < len(c.mark) {
		return c.mark[reg]
	}
	return mamNotChecked
}

func (c *smtMAMCompiler) setCheckMark(reg uint32, m smtMAMCheckMark) {
	for uint32(len(c.mark)) <= reg {
		c.mark = append(c.mark, mamNotChecked)
	}
	c.mark[reg] = m
}

func mamEraseReg(v []uint32, r uint32) []uint32 {
	for i, x := range v {
		if x == r {
			return append(v[:i], v[i+1:]...)
		}
	}
	return v
}

func (c *smtMAMCompiler) init(t *smtMAMCodeTree, qa, mp ast.ASTHandle, firstIdx uint32) {
	c.tree, c.qa, c.mp = t, qa, mp
	c.numChoices = 0
	c.todo = c.todo[:0]
	for i := range c.registers {
		c.registers[i] = ast.ASTHandle{}
	}
	p := c.arg(mp, firstIdx)
	n := c.numArgs(p)
	for i := uint32(0); i < n; i++ {
		c.setRegister(i+1, c.arg(p, i))
		c.todo = append(c.todo, i+1)
	}
	numDecls, err := c.state.manager.GetNumDeclarations(qa)
	c.fail(err)
	for uint32(len(c.vars)) < numDecls {
		c.vars = append(c.vars, -1)
	}
	for j := uint32(0); j < numDecls; j++ {
		c.vars[j] = -1
	}
}

func (c *smtMAMCompiler) allArgsAreBoundVars(n ast.ASTHandle) bool {
	k := c.numArgs(n)
	for i := uint32(0); i < k; i++ {
		a := c.arg(n, i)
		if !c.isVar(a) || c.vars[c.varIdx(a)] == -1 {
			return false
		}
	}
	return true
}

func (c *smtMAMCompiler) getStatsCore(n ast.ASTHandle, sz, unbound *uint32) {
	*sz++
	if c.isGround(n) {
		return
	}
	k := c.numArgs(n)
	for i := uint32(0); i < k; i++ {
		a := c.arg(n, i)
		if c.isVar(a) {
			*sz++
			if c.vars[c.varIdx(a)] == -1 {
				*unbound++
			}
		} else if c.isApp(a) {
			c.getStatsCore(a, sz, unbound)
		}
	}
}

func (c *smtMAMCompiler) getStats(n ast.ASTHandle) (sz, unbound uint32) {
	c.getStatsCore(n, &sz, &unbound)
	return sz, unbound
}

// lineariseCore is linearise_core.
func (c *smtMAMCompiler) lineariseCore() {
	c.aux = c.aux[:0]
	var firstApp ast.ASTHandle
	var firstAppReg, firstAppSz, firstAppUnbound uint32
	for _, reg := range c.todo {
		p := c.registers[reg]
		if c.isVar(p) {
			v := c.varIdx(p)
			if c.vars[v] != -1 {
				c.seq = append(c.seq, c.ct.mkCompare(uint32(c.vars[v]), reg))
			} else {
				c.vars[v] = int(reg)
			}
			continue
		}
		if c.isGround(p) {
			// ground applications are viewed as constants and eagerly
			// converted into enodes.
			e := c.mkEnode(p)
			c.seq = append(c.seq, c.ct.mkCheck(reg, e))
			c.setCheckMark(reg, mamNotChecked)
			continue
		}
		if matched, ok := c.matchedExprs[p]; ok && reg != matched {
			c.seq = append(c.seq, c.ct.mkCompare(matched, reg))
			c.setCheckMark(reg, mamNotChecked)
			continue
		}
		c.matchedExprs[p] = reg
		if c.useFilters && c.getCheckMark(reg) != mamCheckSingleton {
			s := bitset.NewApproxSet(c.lblHash(c.decl(p)))
			c.seq = append(c.seq, c.ct.mkFilter(mamFILTER, reg, s))
			c.setCheckMark(reg, mamCheckSingleton)
		}
		if !firstApp.IsNull() {
			// Try to select the best first_app.
			if firstAppUnbound == 0 {
				c.aux = append(c.aux, reg)
			} else {
				sz, unbound := c.getStats(p)
				if unbound == 0 || sz > firstAppSz || (sz == firstAppSz && unbound < firstAppUnbound) {
					c.aux = append(c.aux, firstAppReg)
					firstApp, firstAppReg, firstAppSz, firstAppUnbound = p, reg, sz, unbound
				} else {
					c.aux = append(c.aux, reg)
				}
			}
		} else {
			firstApp, firstAppReg = p, reg
			firstAppSz, firstAppUnbound = c.getStats(firstApp)
		}
	}
	if !firstApp.IsNull() {
		lbl := c.decl(firstApp)
		numArgs := c.numArgs(firstApp)
		if c.allArgsAreBoundVars(firstApp) {
			// use IS_CGR instead of BIND
			iregs := make([]uint32, numArgs)
			for i := uint32(0); i < numArgs; i++ {
				iregs[i] = uint32(c.vars[c.varIdx(c.arg(firstApp, i))])
			}
			c.seq = append(c.seq, c.ct.mkIsCgr(lbl, firstAppReg, iregs))
		} else {
			oreg := c.tree.numRegs
			c.tree.numRegs += numArgs
			for j := uint32(0); j < numArgs; j++ {
				c.setRegister(oreg+j, c.arg(firstApp, j))
				c.aux = append(c.aux, oreg+j)
			}
			c.seq = append(c.seq, c.ct.mkBind(lbl, numArgs, firstAppReg, oreg))
			c.numChoices++
		}
		c.setCheckMark(firstAppReg, mamNotChecked)
	}
	c.todo, c.aux = c.aux, c.todo
}

func (c *smtMAMCompiler) getNumBoundVarsCore(n ast.ASTHandle, hasUnbound *bool) uint32 {
	if c.isGround(n) {
		return 0
	}
	var r uint32
	k := c.numArgs(n)
	for i := uint32(0); i < k; i++ {
		a := c.arg(n, i)
		if c.isVar(a) {
			if c.vars[c.varIdx(a)] != -1 {
				r++
			} else {
				*hasUnbound = true
			}
		} else if c.isApp(a) {
			r += c.getNumBoundVarsCore(a, hasUnbound)
		}
	}
	return r
}

// genMPFilter is gen_mp_filter.
func (c *smtMAMCompiler) genMPFilter(n ast.ASTHandle) uint32 {
	if c.isGround(n) {
		oreg := c.tree.numRegs
		c.tree.numRegs++
		e := c.mkEnode(n)
		c.seq = append(c.seq, c.ct.mkGetEnode(oreg, e))
		return oreg
	}
	k := c.numArgs(n)
	iregs := make([]uint32, 0, k)
	for i := uint32(0); i < k; i++ {
		a := c.arg(n, i)
		if c.isVar(a) {
			iregs = append(iregs, uint32(c.vars[c.varIdx(a)]))
		} else {
			iregs = append(iregs, c.genMPFilter(a))
		}
	}
	oreg := c.tree.numRegs
	c.tree.numRegs++
	d := c.decl(n)
	c.seq = append(c.seq, c.ct.mkGetCgr(d, c.declID(d), oreg, iregs))
	return oreg
}

// lineariseMultiPattern is linearise_multi_pattern.
func (c *smtMAMCompiler) lineariseMultiPattern(firstIdx uint32) {
	numArgs := c.numArgs(c.mp)
	for i := uint32(1); i < numArgs; i++ {
		// select the pattern with the biggest number of bound variables
		var best ast.ASTHandle
		var bestNumBvars, bestJ uint32
		foundBoundedMP := false
		for j := uint32(0); j < numArgs; j++ {
			if c.mpAlreadyProcessed[j] {
				continue
			}
			p := c.arg(c.mp, j)
			hasUnbound := false
			numBvars := c.getNumBoundVarsCore(p, &hasUnbound)
			if !hasUnbound {
				best, bestJ, foundBoundedMP = p, j, true
				break
			}
			if best.IsNull() || numBvars > bestNumBvars {
				best, bestNumBvars, bestJ = p, numBvars, j
			}
		}
		c.mpAlreadyProcessed[bestJ] = true
		p := best
		lbl := c.decl(p)
		pArgs := c.numArgs(p)
		var s bitset.ApproxSet
		if c.useFilters {
			s.Insert(c.lblHash(lbl))
		}
		if foundBoundedMP {
			c.genMPFilter(p)
			continue
		}
		// USE CONTINUE
		oreg := c.tree.numRegs
		c.tree.numRegs += pArgs
		joints := make([]smtMAMJoint, 0, pArgs)
		hasDepth1Joint := false
		for j := uint32(0); j < pArgs; j++ {
			curr := c.arg(p, j)
			c.setRegister(oreg+j, curr)
			c.todo = append(c.todo, oreg+j)
			if (c.isVar(curr) && c.vars[c.varIdx(curr)] >= 0) || (c.isApp(curr) && c.isGround(curr)) {
				hasDepth1Joint = true
			}
		}
		if hasDepth1Joint {
			for j := uint32(0); j < pArgs; j++ {
				curr := c.arg(p, j)
				if c.isVar(curr) {
					if v := c.vars[c.varIdx(curr)]; v >= 0 {
						joints = append(joints, smtMAMJoint{tag: mamVarTag, reg: uint32(v)})
					} else {
						joints = append(joints, smtMAMJoint{tag: mamNullTag})
					}
					continue
				}
				if c.isGround(curr) {
					joints = append(joints, smtMAMJoint{tag: mamGroundTermTag, enode: c.mkEnode(curr)})
					continue
				}
				joints = append(joints, smtMAMJoint{tag: mamNullTag})
			}
		} else {
			// Only try depth-2 joints if there is no depth-1 joint.
			for j := uint32(0); j < pArgs; j++ {
				curr := c.arg(p, j)
				if !c.isApp(curr) {
					joints = append(joints, smtMAMJoint{tag: mamNullTag})
					continue
				}
				n2 := c.numArgs(curr)
				k := uint32(0)
				for ; k < n2; k++ {
					a := c.arg(curr, k)
					if !c.isVar(a) {
						continue
					}
					v := c.vars[c.varIdx(a)]
					if v < 0 {
						continue
					}
					joints = append(joints, smtMAMJoint{tag: mamNestedVarTag, joint2: c.ct.mkJoint2(c.decl(curr), k, uint32(v))})
					break
				}
				if k == n2 {
					joints = append(joints, smtMAMJoint{tag: mamNullTag})
				}
			}
		}
		c.seq = append(c.seq, c.ct.mkCont(lbl, pArgs, oreg, s, joints))
		c.numChoices++
		for len(c.todo) != 0 && c.err == nil {
			c.lineariseCore()
		}
	}
}

// linearise produces the operations for the registers in m_todo.
func (c *smtMAMCompiler) linearise(head *smtMAMInstr, firstIdx uint32) {
	c.seq = c.seq[:0]
	clear(c.matchedExprs)
	for len(c.todo) != 0 && c.err == nil {
		c.lineariseCore()
	}
	if c.numArgs(c.mp) > 1 {
		c.mpAlreadyProcessed = make([]bool, c.numArgs(c.mp))
		c.mpAlreadyProcessed[firstIdx] = true
		c.lineariseMultiPattern(firstIdx)
	}
	numDecls, err := c.state.manager.GetNumDeclarations(c.qa)
	c.fail(err)
	bindings := make([]uint32, numDecls)
	for i := range bindings {
		bindings[i] = uint32(c.vars[i])
	}
	c.seq = append(c.seq, c.ct.mkYield(c.qa, c.mp, bindings))
	for _, curr := range c.seq {
		head.next = curr
		head = curr
	}
}

func (c *smtMAMCompiler) setNext(instr, next *smtMAMInstr) {
	if c.isTmpTree {
		instr.next = next
	} else {
		c.ct.setNext(instr, next)
	}
}

func (c *smtMAMCompiler) findBestChild(firstChild *smtMAMInstr) *smtMAMInstr {
	numTooSimple := 0
	var bestChild *smtMAMInstr
	var maxCompatibility uint32
	for curr := firstChild; curr != nil; curr = curr.alt {
		compat, simple := c.getCompatibilityMeasure(curr)
		if simple {
			numTooSimple++
			if numTooSimple > mamFindBestChildThreshold {
				return nil
			}
		}
		if compat > maxCompatibility {
			bestChild, maxCompatibility = curr, compat
		}
	}
	return bestChild
}

func (c *smtMAMCompiler) isCompatibleBind(instr *smtMAMInstr) bool {
	n := c.register(instr.ireg)
	return !n.IsNull() && c.isApp(n) && !c.isGround(n) && c.decl(n) == instr.label && c.numArgs(n) == instr.numArgs
}

func (c *smtMAMCompiler) isCompatibleCompare(instr *smtMAMInstr) bool {
	r1 := c.register(instr.reg1)
	return !r1.IsNull() && r1 == c.register(instr.reg2)
}

func (c *smtMAMCompiler) isCompatibleCheck(instr *smtMAMInstr) bool {
	r := c.register(instr.reg)
	if r.IsNull() || !c.isApp(r) || !c.isGround(r) {
		return false
	}
	nPrime := c.mkEnode(r)
	return nPrime != nil && instr.enode.root == nPrime.root
}

// getPatLblHash is get_pat_lbl_hash.
func (c *smtMAMCompiler) getPatLblHash(reg uint32) uint32 {
	p := c.register(reg)
	if c.isGround(p) {
		e := c.mkEnode(p)
		if e == nil {
			return 0
		}
		if !e.HasLabelHash() {
			c.fail(e.SetLabelHash(&c.state.enodeTrail))
		}
		h, err := e.LabelHash()
		c.fail(err)
		return uint32(h)
	}
	return c.lblHash(c.decl(p))
}

func (c *smtMAMCompiler) isSemiCompatibleCheck(instr *smtMAMInstr) bool {
	r := c.register(instr.reg)
	if r.IsNull() || c.getCheckMark(instr.reg) != mamNotChecked || !c.isGround(r) {
		return false
	}
	h, err := instr.enode.LabelHash()
	c.fail(err)
	return c.getPatLblHash(instr.reg) == uint32(h)
}

func (c *smtMAMCompiler) isCompatibleFilter(instr *smtMAMInstr) bool {
	r := c.register(instr.reg)
	if !r.IsNull() && c.isApp(r) && !c.isGround(r) {
		return instr.lblSet.MayContain(c.getPatLblHash(instr.reg))
	}
	return false
}

func (c *smtMAMCompiler) isCFilterCompatible(instr *smtMAMInstr) bool {
	r := c.register(instr.reg)
	if !r.IsNull() && c.isGround(r) {
		return instr.lblSet.MayContain(c.getPatLblHash(instr.reg))
	}
	return false
}

func (c *smtMAMCompiler) isSemiCompatibleFilter(instr *smtMAMInstr) bool {
	r := c.register(instr.reg)
	return !r.IsNull() && c.getCheckMark(instr.reg) == mamNotChecked && c.isApp(r) && !c.isGround(r)
}

func mamIsBind(op smtMAMOpcode) bool { return op >= mamBIND1 && op <= mamBINDN }

// getCompatibilityMeasure is get_compatibility_measure.
func (c *smtMAMCompiler) getCompatibilityMeasure(child *smtMAMInstr) (weight uint32, simple bool) {
	simple = true
	c.toReset = c.toReset[:0]
	numInstr := 0
	curr := child.next
	for curr != nil && curr.op != mamCHOOSE && curr.op != mamNOOP {
		numInstr++
		switch {
		case mamIsBind(curr.op):
			if c.isCompatibleBind(curr) {
				weight += 4
				n := c.register(curr.ireg)
				for i := uint32(0); i < curr.numArgs; i++ {
					c.setRegister(curr.oreg+i, c.arg(n, i))
					c.toReset = append(c.toReset, curr.oreg+i)
				}
			}
		case curr.op == mamCOMPARE:
			if c.isCompatibleCompare(curr) {
				weight += 2
			}
		case curr.op == mamCHECK:
			if c.isCompatibleCheck(curr) {
				weight += 2
			} else if c.useFilters && c.isSemiCompatibleCheck(curr) {
				weight++
			}
		case curr.op == mamCFILTER:
			if c.isCFilterCompatible(curr) {
				weight += 2
			}
		case curr.op == mamFILTER:
			if c.isCompatibleFilter(curr) {
				weight += 2
			} else if c.isSemiCompatibleFilter(curr) {
				weight++
			}
		}
		curr = curr.next
	}
	if numInstr > mamSimpleSeqThreshold || (curr != nil && curr.op == mamCHOOSE) {
		simple = false
	}
	for _, r := range c.toReset {
		c.registers[r] = ast.ASTHandle{}
	}
	return weight, simple
}

func mamApproxSize(s bitset.ApproxSet) int {
	n := 0
	for v := uint32(0); v < 64; v++ {
		if s.MayContain(v) {
			n++
		}
	}
	return n
}

// insertAt is compiler::insert(head, first_mp_idx).
func (c *smtMAMCompiler) insertAt(head *smtMAMInstr, firstMPIdx uint32) {
	for c.err == nil {
		c.compatible = c.compatible[:0]
		c.incompatible = c.incompatible[:0]
		curr := head.next
		last := head
		for curr != nil && curr.op != mamCHOOSE && curr.op != mamNOOP {
			switch {
			case mamIsBind(curr.op):
				if c.isCompatibleBind(curr) {
					ireg := curr.ireg
					c.todo = mamEraseReg(c.todo, ireg)
					c.setCheckMark(ireg, mamNotChecked)
					c.compatible = append(c.compatible, curr)
					app := c.register(ireg)
					for i := uint32(0); i < curr.numArgs; i++ {
						c.setRegister(curr.oreg+i, c.arg(app, i))
						c.todo = append(c.todo, curr.oreg+i)
					}
				} else {
					c.incompatible = append(c.incompatible, curr)
				}
			case curr.op == mamCHECK:
				if c.isCompatibleCheck(curr) {
					c.todo = mamEraseReg(c.todo, curr.reg)
					c.setCheckMark(curr.reg, mamNotChecked)
					c.compatible = append(c.compatible, curr)
				} else if c.useFilters && c.isSemiCompatibleCheck(curr) {
					n1 := curr.enode
					if !n1.HasLabelHash() {
						c.fail(n1.SetLabelHash(&c.state.enodeTrail))
					}
					h1, err := n1.LabelHash()
					c.fail(err)
					h2 := c.getPatLblHash(curr.reg)
					s := bitset.NewApproxSet(uint32(h1))
					s.Insert(h2)
					newInstr := c.ct.mkFilter(mamCFILTER, curr.reg, s)
					c.setCheckMark(curr.reg, mamCheckSet)
					c.compatible = append(c.compatible, newInstr)
					c.incompatible = append(c.incompatible, curr)
				} else {
					c.incompatible = append(c.incompatible, curr)
				}
			case curr.op == mamCOMPARE:
				if c.isCompatibleCompare(curr) {
					c.todo = mamEraseReg(c.todo, curr.reg2)
					if r1 := c.register(curr.reg1); c.isVar(r1) {
						c.todo = mamEraseReg(c.todo, curr.reg1)
						if v := c.varIdx(r1); c.vars[v] == -1 {
							c.vars[v] = int(curr.reg1)
						}
					}
					c.compatible = append(c.compatible, curr)
				} else {
					c.incompatible = append(c.incompatible, curr)
				}
			case curr.op == mamCFILTER:
				if c.isCFilterCompatible(curr) {
					c.setCheckMark(curr.reg, mamCheckSingleton)
					c.compatible = append(c.compatible, curr)
				} else {
					c.incompatible = append(c.incompatible, curr)
				}
			case curr.op == mamFILTER:
				if c.isCompatibleFilter(curr) {
					if mamApproxSize(curr.lblSet) == 1 {
						c.setCheckMark(curr.reg, mamCheckSingleton)
					} else {
						c.setCheckMark(curr.reg, mamCheckSet)
					}
					c.compatible = append(c.compatible, curr)
				} else if c.isSemiCompatibleFilter(curr) {
					h := c.getPatLblHash(curr.reg)
					c.setCheckMark(curr.reg, mamCheckSet)
					if mamApproxSize(curr.lblSet) > 1 {
						c.ct.insertNewLblHash(curr, h)
						c.compatible = append(c.compatible, curr)
					} else {
						newS := curr.lblSet
						newS.Insert(h)
						c.compatible = append(c.compatible, c.ct.mkFilter(mamFILTER, curr.reg, newS))
						c.incompatible = append(c.incompatible, curr)
					}
				} else {
					c.incompatible = append(c.incompatible, curr)
				}
			default:
				c.incompatible = append(c.incompatible, curr)
			}
			last = curr
			curr = curr.next
		}
		if len(c.incompatible) == 0 {
			// sequence starting at head is fully compatible
			firstChild := curr
			bestChild := c.findBestChild(firstChild)
			if bestChild == nil {
				newChild := c.ct.mkChoose(firstChild)
				c.numChoices++
				c.setNext(last, newChild)
				c.linearise(newChild, firstMPIdx)
				return
			}
			head = bestChild
			continue
		}
		// head -> c1 -> c2 -> c3 -> new_child_head1
		// new_child_head1: CHOOSE(new_child_head2) -> i1 -> i2 -> first_child_head
		// new_child_head2: NOOP -> linearise()
		firstChildHead := curr
		newChildHead2 := c.ct.mkNoop()
		newChildHead1 := c.ct.mkChoose(newChildHead2)
		c.numChoices++
		curr = head
		for _, instr := range c.compatible {
			c.setNext(curr, instr)
			curr = instr
		}
		c.setNext(curr, newChildHead1)
		curr = newChildHead1
		for _, instr := range c.incompatible {
			if curr == newChildHead1 {
				curr.next = instr // a new node needs no trail
			} else {
				c.setNext(curr, instr)
			}
			curr = instr
		}
		c.setNext(curr, firstChildHead)
		c.linearise(newChildHead2, firstMPIdx)
		return
	}
}

// mkTree is compiler::mk_tree.
func (c *smtMAMCompiler) mkTree(qa, mp ast.ASTHandle, firstIdx uint32, filterCandidates bool) (*smtMAMCodeTree, error) {
	c.err = nil
	p := c.arg(mp, firstIdx)
	r := c.ct.mkCodeTree(c.decl(p), c.numArgs(p), filterCandidates)
	c.init(r, qa, mp, firstIdx)
	c.linearise(r.root, firstIdx)
	r.numChoices = c.numChoices
	return r, c.err
}

// insert is compiler::insert(tree, qa, mp, first_idx, is_tmp_tree).
func (c *smtMAMCompiler) insert(tree *smtMAMCodeTree, qa, mp ast.ASTHandle, firstIdx uint32, isTmpTree bool) error {
	c.err = nil
	if tree.numArgs != c.numArgs(c.arg(mp, firstIdx)) {
		// n-ary + and *: ignore a pattern without the expected arity.
		return c.err
	}
	c.isTmpTree = isTmpTree
	if !isTmpTree {
		c.ct.saveNumRegs(tree)
	}
	c.init(tree, qa, mp, firstIdx)
	c.numChoices = tree.numChoices
	c.insertAt(tree.root, firstIdx)
	if c.numChoices > tree.numChoices {
		if !isTmpTree {
			c.ct.saveNumChoices(tree)
		}
		tree.numChoices = c.numChoices
	}
	return c.err
}
