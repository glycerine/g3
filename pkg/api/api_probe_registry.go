package api

// Source: src/api/api_tactic.cpp:48-67,286-322;
// src/tactic/probe.h:64-126. Only probes backed by retained concrete Go
// evaluators are registered here.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// APIProbeDescriptor is the stable public metadata associated with a named
// probe. It is a value so callers never borrow registry storage.
type APIProbeDescriptor struct {
	Name        string
	Description string
}

type apiProbeRegistration struct {
	descriptor APIProbeDescriptor
	kind       tactic.ProbeKind
}

// Keep source registration order. This order is observable through
// Z3_get_probe_name and is therefore not represented by a Go map.
var apiProbeRegistrations = [...]apiProbeRegistration{
	{APIProbeDescriptor{"memory", "amount of used memory in megabytes."}, tactic.ProbeMemory},
	{APIProbeDescriptor{"depth", "depth of the input goal."}, tactic.ProbeDepth},
	{APIProbeDescriptor{"size", "number of assertions in the given goal."}, tactic.ProbeSize},
	{APIProbeDescriptor{"num-exprs", "number of expressions/terms in the given goal."}, tactic.ProbeNumExprs},
	{APIProbeDescriptor{"num-consts", "number of non Boolean constants in the given goal."}, tactic.ProbeNumConsts},
	{APIProbeDescriptor{"num-bool-consts", "number of Boolean constants in the given goal."}, tactic.ProbeNumBoolConsts},
	{APIProbeDescriptor{"num-arith-consts", "number of arithmetic constants in the given goal."}, tactic.ProbeNumArithmeticConsts},
	{APIProbeDescriptor{"num-bv-consts", "number of bit-vector constants in the given goal."}, tactic.ProbeNumBitVectorConsts},
	{APIProbeDescriptor{"produce-proofs", "true if proof generation is enabled for the given goal."}, tactic.ProbeProduceProofs},
	{APIProbeDescriptor{"produce-model", "true if model generation is enabled for the given goal."}, tactic.ProbeProduceModels},
	{APIProbeDescriptor{"produce-unsat-cores", "true if unsat-core generation is enabled for the given goal."}, tactic.ProbeProduceUnsatCores},
	{APIProbeDescriptor{"has-quantifiers", "true if the goal contains quantifiers."}, tactic.ProbeHasQuantifier},
	{APIProbeDescriptor{"has-patterns", "true if the goal contains quantifiers with patterns."}, tactic.ProbeHasPattern},
	{APIProbeDescriptor{"is-propositional", "true if the goal is in propositional logic."}, tactic.ProbeIsPropositional},
	{APIProbeDescriptor{"is-qfbv", "true if the goal is in QF_BV."}, tactic.ProbeIsQFBitVector},
	{APIProbeDescriptor{"is-qfaufbv", "true if the goal is in QF_AUFBV."}, tactic.ProbeIsQFArrayUFBitVector},
}

func apiProbeRegistrationByName(name string) (apiProbeRegistration, bool) {
	for _, registration := range apiProbeRegistrations {
		if registration.descriptor.Name == name {
			return registration, true
		}
	}
	return apiProbeRegistration{}, false
}

// ProbeCount returns the number of retained named probes in native
// registration order.
func (c *SMTContext) ProbeCount() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return uint32(len(apiProbeRegistrations)), nil
}

// ProbeDescriptor returns copied metadata at a native registration index.
func (c *SMTContext) ProbeDescriptor(index uint32) (APIProbeDescriptor, error) {
	if err := c.open(); err != nil {
		return APIProbeDescriptor{}, err
	}
	if uint64(index) >= uint64(len(apiProbeRegistrations)) {
		return APIProbeDescriptor{}, arena.ErrBounds
	}
	return apiProbeRegistrations[index].descriptor, nil
}

// ProbeDescription returns the retained source description for a name.
func (c *SMTContext) ProbeDescription(name string) (string, error) {
	if err := c.open(); err != nil {
		return "", err
	}
	registration, found := apiProbeRegistrationByName(name)
	if !found {
		return "", ast.ErrASTDomain
	}
	return registration.descriptor.Description, nil
}
