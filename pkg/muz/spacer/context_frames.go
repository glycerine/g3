package spacer

// Source: src/muz/spacer/spacer_context.h and spacer_context.cpp
// pred_transformer::frames.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
)

type FrameTransformer interface {
	EnsureLevel(uint32) error
	AddLemmaCore(*Lemma) error
	IsInvariant(level uint32, formula ast.ASTHandle) (invariant bool, solverLevel uint32, err error)
}

// Simplify is pred_transformer::frames::simplify_formulas: one
// unit-subsumption tactic and one goal serve every frame and the infinity
// frame, and only a level with more than one lemma is simplified.
func (f *Frames) Simplify(owner *Manager) (resultErr error) {
	if err := f.open(); err != nil {
		return err
	}
	if owner == nil || owner.open() != nil || owner.manager != f.manager || owner.contexts[0] == nil {
		return ErrManagerOwner
	}
	if err := f.sort(); err != nil {
		return err
	}
	simplifier, err := smtcore.NewUnitSubsumptionTactic(f.manager, nil, owner.contexts[0].config)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, simplifier.Close()) }()
	lemmasSize := len(f.lemmas)
	g, err := tactic.NewGoal(f.manager, false, false, false)
	if err != nil {
		return err
	}
	// Locals die in reverse: g, then new_lemmas, then the simplifier.
	defer func() { resultErr = errors.Join(resultErr, g.Close()) }()
	newLemmas := make([]*Lemma, 0, lemmasSize)
	used := make([]bool, lemmasSize)
	j := 0
	for i := uint32(0); i <= f.size; i++ {
		if err = g.ResetAll(); err != nil {
			return err
		}
		level := InfinityLevel
		if i < f.size {
			level = i
		}
		begin := j
		for ; j < lemmasSize; j++ {
			lemmaLevel, err := f.lemmas[j].Level()
			if err != nil {
				return err
			}
			if lemmaLevel > level {
				break
			}
			if lemmaLevel == level {
				body, err := f.lemmas[j].Body()
				if err != nil {
					return err
				}
				if err = g.Assert(body); err != nil {
					return err
				}
			}
		}
		end := j
		sz := end - begin
		if sz <= 0 {
			continue
		}
		if sz == 1 {
			newLemmas = append(newLemmas, f.lemmas[begin])
			used[begin] = true
			continue
		}
		if err = f.simplifyLevel(simplifier, g, begin, end, &newLemmas, used); err != nil {
			return err
		}
	}
	if len(newLemmas) < len(f.lemmas) {
		for index, lemma := range f.lemmas {
			if !used[index] {
				resultErr = errors.Join(resultErr, lemma.Close())
			}
		}
		f.lemmas = newLemmas
		f.sorted = false
		resultErr = errors.Join(resultErr, f.sort())
	}
	return resultErr
}

// simplifyLevel runs the simplifier on the level's goal and keeps the lemmas
// whose expressions remain. A remaining form takes the first unused lemma
// with that expression.
func (f *Frames) simplifyLevel(simplifier *smtcore.UnitSubsumptionTactic, g *tactic.Goal, begin, end int, newLemmas *[]*Lemma, used []bool) (resultErr error) {
	result := &tactic.TacticResult{}
	defer func() { resultErr = errors.Join(resultErr, result.Close()) }()
	if err := simplifier.Apply(g, result); err != nil {
		return err
	}
	if len(result.Goals) != 1 {
		return ast.ErrASTDomain
	}
	r := result.Goals[0]
	size, err := r.Size()
	if err != nil {
		return err
	}
	if int(size) == end-begin {
		for n := begin; n < end; n++ {
			*newLemmas = append(*newLemmas, f.lemmas[n])
			used[n] = true
		}
		return nil
	}
	for k := uint32(0); k < size; k++ {
		form, err := r.Form(k)
		if err != nil {
			return err
		}
		found := false
		for n := begin; n < end && !found; n++ {
			body, err := f.lemmas[n].Body()
			if err != nil {
				return err
			}
			if !used[n] && body == form {
				*newLemmas = append(*newLemmas, f.lemmas[n])
				used[n] = true
				found = true
			}
		}
		if !found {
			return ast.ErrASTDomain // ENSURE(found)
		}
	}
	return nil
}

type Frames struct {
	manager     *ast.ASTManager
	transformer FrameTransformer
	lemmas      []*Lemma
	additions   uint64
	size        uint32
	sorted      bool
	closed      bool
}

func NewFrames(manager *ast.ASTManager, transformer FrameTransformer) (*Frames, error) {
	if manager == nil || transformer == nil {
		return nil, ErrManagerOwner
	}
	return &Frames{manager: manager, transformer: transformer, sorted: true}, nil
}

func (f *Frames) open() error {
	if f == nil || f.closed || f.manager == nil || f.transformer == nil {
		return ErrManagerOwner
	}
	return nil
}

func (f *Frames) AddFrame() error {
	if err := f.open(); err != nil {
		return err
	}
	f.size++
	return nil
}

func (f *Frames) Size() (uint32, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return f.size, nil
}

func (f *Frames) LemmaSize() (int, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return len(f.lemmas), nil
}

// LemmaAdditions returns the cumulative number of successful frame insertions
// and promotions used by the Spacer restart schedule.
func (f *Frames) LemmaAdditions() (uint64, error) {
	if err := f.open(); err != nil {
		return 0, err
	}
	return f.additions, nil
}

func (f *Frames) sort() error {
	if f.sorted {
		return nil
	}
	// lemma_lt_proc orders first by frame level and then ast_lt_proc's
	// source AST ID, not the separate structural ast_lt comparison.
	type entry struct {
		lemma *Lemma
		level uint32
		id    uint32
	}
	entries := make([]entry, len(f.lemmas))
	for i, lemma := range f.lemmas {
		level, err := lemma.Level()
		if err != nil {
			return err
		}
		body, err := lemma.Body()
		if err != nil {
			return err
		}
		id, err := f.manager.GetASTID(body)
		if err != nil {
			return err
		}
		entries[i] = entry{lemma: lemma, level: level, id: id}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].level < entries[j].level ||
			(entries[i].level == entries[j].level && entries[i].id < entries[j].id)
	})
	for i, item := range entries {
		f.lemmas[i] = item.lemma
	}
	f.sorted = true
	return nil
}

// movePromotedLemma follows the source's adjacent forward swaps. The vector
// may still be unsorted after newly appended lemmas; do not sort unrelated
// entries or change its sorted flag during duplicate promotion.
func (f *Frames) movePromotedLemma(index int) error {
	for index+1 < len(f.lemmas) {
		current, next := f.lemmas[index], f.lemmas[index+1]
		currentLevel, err := current.Level()
		if err != nil {
			return err
		}
		nextLevel, err := next.Level()
		if err != nil {
			return err
		}
		less := nextLevel < currentLevel
		if nextLevel == currentLevel {
			currentBody, err := current.Body()
			if err != nil {
				return err
			}
			nextBody, err := next.Body()
			if err != nil {
				return err
			}
			currentID, err := f.manager.GetASTID(currentBody)
			if err != nil {
				return err
			}
			nextID, err := f.manager.GetASTID(nextBody)
			if err != nil {
				return err
			}
			less = nextID < currentID
		}
		if !less {
			break
		}
		f.lemmas[index], f.lemmas[index+1] = next, current
		index++
	}
	return nil
}

func (f *Frames) AddLemma(lemma *Lemma) (added bool, resultErr error) {
	if err := f.open(); err != nil {
		return false, err
	}
	if lemma == nil || lemma.open() != nil || lemma.manager != f.manager {
		return false, ErrManagerOwner
	}
	body, err := lemma.Body()
	if err != nil {
		return false, err
	}
	for index, existing := range f.lemmas {
		other, bodyErr := existing.Body()
		if bodyErr != nil {
			return false, bodyErr
		}
		if other != body {
			continue
		}
		bindings, bindingErr := lemma.bindingHandles()
		if bindingErr != nil {
			return false, bindingErr
		}
		if bindingErr = existing.AddBindings(bindings); bindingErr != nil {
			return false, bindingErr
		}
		level, levelErr := lemma.Level()
		if levelErr != nil {
			return false, levelErr
		}
		existingLevel, levelErr := existing.Level()
		if levelErr != nil {
			return false, levelErr
		}
		if closeErr := lemma.Close(); closeErr != nil {
			return false, closeErr
		}
		if existingLevel >= level {
			return false, nil
		}
		if err = existing.setLevel(level); err != nil {
			return false, err
		}
		if err = f.transformer.AddLemmaCore(existing); err != nil {
			return false, err
		}
		if err = f.movePromotedLemma(index); err != nil {
			return false, err
		}
		f.additions++
		return true, nil
	}
	f.lemmas = append(f.lemmas, lemma)
	f.sorted = false
	if err = f.transformer.AddLemmaCore(lemma); err != nil {
		f.lemmas = f.lemmas[:len(f.lemmas)-1]
		return false, err
	}
	f.additions++
	return true, nil
}

func (f *Frames) FrameLemmas(level uint32) ([]ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	var result []ast.ASTHandle
	for _, lemma := range f.lemmas {
		lemmaLevel, err := lemma.Level()
		if err != nil {
			return nil, err
		}
		if lemmaLevel == level {
			body, err := lemma.Body()
			if err != nil {
				return nil, err
			}
			result = append(result, body)
		}
	}
	return result, nil
}

func (f *Frames) FrameGEQLemmas(level uint32) ([]ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	var result []ast.ASTHandle
	for _, lemma := range f.lemmas {
		lemmaLevel, err := lemma.Level()
		if err != nil {
			return nil, err
		}
		if lemmaLevel >= level {
			body, err := lemma.Body()
			if err != nil {
				return nil, err
			}
			result = append(result, body)
		}
	}
	return result, nil
}

func (f *Frames) PropagateToInfinity(level uint32) error {
	if err := f.open(); err != nil {
		return err
	}
	for _, lemma := range f.lemmas {
		lemmaLevel, err := lemma.Level()
		if err != nil {
			return err
		}
		if lemmaLevel >= level && lemmaLevel != InfinityLevel {
			if err = lemma.setLevel(InfinityLevel); err != nil {
				return err
			}
			if err = f.transformer.AddLemmaCore(lemma); err != nil {
				return err
			}
			f.sorted = false
		}
	}
	return nil
}

func (f *Frames) PropagateToNextLevel(level uint32) (bool, error) {
	if err := f.open(); err != nil {
		return false, err
	}
	if err := f.sort(); err != nil {
		return false, err
	}
	if len(f.lemmas) == 0 {
		return true, nil
	}
	target := level + 1
	if level == InfinityLevel {
		target = InfinityLevel
	}
	if err := f.transformer.EnsureLevel(target); err != nil {
		return false, err
	}
	all := true
	for index := 0; index < len(f.lemmas); {
		lemma := f.lemmas[index]
		lemmaLevel, err := lemma.Level()
		if err != nil {
			return false, err
		}
		if lemmaLevel < level {
			index++
			continue
		}
		if lemmaLevel > level {
			break
		}
		body, err := lemma.Body()
		if err != nil {
			return false, err
		}
		invariant, solverLevel, err := f.transformer.IsInvariant(target, body)
		if err != nil {
			return false, err
		}
		if !invariant {
			all = false
			index++
			continue
		}
		if err = lemma.setLevel(solverLevel); err != nil {
			return false, err
		}
		if err = f.transformer.AddLemmaCore(lemma); err != nil {
			return false, err
		}
		if err = f.movePromotedLemma(index); err != nil {
			return false, err
		}
	}
	return all, nil
}

// inheritFrom translates frames::inherit_frames: normalize a new lemma for
// each old body/level, copy its binding vector, and assert it into this frame.
func (f *Frames) inheritFrom(owner *Manager, other *Frames) error {
	if err := f.open(); err != nil {
		return err
	}
	if other == nil || other.open() != nil || owner == nil || owner.manager != f.manager || other.manager != f.manager {
		return ErrManagerOwner
	}
	for _, previous := range other.lemmas {
		body, err := previous.Body()
		if err != nil {
			return err
		}
		level, err := previous.Level()
		if err != nil {
			return err
		}
		lemma, err := NewLemma(owner, body, level)
		if err != nil {
			return err
		}
		bindings, err := previous.bindingHandles()
		if err == nil {
			err = lemma.AddBindings(bindings)
		}
		if err == nil {
			_, err = f.AddLemma(lemma)
		}
		if err != nil {
			return errors.Join(err, lemma.Close())
		}
	}
	f.sorted = false
	return nil
}

func (f *Frames) Close() (result error) {
	if f == nil {
		return ErrManagerOwner
	}
	if f.closed {
		return nil
	}
	if f.manager == nil {
		return ErrManagerOwner
	}
	for i := len(f.lemmas) - 1; i >= 0; i-- {
		result = errors.Join(result, f.lemmas[i].Close())
	}
	if result == nil {
		f.lemmas = nil
		f.manager = nil
		f.transformer = nil
		f.closed = true
	}
	return result
}
