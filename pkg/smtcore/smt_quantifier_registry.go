package smtcore

// Source: src/smt/smt_quantifier.cpp quantifier_manager::imp registry,
// statistics, add_instance, search initialization, and scoped queue storage.

import (
	"fmt"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type SMTQuantifierRegistry struct {
	manager      *ast.ASTManager
	generator    *SMTQuantifierStatGenerator
	fingerprints *SMTFingerprintSet
	schedule     *SMTQIQueueSchedule
	costs        *SMTQICostModel
	statistics   map[ast.ASTHandle]*SMTQuantifierStat
	quantifiers  []ast.ASTHandle
	numInstances uint32
	maxInstances uint32
	scopeLevel   uint32
	pendingPop   uint32
	popScheduled bool
	// m_params.m_qi_profile and m_qi_profile_freq.
	profile          bool
	profileFrequency uint32
}

func NewSMTQuantifierRegistry(manager *ast.ASTManager, schedule *SMTQIQueueSchedule, costs *SMTQICostModel, maxInstances uint32) (*SMTQuantifierRegistry, error) {
	if manager == nil || schedule == nil || schedule.manager != manager || costs == nil || costs.manager != manager {
		return nil, ast.ErrASTHandle
	}
	generator, err := NewSMTQuantifierStatGenerator(manager)
	if err != nil {
		return nil, err
	}
	return &SMTQuantifierRegistry{
		manager: manager, generator: generator, fingerprints: NewSMTFingerprintSet(), schedule: schedule, costs: costs,
		statistics: make(map[ast.ASTHandle]*SMTQuantifierStat), maxInstances: maxInstances,
		profileFrequency: ^uint32(0),
	}, nil
}

func (r *SMTQuantifierRegistry) Add(quantifier ast.ASTHandle, generation uint32) error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.pendingPop != 0 {
		return ast.ErrASTPending
	}
	if _, found := r.statistics[quantifier]; found {
		return ast.ErrASTDomain
	}
	stat, err := r.generator.Generate(quantifier, generation)
	if err != nil {
		return err
	}
	r.statistics[quantifier] = stat
	r.quantifiers = append(r.quantifiers, quantifier)
	return nil
}

func (r *SMTQuantifierRegistry) Delete(quantifier ast.ASTHandle) error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if len(r.quantifiers) == 0 || r.quantifiers[len(r.quantifiers)-1] != quantifier {
		return ast.ErrASTDomain
	}
	if r.profile {
		if err := r.displayStatsVerbose(quantifier); err != nil {
			return err
		}
	}
	if stat := r.statistics[quantifier]; stat != nil {
		stat.candidateBody = nil
	}
	delete(r.statistics, quantifier)
	r.quantifiers = r.quantifiers[:len(r.quantifiers)-1]
	return nil
}

// displayStats is quantifier_manager::imp::display_stats.
func (r *SMTQuantifierRegistry) displayStats(quantifier ast.ASTHandle) (string, error) {
	stat, err := r.Stat(quantifier)
	if err != nil {
		return "", err
	}
	if stat.NumInstances() == 0 {
		return "", nil
	}
	qid, _, err := r.manager.GetQuantifierIDs(quantifier)
	if err != nil {
		return "", err
	}
	name, err := r.manager.Symbols.String(qid)
	if err != nil {
		return "", err
	}
	// out.width(10) << qid << " : "; out.width(6) << num_instances << " : ";
	// out.width(3) << max_generation << " : " << max_cost (a float, %g).
	return fmt.Sprintf("[quantifier_instances] %10s : %6d : %3d : %s\n", name, stat.NumInstances(), stat.MaxGeneration(),
		strconv.FormatFloat(float64(stat.MaxCost()), 'g', 6, 64)), nil
}

// displayStatsVerbose is display_stats(verbose_stream(), q).
func (r *SMTQuantifierRegistry) displayStatsVerbose(quantifier ast.ASTHandle) error {
	text, err := r.displayStats(quantifier)
	if err != nil || text == "" {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(text); err != nil {
		return err
	}
	return out.Flush()
}

// profileInstance is qi_queue::instantiate's
// if (stat->get_num_instances() % m_params.m_qi_profile_freq == 0)
// m_qm.display_stats(verbose_stream(), q), after inc_num_instances.
func (r *SMTQuantifierRegistry) profileInstance(quantifier ast.ASTHandle, stat *SMTQuantifierStat) error {
	if r == nil || r.profileFrequency == 0 || stat.NumInstances()%r.profileFrequency != 0 {
		return nil
	}
	return r.displayStatsVerbose(quantifier)
}

func (r *SMTQuantifierRegistry) Empty() bool { return r == nil || len(r.quantifiers) == 0 }
func (r *SMTQuantifierRegistry) NumQuantifiers() uint32 {
	if r == nil {
		return 0
	}
	return uint32(len(r.quantifiers))
}
func (r *SMTQuantifierRegistry) Quantifiers() []ast.ASTHandle {
	if r == nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), r.quantifiers...)
}

func (r *SMTQuantifierRegistry) Stat(quantifier ast.ASTHandle) (*SMTQuantifierStat, error) {
	if r == nil || r.manager == nil {
		return nil, ast.ErrASTHandle
	}
	stat := r.statistics[quantifier]
	if stat == nil {
		return nil, ast.ErrASTDomain
	}
	return stat, nil
}

func (r *SMTQuantifierRegistry) Generation(quantifier ast.ASTHandle) (uint32, error) {
	stat, err := r.Stat(quantifier)
	if err != nil {
		return 0, err
	}
	return stat.Generation(), nil
}

func (r *SMTQuantifierRegistry) AddInstance(quantifier, pattern ast.ASTHandle, bindings []*SMTEnode, maxGeneration, minTopGeneration, maxTopGeneration uint32) (bool, error) {
	return r.AddInstanceUsed(quantifier, pattern, bindings, nil, maxGeneration, minTopGeneration, maxTopGeneration)
}

// traceNewMatch is add_instance's [new-match] record: the fingerprint's
// address, the quantifier, its bindings and, after ';', the used enodes.
func (r *SMTQuantifierRegistry) traceNewMatch(fingerprint *SMTFingerprint, quantifierID uint32, bindings, usedEnodes []*SMTEnode) {
	out := r.manager.TraceStream
	if out == nil {
		return
	}
	w := traceStreamWriter(out)
	w(fmt.Sprintf("[new-match] %p #%d", fingerprint, quantifierID))
	for _, binding := range bindings {
		id, _ := r.manager.GetASTID(binding.owner)
		w(" #" + strconv.FormatUint(uint64(id), 10))
	}
	w(" ;")
	for _, used := range usedEnodes {
		id, _ := r.manager.GetASTID(used.owner)
		w(" #" + strconv.FormatUint(uint64(id), 10))
	}
	w("\n")
}

// AddInstanceUsed is quantifier_manager::add_instance with the used_enodes
// that justify an e-matching instance.
func (r *SMTQuantifierRegistry) AddInstanceUsed(quantifier, pattern ast.ASTHandle, bindings, usedEnodes []*SMTEnode, maxGeneration, minTopGeneration, maxTopGeneration uint32) (bool, error) {
	if r == nil || r.manager == nil || r.fingerprints == nil || r.schedule == nil || r.costs == nil {
		return false, ast.ErrASTHandle
	}
	if r.pendingPop != 0 {
		return false, ast.ErrASTPending
	}
	stat, err := r.Stat(quantifier)
	if err != nil {
		return false, err
	}
	// Preserve quantifier_manager's public forwarding behavior: the supplied
	// max_top_generation is replaced with the original max_generation.
	_ = maxTopGeneration
	sourceMaxTopGeneration := maxGeneration
	if stat.Generation() > maxGeneration {
		maxGeneration = stat.Generation()
	}
	if r.numInstances > r.maxInstances {
		return false, nil
	}
	stat.UpdateMaxGeneration(maxGeneration)
	id, err := r.manager.GetASTID(quantifier)
	if err != nil {
		return false, err
	}
	fingerprint, err := r.fingerprints.Insert(quantifier, id, bindings)
	if err != nil || fingerprint == nil {
		return false, err
	}
	r.traceNewMatch(fingerprint, id, bindings, usedEnodes)
	cost, err := r.costs.Cost(quantifier, pattern, stat, maxGeneration, minTopGeneration, sourceMaxTopGeneration, r.scopeLevel)
	if err != nil {
		return false, err
	}
	if err = r.schedule.Insert(fingerprint, cost, maxGeneration); err != nil {
		return false, err
	}
	r.numInstances++
	return true, nil
}

func (r *SMTQuantifierRegistry) InitSearch() error {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.pendingPop != 0 {
		return ast.ErrASTPending
	}
	r.numInstances = 0
	for _, quantifier := range r.quantifiers {
		r.statistics[quantifier].ResetInstancesCurrentSearch()
	}
	return nil
}

func (r *SMTQuantifierRegistry) PushScope() error {
	if r == nil || r.fingerprints == nil || r.schedule == nil {
		return ast.ErrASTHandle
	}
	if r.pendingPop != 0 {
		return ast.ErrASTPending
	}
	if err := r.schedule.PushScope(); err != nil {
		return err
	}
	if err := r.fingerprints.PushScope(); err != nil {
		_ = r.schedule.PopScopes(1)
		return err
	}
	r.scopeLevel++
	return nil
}

func (r *SMTQuantifierRegistry) PopScopes(count uint32) error {
	if r == nil || r.fingerprints == nil || r.schedule == nil {
		return ast.ErrASTHandle
	}
	if r.pendingPop != 0 && r.pendingPop != count {
		return ast.ErrASTPending
	}
	if count == 0 || count > r.scopeLevel {
		return ast.ErrASTDomain
	}
	r.pendingPop = count
	if !r.popScheduled {
		if err := r.schedule.PopScopes(count); err != nil {
			return err
		}
		r.popScheduled = true
	}
	if err := r.fingerprints.PopScopes(count); err != nil {
		return err
	}
	r.scopeLevel -= count
	r.pendingPop = 0
	r.popScheduled = false
	return nil
}

func (r *SMTQuantifierRegistry) Reset() error {
	if r == nil {
		return nil
	}
	if err := r.schedule.Reset(); err != nil {
		return err
	}
	r.fingerprints.Reset()
	for _, stat := range r.statistics {
		if stat != nil {
			stat.candidateBody = nil
		}
	}
	r.statistics = make(map[ast.ASTHandle]*SMTQuantifierStat)
	r.quantifiers = nil
	r.numInstances = 0
	r.scopeLevel = 0
	r.pendingPop = 0
	r.popScheduled = false
	return nil
}
