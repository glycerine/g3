package rel

import (
	"errors"
	"testing"
)

func refusingLazyConstructOwner(t *testing.T, signature Signature) (*LazySparseEngine, *LazySparseTable, *lazySparseNode) {
	t.Helper()
	engine := NewLazySparseEngine()
	base, err := engine.New(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := base.Project(nil)
	if err != nil {
		t.Fatal(err)
	}
	child := derived.root.left
	child.references = 0
	return engine, derived, child
}

func TestConstructInstructionDisposesIncomingAfterRegisterReplacementRefusal(t *testing.T) {
	f := newTableRelationFixture(t)
	factory := NewLazySparseFiniteFactory()
	if err := f.relations.SetLazySparseFiniteFactory(factory); err != nil {
		t.Fatal(err)
	}
	if err := f.relations.SetDefaultFiniteTable("lazy_sparse"); err != nil {
		t.Fatal(err)
	}
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	priorEngine, prior, child := refusingLazyConstructOwner(t, signature)
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = context.SetFiniteRegister(0, prior); err != nil {
		t.Fatal(err)
	}
	instruction := NewTotalInstruction(signature, 0)
	concrete := instruction.(*constructInstruction)
	complete, err := instruction.Perform(context)
	if complete || !errors.Is(err, ErrLazySparse) {
		t.Fatal("replacement refusal", complete, err)
	}
	stored, err := context.FiniteRegister(0)
	if err != nil || stored != prior {
		t.Fatal("replacement refusal lost prior owner", stored, err)
	}
	if concrete.pendingFinite != nil || len(factory.engine.handles) != 0 {
		t.Fatal("replacement refusal lost or retained disposed incoming owner", concrete.pendingFinite, len(factory.engine.handles))
	}
	child.references = 2
	complete, err = instruction.Perform(context)
	if err != nil || !complete {
		t.Fatal("replacement retry", complete, err)
	}
	stored, err = context.FiniteRegister(0)
	if err != nil || !factory.Owns(stored) {
		t.Fatal("replacement retry did not publish incoming owner", stored, err)
	}
	if err = instruction.Close(); err != nil {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.relations.Close(); err != nil {
		t.Fatal(err)
	}
	if err = factory.Close(); err != nil {
		t.Fatal(err)
	}
	if err = priorEngine.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConstructInstructionRetainsIncomingWhenDisposalAlsoRefuses(t *testing.T) {
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	engine, incoming, child := refusingLazyConstructOwner(t, signature)
	instruction := NewTotalInstruction(signature, 0).(*constructInstruction)
	cause := errors.New("register replacement refused")
	err = instruction.disposeUninstalledFinite(incoming, cause)
	if !errors.Is(err, cause) || !errors.Is(err, ErrLazySparse) || instruction.pendingFinite != incoming {
		t.Fatal("incoming refusal owner receipt", err, instruction.pendingFinite)
	}
	if err = instruction.Close(); !errors.Is(err, ErrLazySparse) || instruction.pendingFinite != incoming || instruction.closed {
		t.Fatal("instruction close lost refused incoming", err, instruction.pendingFinite, instruction.closed)
	}
	child.references = 2
	if err = instruction.Close(); err != nil {
		t.Fatal("instruction close retry", err)
	}
	if instruction.pendingFinite != nil || !incoming.closed || !instruction.closed {
		t.Fatal("instruction close retry did not consume owners")
	}
	if err = engine.Close(); err != nil {
		t.Fatal(err)
	}
}
