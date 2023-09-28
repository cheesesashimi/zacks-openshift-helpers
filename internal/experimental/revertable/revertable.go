package daemon

import (
	"errors"
	"fmt"

	aggerrs "k8s.io/apimachinery/pkg/util/errors"
)

var ErrCanceledRevert = fmt.Errorf("canceled revert")

type revertable interface {
	Apply() error
	Revert() error
}

type simpleRevertable struct {
	applyFunc  func() error
	revertFunc func() error
}

func NewRevertable(applyFunc, revertFunc func() error) revertable {
	return &simpleRevertable{
		applyFunc:  applyFunc,
		revertFunc: revertFunc,
	}
}

func (r *simpleRevertable) Apply() error {
	return r.applyFunc()
}

func (r *simpleRevertable) Revert() error {
	return r.revertFunc()
}

func RunRevertables(rs []revertable) (retErr error) {
	errs := []error{}
	defer func() {
		retErr = aggerrs.NewAggregate(errs)
	}()

	isCanceledRevert := false

	runRevertFunc := func(revertFunc func() error) {
		if isCanceledRevert {
			return
		}

		err := revertFunc()
		if err == nil {
			return
		}

		errs = append(errs, fmt.Errorf("could not revert: %w", err))

		if errors.Is(err, ErrCanceledRevert) {
			isCanceledRevert = true
		}
	}

	for _, r := range rs {
		r := r
		applyErr := r.Apply()
		defer func() {
			if applyErr != nil {
				errs = append(errs, fmt.Errorf("could not apply: %w", applyErr))
				runRevertFunc(r.Revert)
			}

			if applyErr == nil && len(errs) != 0 {
				runRevertFunc(r.Revert)
			}
		}()

		if applyErr != nil {
			return
		}
	}

	return
}
