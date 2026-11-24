package smtcore

// Source: src/smt/mam.cpp backtrack_point and interpreter (lines 1832-2815).

import (
	"github.com/glycerine/g3/pkg/ast"
)

// smtMAMOnMatch is mam::on_match(qa, pat, num_bindings, bindings,
// max_generation, used_enodes).
type smtMAMOnMatch func(qa, pat ast.ASTHandle, bindings []*SMTEnode, maxGeneration uint32, used []*SMTEnode) error

// smtMAMBacktrackPoint is backtrack_point. For CONTINUE, candidates/it
// replace the [m_it, m_end) range (decl2enodes or a depth-1/2 vector).
type smtMAMBacktrackPoint struct {
	instr             *smtMAMInstr
	oldMaxGeneration  uint32
	oldUsedEnodesSize int
	curr              *SMTEnode
	candidates        []*SMTEnode
	it                int
	toRecycle         *[]*SMTEnode // m_to_recycle: the pooled depth vector, if any
}

// smtMAMInterpreter is mam.cpp's interpreter.
type smtMAMInterpreter struct {
	state      *SMTInternalizerState
	onMatch    smtMAMOnMatch
	useFilters bool
	registers  []*SMTEnode
	bindings   []*SMTEnode
	args       []*SMTEnode
	backtrack  []smtMAMBacktrackPoint
	top        int
	pool       smtMAMEnodeVectorPool // m_pool
	pc         *smtMAMInstr

	maxGeneration     uint32
	usedEnodes        []*SMTEnode
	patternInstances  []*SMTEnode
	minTopGeneration  []uint32
	maxTopGeneration  []uint32
	traceEnabled      func() bool
	cancelled         func() bool
	resourcesExceeded func() bool
	err               error
}

func newSMTMAMInterpreter(state *SMTInternalizerState, onMatch smtMAMOnMatch, useFilters bool) *smtMAMInterpreter {
	return &smtMAMInterpreter{state: state, onMatch: onMatch, useFilters: useFilters, args: make([]*SMTEnode, 16)}
}

func (it *smtMAMInterpreter) hasTraceStream() bool {
	return it.state.manager.TraceStream != nil
}

func (it *smtMAMInterpreter) relevant(n *SMTEnode) bool { return it.state.isRelevant(n.owner) }

func (it *smtMAMInterpreter) updateMaxGeneration(n *SMTEnode) {
	if n.generation > it.maxGeneration {
		it.maxGeneration = n.generation
	}
	if it.hasTraceStream() {
		it.usedEnodes = append(it.usedEnodes, n)
	}
}

func mamIsCgr(n *SMTEnode) bool { return n.congruence == n }

func (it *smtMAMInterpreter) getFirstFApp(lbl ast.ASTHandle, numArgs uint32, curr *SMTEnode) *SMTEnode {
	first := curr
	for {
		if curr.declaration == lbl && mamIsCgr(curr) && uint32(len(curr.arguments)) == numArgs {
			it.updateMaxGeneration(curr)
			return curr
		}
		curr = curr.next
		if curr == first {
			return nil
		}
	}
}

func (it *smtMAMInterpreter) getNextFApp(lbl ast.ASTHandle, numArgs uint32, first, curr *SMTEnode) *SMTEnode {
	for curr = curr.next; curr != first; curr = curr.next {
		if curr.declaration == lbl && mamIsCgr(curr) && uint32(len(curr.arguments)) == numArgs {
			it.updateMaxGeneration(curr)
			return curr
		}
	}
	return nil
}

func (it *smtMAMInterpreter) execIsCgr(pc *smtMAMInstr) bool {
	n := it.registers[pc.ireg]
	f := pc.label
	k := int(pc.numArgs)
	for len(it.args) < k+1 {
		it.args = append(it.args, nil)
	}
	for i := 0; i < k; i++ {
		it.args[i] = it.registers[pc.iregs[i]].root
	}
	first := n
	for {
		if n.declaration == f && len(n.arguments) >= k {
			i := 0
			for ; i < k; i++ {
				if n.arguments[i].root != it.args[i] {
					break
				}
			}
			if i == k {
				it.updateMaxGeneration(n)
				return true
			}
		}
		n = n.next
		if n == first {
			return false
		}
	}
}

// mkDepth1Vector: the relevant f-parents of n whose i-th argument is n.
func (it *smtMAMInterpreter) mkDepth1Vector(n *SMTEnode, f ast.ASTHandle, i uint32) *[]*SMTEnode {
	v := it.pool.mk()
	n = n.root
	for _, p := range n.sourceParents {
		if p.declaration == f && i < uint32(len(p.arguments)) && it.relevant(p) && mamIsCgr(p) && p.arguments[i].root == n {
			*v = append(*v, p)
		}
	}
	return v
}

// mkDepth2Vector: the relevant f-parents of each p in joint2.
func (it *smtMAMInterpreter) mkDepth2Vector(j2 *smtMAMJoint2, f ast.ASTHandle, i uint32) *[]*SMTEnode {
	n := it.registers[j2.reg].root
	if len(n.sourceParents) == 0 {
		return nil
	}
	numArgs := len(n.arguments)
	v := it.pool.mk()
	for _, p := range n.sourceParents {
		if p.declaration == j2.decl && it.relevant(p) && mamIsCgr(p) && p.arguments[j2.argPos].root == n {
			p = p.root
			for _, p2 := range p.sourceParents {
				if p2.declaration == f && numArgs == len(n.arguments) && it.relevant(p2) && mamIsCgr(p2) && p2.arguments[i].root == p {
					*v = append(*v, p2)
				}
			}
		}
	}
	return v
}

// enodesOf is context::begin_enodes_of(lbl)..end_enodes_of(lbl):
// m_decl2enodes keeps the non-equality enodes with arguments.
func (it *smtMAMInterpreter) enodesOf(lbl ast.ASTHandle) []*SMTEnode {
	return it.state.context.matchingDeclarations()[lbl]
}

func (it *smtMAMInterpreter) initContinue(c *smtMAMInstr, expectedNumArgs uint32) *SMTEnode {
	lbl := c.label
	// quick filter: does any joint point have zero parents?
	for i := uint32(0); i < c.numArgs; i++ {
		var n *SMTEnode
		switch j := c.joints[i]; j.tag {
		case mamNullTag, mamNestedVarTag:
			continue
		case mamGroundTermTag:
			n = j.enode
		case mamVarTag:
			n = it.registers[j.reg]
		}
		r := n.root
		if it.useFilters && r.propagatedLabels.EmptyIntersection(c.lblSet) {
			return nil
		}
		if len(r.sourceParents) == 0 {
			return nil
		}
	}
	all := it.enodesOf(lbl) // min_sz = get_num_enodes_of(lbl); no side effects
	minSz := len(all)
	// traverse each joint and select the best one.
	var best *[]*SMTEnode
	for i := uint32(0); i < c.numArgs; i++ {
		var curr *[]*SMTEnode
		switch j := c.joints[i]; j.tag {
		case mamGroundTermTag:
			curr = it.mkDepth1Vector(j.enode, lbl, i)
		case mamVarTag:
			curr = it.mkDepth1Vector(it.registers[j.reg], lbl, i)
		case mamNestedVarTag:
			curr = it.mkDepth2Vector(j.joint2, lbl, i)
		}
		if curr == nil {
			continue
		}
		if len(*curr) < minSz && (best == nil || len(*curr) < len(*best)) {
			if best != nil {
				it.pool.recycle(best)
			}
			best = curr
			if len(*best) == 0 {
				it.pool.recycle(best)
				return nil
			}
		} else {
			it.pool.recycle(curr)
		}
	}
	for len(it.backtrack) <= it.top {
		it.backtrack = append(it.backtrack, smtMAMBacktrackPoint{})
	}
	bp := &it.backtrack[it.top]
	bp.instr = c
	bp.oldMaxGeneration = it.maxGeneration
	bp.oldUsedEnodesSize = len(it.usedEnodes)
	if best != nil {
		bp.toRecycle = best
		bp.candidates = *best
	} else {
		bp.toRecycle = nil
		bp.candidates = all
	}
	// find an application with the right number of arguments
	for bp.it = 0; bp.it < len(bp.candidates); bp.it++ {
		curr := bp.candidates[bp.it]
		if uint32(len(curr.arguments)) == expectedNumArgs && it.relevant(curr) {
			break
		}
	}
	if bp.it == len(bp.candidates) {
		return nil
	}
	it.top++
	it.updateMaxGeneration(bp.candidates[bp.it])
	return bp.candidates[bp.it]
}

// recycleBacktrackVector is recycle_enode_vector(bp.m_to_recycle).
func (it *smtMAMInterpreter) recycleBacktrackVector(bp *smtMAMBacktrackPoint) {
	it.pool.recycle(bp.toRecycle)
	bp.toRecycle = nil
	bp.candidates = nil
}

// releaseFrames recycles the pooled vectors of the live backtrack frames
// when execution stops early (an on_match error or cancellation), so they
// return to m_pool instead of staying referenced by stale frames.
func (it *smtMAMInterpreter) releaseFrames() {
	for i := 0; i < it.top && i < len(it.backtrack); i++ {
		if bp := &it.backtrack[i]; bp.toRecycle != nil {
			it.recycleBacktrackVector(bp)
		} else {
			bp.candidates = nil
		}
	}
	it.top = 0
}

func (it *smtMAMInterpreter) init(t *smtMAMCodeTree) {
	for uint32(len(it.registers)) < t.numRegs {
		it.registers = append(it.registers, nil)
	}
	for uint32(len(it.bindings)) < t.numRegs {
		it.bindings = append(it.bindings, nil)
	}
	for uint32(len(it.backtrack)) < t.numChoices {
		it.backtrack = append(it.backtrack, smtMAMBacktrackPoint{})
	}
}

// execute is interpreter::execute.
func (it *smtMAMInterpreter) execute(t *smtMAMCodeTree) error {
	it.err = nil
	it.init(t)
	if t.filterCandidates {
		for _, app := range t.candidates {
			if !app.mark && mamIsCgr(app) {
				it.executeCore(t, app)
				app.mark = true
			}
		}
		for _, app := range t.candidates {
			app.mark = false
		}
	} else {
		for _, app := range t.candidates {
			if mamIsCgr(app) {
				it.executeCore(t, app)
			}
		}
	}
	return it.err
}

func (it *smtMAMInterpreter) takeErr() error {
	err := it.err
	it.err = nil
	return err
}

// getMinMaxTopGeneration is get_min_max_top_generation.
func (it *smtMAMInterpreter) getMinMaxTopGeneration() (lo, hi uint32) {
	if len(it.minTopGeneration) == 0 {
		lo = it.patternInstances[0].generation
		hi = lo
		it.minTopGeneration = append(it.minTopGeneration, lo)
		it.maxTopGeneration = append(it.maxTopGeneration, hi)
	} else {
		lo = it.minTopGeneration[len(it.minTopGeneration)-1]
		hi = it.maxTopGeneration[len(it.maxTopGeneration)-1]
	}
	for i := len(it.minTopGeneration); i < len(it.patternInstances); i++ {
		curr := it.patternInstances[i].generation
		if curr < lo {
			lo = curr
		}
		it.minTopGeneration = append(it.minTopGeneration, lo)
		if curr > hi {
			hi = curr
		}
		it.maxTopGeneration = append(it.maxTopGeneration, hi)
	}
	return lo, hi
}

func (it *smtMAMInterpreter) setRegister(reg uint32, n *SMTEnode) {
	for uint32(len(it.registers)) <= reg {
		it.registers = append(it.registers, nil)
	}
	it.registers[reg] = n
}

func (it *smtMAMInterpreter) pushBacktrack(bp smtMAMBacktrackPoint) {
	for len(it.backtrack) <= it.top {
		it.backtrack = append(it.backtrack, smtMAMBacktrackPoint{})
	}
	it.backtrack[it.top] = bp
	it.top++
}

// executeCore is interpreter::execute_core.
func (it *smtMAMInterpreter) executeCore(t *smtMAMCodeTree, n *SMTEnode) {
	if it.err != nil {
		return
	}
	sinceLastCheck := 0
	it.patternInstances = append(it.patternInstances[:0], n)
	it.minTopGeneration = it.minTopGeneration[:0]
	it.maxTopGeneration = it.maxTopGeneration[:0]
	it.maxGeneration = n.generation
	if it.hasTraceStream() {
		it.usedEnodes = append(it.usedEnodes[:0], n)
	}
	it.pc = t.root
	it.setRegister(0, n)
	it.top = 0

	for {
		// main_loop: run until an instruction asks to backtrack.
		ok := it.step()
		if it.err != nil {
			it.releaseFrames()
			return
		}
		if ok {
			continue
		}
		// backtrack:
		for {
			if it.top == 0 {
				return // no more alternatives
			}
			bp := &it.backtrack[it.top-1]
			it.maxGeneration = bp.oldMaxGeneration
			if it.hasTraceStream() && bp.oldUsedEnodesSize <= len(it.usedEnodes) {
				it.usedEnodes = it.usedEnodes[:bp.oldUsedEnodesSize]
			}
			sinceLastCheck++
			if sinceLastCheck > 101 {
				sinceLastCheck = 0
				if it.resourcesExceeded != nil && it.resourcesExceeded() {
					// Soft timeout: recycle the CONTINUE vectors on the stack.
					for ; it.top != 0; it.top-- {
						if b := &it.backtrack[it.top-1]; b.instr.op == mamCONTINUE && b.toRecycle != nil {
							it.recycleBacktrackVector(b)
						}
					}
					return
				}
			}
			resumed := false
			switch op := bp.instr.op; {
			case op == mamCHOOSE:
				it.pc = bp.instr.alt
				it.top--
				resumed = true
			case mamIsBind(op):
				b := bp.instr
				n1 := it.registers[b.ireg]
				app := it.getNextFApp(b.label, b.numArgs, n1, bp.curr)
				if app == nil {
					it.top--
					continue
				}
				bp.curr = app
				for i := uint32(0); i < b.numArgs; i++ {
					it.setRegister(b.oreg+i, app.arguments[i])
				}
				it.pc = b.next
				resumed = true
			case op == mamCONTINUE:
				c := bp.instr
				for bp.it++; bp.it < len(bp.candidates); bp.it++ {
					app := bp.candidates[bp.it]
					if uint32(len(app.arguments)) == c.numArgs && it.relevant(app) {
						if len(it.patternInstances) == len(it.maxTopGeneration) {
							it.maxTopGeneration = it.maxTopGeneration[:len(it.maxTopGeneration)-1]
							it.minTopGeneration = it.minTopGeneration[:len(it.minTopGeneration)-1]
						}
						it.patternInstances[len(it.patternInstances)-1] = app
						it.updateMaxGeneration(app)
						for i := uint32(0); i < c.numArgs; i++ {
							it.setRegister(c.oreg+i, app.arguments[i])
						}
						it.pc = c.next
						resumed = true
						break
					}
				}
				if !resumed {
					// continue failed
					if bp.toRecycle != nil {
						it.recycleBacktrackVector(bp)
					}
					bp.candidates = nil
					it.top--
					continue
				}
			default:
				it.err = ast.ErrASTDomain // UNREACHABLE
				return
			}
			if resumed {
				break
			}
		}
	}
}

// step executes the instruction at pc; false means backtrack.
func (it *smtMAMInterpreter) step() bool {
	pc := it.pc
	switch op := pc.op; {
	case op <= mamINITN:
		app := it.registers[0]
		if uint32(len(app.arguments)) != pc.numArgs {
			return false
		}
		for i, a := range app.arguments {
			it.setRegister(uint32(i)+1, a)
		}
		it.pc = pc.next
		return true
	case op == mamCOMPARE:
		if it.registers[pc.reg1].root != it.registers[pc.reg2].root {
			return false
		}
		it.pc = pc.next
		return true
	case op == mamCHECK:
		if it.registers[pc.reg].root != pc.enode.root {
			return false
		}
		it.pc = pc.next
		return true
	case op == mamCFILTER || op == mamFILTER:
		if pc.lblSet.EmptyIntersection(it.registers[pc.reg].root.labels) {
			return false
		}
		it.pc = pc.next
		return true
	case op == mamPFILTER:
		if pc.lblSet.EmptyIntersection(it.registers[pc.reg].root.propagatedLabels) {
			return false
		}
		it.pc = pc.next
		return true
	case op == mamCHOOSE:
		it.pushBacktrack(smtMAMBacktrackPoint{instr: pc, oldMaxGeneration: it.maxGeneration, oldUsedEnodesSize: len(it.usedEnodes)})
		it.pc = pc.next
		return true
	case op == mamNOOP:
		it.pc = pc.next
		return true
	case mamIsBind(op):
		n1 := it.registers[pc.ireg]
		currMax, currUsed := it.maxGeneration, len(it.usedEnodes)
		app := it.getFirstFApp(pc.label, pc.numArgs, n1)
		if app == nil {
			return false
		}
		it.pushBacktrack(smtMAMBacktrackPoint{instr: pc, oldMaxGeneration: currMax, oldUsedEnodesSize: currUsed, curr: app})
		for i := uint32(0); i < pc.numArgs; i++ {
			it.setRegister(pc.oreg+i, app.arguments[i])
		}
		it.pc = pc.next
		return true
	case op >= mamYIELD1 && op <= mamYIELDN:
		k := len(pc.binding)
		for len(it.bindings) < k {
			it.bindings = append(it.bindings, nil)
		}
		// m_bindings is filled in reverse register order.
		for i := 0; i < k; i++ {
			it.bindings[i] = it.registers[pc.binding[k-i-1]]
		}
		if g := SMTEnodesMaxGeneration(it.bindings[:k]); g > it.maxGeneration {
			it.maxGeneration = g
		}
		if it.cancelled != nil && it.cancelled() {
			it.releaseFrames() // m_top = 0
			return false
		}
		// on_match receives m_bindings itself; add_instance copies what it
		// keeps (the fingerprint's roots).
		if err := it.onMatch(pc.qa, pc.pat, it.bindings[:k:k], it.maxGeneration, it.usedEnodes); err != nil {
			it.err = err
		}
		return false
	case op == mamGETENODE:
		it.setRegister(pc.oreg, pc.enode)
		it.pc = pc.next
		return true
	case op >= mamGETCGR1 && op <= mamGETCGRN:
		k := int(pc.numArgs)
		for len(it.args) < k {
			it.args = append(it.args, nil)
		}
		for i := 0; i < k; i++ {
			it.args[i] = it.registers[pc.iregs[i]]
			// SET_VAR's plbls filter applies to GET_CGR1..6, not GET_CGRN.
			if op != mamGETCGRN && it.useFilters && pc.lblSet.EmptyIntersection(it.args[i].root.propagatedLabels) {
				return false
			}
		}
		flags, err := it.state.manager.GetFuncDeclFlags(pc.label)
		if err != nil {
			it.err = err
			return false
		}
		n1, err := it.state.congruence.FindApp(pc.label, it.args[:k], flags.Commutative)
		if err != nil {
			it.err = err
			return false
		}
		if n1 == nil || !it.relevant(n1) {
			return false
		}
		it.setRegister(pc.oreg, n1)
		it.pc = pc.next
		return true
	case op == mamISCGR:
		if !it.execIsCgr(pc) {
			return false
		}
		it.pc = pc.next
		return true
	case op == mamCONTINUE:
		app := it.initContinue(pc, pc.numArgs)
		if app == nil {
			return false
		}
		it.patternInstances = append(it.patternInstances, app)
		for i := uint32(0); i < pc.numArgs; i++ {
			it.setRegister(pc.oreg+i, app.arguments[i])
		}
		it.pc = pc.next
		return true
	}
	it.err = ast.ErrASTDomain
	return false
}
