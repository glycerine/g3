package api

// Source: generated api_commands.cpp and src/api/api_interp.cpp.

import (
	"bytes"
	"errors"

	"github.com/glycerine/g3/pkg/interp"
)

var replayInterpolationProfileText string

func replayInterpolationProfile(r *Z3Replayer) error {
	if _, err := r.Object(0); err != nil {
		return err
	}
	var output bytes.Buffer
	if err := interp.ProcessProfiler().Print(&output); err != nil {
		return err
	}
	replayInterpolationProfileText = output.String()
	return nil
}

func replayMkInterpolant(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	result, err := owner.context.Interpolant(value.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, result)
}

func replayMkInterpolationContext(r *Z3Replayer) (err error) {
	id, err := r.Object(0)
	if err != nil {
		return err
	}
	var config *Config
	if id == 0 {
		config, err = MkConfig()
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, config.Close()) }()
	} else {
		config = r.configs[id]
		if config == nil {
			return ErrZ3ReplayReference
		}
	}
	if err = SetParamValue(config, "proof", "true"); err != nil {
		return err
	}
	if err = SetParamValue(config, "model", "true"); err != nil {
		return err
	}
	return replayCreateContextWithConfig(r, config, false)
}
